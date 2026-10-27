package alis

// Map sprites (START.EXE 0x12c16..0x134ab): a resource of type 0x7f is a
// map; its header sits below the resource (e = resource+0x3d8, fields at
// e-off). Type 9 maps draw a textured cell map, type 10 maps draw nothing,
// the others draw a tile per map byte.

const dsMapSprite = 0x1c0fe // the sprite's first 0x30 bytes

// blitMap is 0x12c16 for sprite di and the map resource esi.
func (v *VM) blitMap(di uint16, esi uint32) {
	for i := uint16(0); i < 0x18; i++ {
		v.wu16(dsMapSprite+uint32(i)*2, v.fsu16(di+2*i, 0))
	}
	e := esi + 0x3d8
	switch v.mem[e-0x400] {
	case 9:
		v.blitTexMap(e)
	case 10:
	default:
		v.blitTileMap(e)
	}
}

// Tile map words (0x1c0e4..0x1c0fc).
const (
	tmRows  = 0x1c0e4 // rows left less one
	tmCols  = 0x1c0e6 // columns left in the row
	tmRows0 = 0x1c0e8
	tmCols0 = 0x1c0ea
	tmX     = 0x1c0ec // the column's screen x
	tmY     = 0x1c0ee // the row's screen y
	tmX0    = 0x1c0f0
	tmNext  = 0x1c0f2 // map offset from a row's end to the next row
	tmSkipL = 0x1c0f4 // pixels of the first column left of the clip
	tmSkipR = 0x1c0f8 // pixels of the last column right of the clip
	tmRow   = 0x1c0fc
)

// blitTileMap is 0x12c69: the map's bytes (columns e-4 apart) pick tiles
// (from e-0x3fa on, up to e-0x3f8) of the tile set named by e-0x3fc,
// e-0x3f4 x e-0x3f0 pixels a cell; with bit 1 of e-0x3fe the tiles stand
// on the middle of their cell. The cell pointer keeps the high word of
// 0x1c0fe in ebp, so the map is read at 0x10000 + its low 16 bits.
// Untested against the oracle: no recorded session draws a tile map.
func (v *VM) blitTileMap(e uint32) {
	s := uint32(int32(int16(v.ru16(e-0x3fc)))) + v.ru32(0x15f72)
	if v.ru16(s) == 0 {
		return
	}
	s = v.ru32(v.ru32(s) - 0x14)
	s += v.ru32(s + 0xe)
	s += v.ru32(s)
	cw, ch := v.ru16(e-0x3f4), v.ru16(e-0x3f0)
	if cw == 0 || ch == 0 {
		v.fatal("draw: tile map division fault (START.EXE 0x12ca3)")
		return
	}
	a := v.ru16(dsClipx1) - v.ru16(dsMapSprite+0xc) + v.ru16(e-0x3e2)
	col, rem := a/cw, a%cw
	v.wu16(tmSkipL, rem)
	v.wu16(tmX, v.ru16(dsClipx1)-rem)
	v.wu16(tmSkipR, 0)
	if rem != 0 {
		v.wu16(tmSkipR, cw-rem)
	}
	a = v.ru16(dsClipy1) - v.ru16(dsMapSprite+0xe) + v.ru16(e-0x3de)
	row := a / ch
	v.wu16(tmRow, row)
	v.wu16(tmY, v.ru16(dsClipy1)-a%ch)
	a = v.ru16(dsClipy2) - v.ru16(dsMapSprite+0xe) + v.ru16(e-0x3de)
	v.wu16(tmRows, a/ch-row)
	stride := v.ru16(e - 4)
	bp := row + stride*col + uint16(e)
	cols := (v.ru16(dsClipx2) - v.ru16(dsClipx1) + 1) / cw
	if rem != 0 {
		cols++
	}
	v.wu16(tmCols, cols)
	v.wu16(tmNext, 1-cols*stride)
	if v.mem[e-0x3fe]&2 != 0 {
		v.wu16(tmY, v.ru16(tmY)+(ch-1)>>1)
		if row != 0 {
			v.wu16(tmRows, v.ru16(tmRows)+1)
			bp--
			v.wu16(tmY, v.ru16(tmY)-ch)
		}
		v.wu16(tmRows, v.ru16(tmRows)+1)
	}
	if cols == 0 {
		v.fatal("draw: tile map without columns loops 2^32 times (START.EXE 0x12e53)")
		return
	}
	v.wu16(tmX0, v.ru16(tmX))
	v.wu16(tmRows0, v.ru16(tmRows))
	v.wu16(tmCols0, cols)
	for {
		for range cols {
			if t := uint16(v.mem[0x10000|uint32(bp)]); t != 0 {
				if t += v.ru16(e-0x3fa) - 1; t <= v.ru16(e-0x3f8) {
					tile := s + uint32(int32(int16(t*4)))
					v.drawTile(e, tile+v.ru32(tile))
				}
			}
			bp += stride
			v.wu16(tmX, v.ru16(tmX)+cw)
			v.wu16(tmCols, v.ru16(tmCols)-1)
		}
		v.wu16(tmY, v.ru16(tmY)+ch)
		v.wu16(tmCols, cols)
		v.wu16(tmX, v.ru16(tmX0))
		bp += v.ru16(tmNext)
		r := int16(v.ru16(tmRows)) - 1
		v.wu16(tmRows, uint16(r))
		if r < 0 {
			return
		}
	}
}

// drawTile is 0x12ea1: tile t (type, width-1, height-1, for 4-bit tiles
// of types 0x10 and 0x12 a colour base, then 16 pixel rows) at tmX, tmY,
// clipped to the clip rows and to the first and last columns' skips.
// Types 0x10 and 0 have transparent 0 nibbles, 0x12 and 2 are opaque, 2 and
// 0 without a colour base; 0x14 is 8-bit with transparent 0, 0x16 8-bit
// opaque. As in the original, partial rows of 8-bit tiles don't step the
// source over the rest of the row the same way as full ones.
func (v *VM) drawTile(e, t uint32) {
	dx := int16(v.ru16(tmY))
	h := int16(v.ru16(t + 4))
	if v.mem[e-0x3fe]&2 != 0 {
		dx -= int16(uint16(h) >> 1)
	}
	cy1, cy2 := int16(v.ru16(dsClipy1)), int16(v.ru16(dsClipy2))
	var bx uint16
	var bp int16
	switch {
	case dx < cy1:
		if bp = dx + h - cy1; bp < 0 {
			return
		}
		a := h - bp
		dx += a
		bx = uint16(a) << 4
		if v.ru16(tmRows0) == 0 {
			if bp = cy2 - dx; bp < 0 {
				return
			}
		}
	case dx+h > cy2:
		if bp = cy2 - dx; bp < 0 {
			return
		}
	default:
		bp = h
	}
	off := int16(uint16(dx-v.r16(dsWlogy1))*v.ru16(dsWloglarg) + v.ru16(tmX) - v.ru16(dsWlogx1))
	di := v.ru32(dsWlogic) + uint32(int32(off))
	w := v.ru16(t + 2)
	cx := w
	switch c := v.ru16(tmCols); {
	case c == v.ru16(tmCols0):
		skip := v.ru16(tmSkipL)
		cx = w - skip
		bx += skip
		di += uint32(int32(int16(skip)))
		if c == 1 {
			cx = w - v.ru16(tmSkipR)
		}
	case c == 1:
		cx = w - v.ru16(tmSkipR)
	}
	cx++
	skipRow := uint32(int32(int16(v.ru16(dsWloglarg) - cx)))
	ecx := uint32(int32(int16(cx)))
	ebx := uint32(int32(int16(bx)))
	rows := int(bp) + 1
	switch typ := v.mem[t]; typ {
	case 0x16:
		src := t + ebx + 8
		ecx >>= 1
		bl := uint8(ecx)
		for range rows {
			copy(v.mem[di:di+2*ecx], v.mem[src:src+2*ecx])
			di += 2*ecx + skipRow
			src += 2 * ecx
			if bl != 0x10 {
				src += 0x10 // ecx is 0 after rep movsw
			}
			ecx = uint32(bl)
		}
	case 0x14:
		src := t + ebx
		src = src&^0xffff | uint32(uint16(src)+8) // add si, 8
		bl := uint8(ecx)
		for range rows {
			for range ecx {
				if b := v.mem[src]; b != 0 {
					v.mem[di] = b
				}
				src++
				di++
			}
			di += skipRow
			ecx = uint32(bl)
		}
	case 0x10, 0x12, 0, 2:
		var base uint8
		src := t + ebx>>1 + 6
		if typ >= 0x10 {
			base = v.mem[t+6]
			src += 2
		}
		transp := typ&2 == 0
		full := uint16(ecx) == 0x10
		if transp {
			full = uint8(ecx) == 0x10
		}
		bl := uint8(ecx)
		if full {
			bl = 8
		}
		for range rows {
			if full {
				src, di = v.nibRow(src, di, 16, false, transp, base)
				di += skipRow
				continue
			}
			src, di = v.nibRow(src, di, ecx, uint8(ecx)&1 != 0 && di&1 == 0, transp, base)
			if transp {
				di += skipRow
			} else {
				di = di&^0xffff | uint32(uint16(di)+uint16(skipRow)) // add di, dx
			}
			src += 8 - uint32(uint8(bl+1)>>1)
			ecx = uint32(bl)
		}
	}
}

// nibRow writes n pixels of packed nibbles (high nibble first, or from the
// first byte's low nibble) plus base, skipping 0 nibbles when transp.
func (v *VM) nibRow(src, di, n uint32, lowFirst, transp bool, base uint8) (uint32, uint32) {
	hi := !lowFirst
	var b uint8
	if lowFirst {
		b = v.mem[src]
		src++
	}
	for ; n != 0; n-- {
		var p uint8
		if hi {
			b = v.mem[src]
			src++
			p = b >> 4
		} else {
			p = b & 0xf
		}
		if !transp || p != 0 {
			v.mem[di] = p + base
		}
		di++
		hi = !hi
	}
	return src, di
}

// blitTexMap is 0x13168: every pixel of the clip rectangle whose bit is
// clear in the mask map (a word per 16 pixels, rows of 2 bytes, columns
// e-0x3c4 apart) shows the texel of its map cell's material through the
// darkness table, from the bottom row up.
func (v *VM) blitTexMap(e uint32) {
	f16 := func(a, off uint32) uint16 { return v.ru16(a - off) }
	v.mem[0x1c16c] = v.mem[e-0x3b0]
	v.mem[0x1c16d] = v.mem[e-0x3ae]
	ax := f16(e, 0x3e0) / f16(e, 0x3f2)
	ax += v.ru16(dsMapSprite+0xe) + f16(e, 0x3d4) - v.ru16(dsClipy2)
	by := uint32(ax)
	ax = f16(e, 0x3e2) / f16(e, 0x3f4)
	ax += (v.ru16(dsClipx1) - v.ru16(dsMapSprite+0xc)) >> 4
	p := uint32(ax) * uint32(f16(e, 0x3c4))
	mask := e
	if int8(v.mem[mask-1]) < 0 {
		mask = v.ru32(v.ru32(mask))
	}
	mask += p + by*2
	m := v.ru32(e - 0x3ba)
	if m == 0 {
		v.alisError(0x1a)
	}
	v.wu32(dsGsBase, m-0xc00)
	bx := uint16(by)
	if cl := int16(f16(e, 0x3c0) - f16(m, 0x3c0)); cl > 0 {
		ax <<= uint(cl & 31)
	}
	if cl := int16(f16(e, 0x3be) - f16(m, 0x3be)); cl > 0 {
		bx <<= uint(cl & 31)
	}
	cell := m
	if int8(v.mem[cell-1]) < 0 {
		cell = v.ru32(v.ru32(cell))
	}
	cell += uint32(ax)*uint32(f16(m, 0x3c4)) + uint32(bx)*2
	rows := v.ru16(dsClipy2) - v.ru16(dsClipy1) + 1
	groups := (v.ru16(dsClipx2) - v.ru16(dsClipx1) + 1) >> 4
	maskStep := uint32(f16(e, 0x3c4))
	pixStep := uint32(f16(m, 0x3c4))
	if cl := int16(f16(e, 0x3c0) - 4 - f16(m, 0x3c0)); cl > 0 {
		pixStep <<= uint(cl & 31)
	}
	rowStep := uint32(2)
	if cl := int16(f16(e, 0x3be) - f16(m, 0x3be)); cl > 0 {
		rowStep <<= uint(cl & 31)
	}
	wl := uint32(v.ru16(dsWloglarg))
	x0 := v.ru16(dsClipx1) - v.ru16(dsWlogx1)
	y := v.ru16(dsClipy2) - v.ru16(dsWlogy1)
	dst := v.ru32(dsWlogic) + uint32(y)*wl + uint32(x0)
	for ; rows != 0; rows-- {
		// 0x133a5 tests the flags of the add, not of the not, so a full
		// group is never skipped
		x, c, d, mk := x0, cell, dst, mask
		for range groups {
			bits := ^v.ru16(mk)
			mk += maskStep
			for range 16 {
				if bits&0x8000 != 0 {
					v.mapTexel(c, d, x, y)
				}
				bits <<= 1
				d++
				x++
				c += pixStep
			}
		}
		y--
		mask += 2
		cell += rowStep
		dst -= wl
	}
	v.wu16(0x1c16e, x0)
	v.wu16(0x1c170, y)
}

// mapTexel is 0x13411: the pixel at dst for map cell c at x, y. The cell's
// second byte picks a material (32 bytes in gs; +0x14 < 0 redirects to
// another cell), the first byte's high nibble and the second's top bits the
// shade.
func (v *VM) mapTexel(c, dst uint32, x, y uint16) {
	mat := uint32(v.mem[c+1]&0x3f) << 5
	if int8(v.mem[v.gs(mat+0x14)]) < 0 {
		c += v.gs32(mat + 0x10)
		mat = uint32(v.mem[c+1]&0x3f) << 5
	}
	if v.mem[v.gs(mat+8)] == 0 {
		return
	}
	tex := v.gs32(mat + 4)
	if tex == 0 {
		return
	}
	tex += v.ru32(tex)
	org := v.gsu16(mat + 2)
	bx := (y&(v.ru16(tex+4)-org) + org) << (v.mem[v.gs(mat)] & 31)
	bx += x & v.ru16(tex+2)
	texel := v.mem[tex+uint32(bx)+8]
	b1 := v.mem[c+1]
	shade := -((b1<<3 | b1>>5) & 6)
	shade = uint8(int8(shade+v.mem[0x1aafe]) >> 1)
	shade += v.mem[c] >> 4
	if shade >= 0xf {
		if int8(shade-0xf) < 0 {
			shade = 0
		} else {
			shade = 0xf
		}
	}
	v.mem[dst] = v.mem[v.ptrdark+(uint32(shade)<<8|uint32(texel))]
}
