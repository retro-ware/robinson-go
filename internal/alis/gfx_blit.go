package alis

// Pixels (START.EXE 0x123e8..0x12c15, 0x139f6..0x13bf6): sprites into the
// logic screen, rectangles of it into video memory, and the mouse cursor.

// blitSprite is 0x12579: sprite di within the window 0x15fea. Its
// resource starts with the type, the width and height less one.
func (v *VM) blitSprite(di uint16) {
	esi := v.fsu32(di, spNewad)
	esi += v.ru32(esi)
	x, y := v.fs16(di, spNewx), v.fs16(di, spNewy)
	if x > v.r16(dsFenx2) || y > v.r16(dsFeny2) {
		return
	}
	x2, y2 := x+v.r16(esi+2), y+v.r16(esi+4)
	if x2 < v.r16(dsFenx1) || y2 < v.r16(dsFeny1) {
		return
	}
	v.w16(dsClipx1, max(x, v.r16(dsFenx1)))
	v.w16(dsClipy1, max(y, v.r16(dsFeny1)))
	v.w16(dsClipx2, min(x2, v.r16(dsFenx2)))
	v.w16(dsClipy2, min(y2, v.r16(dsFeny2)))
	t := v.mem[esi]
	if t == 0x7f {
		v.blitMap(di, esi)
		return
	}
	if v.fs8(di, spNewf)&1 == 0 {
		switch t {
		case 0x10:
			v.mem[dsBlitCol] = v.mem[esi+6]
			if v.fs8(di, spColour+1) == 0x80 {
				v.mem[dsBlitCol] = v.fs8(di, spColour)
			}
			src, dst, w, rows := v.setup4(di, esi)
			v.blit4(src, dst, w, rows, v.mem[dsBlitCol], true)
		case 0x14:
			if v.fs8(di, spColour+1) == 0x80 {
				v.mem[dsBlitCol] = v.fs8(di, spColour)
				src, dst, w, rows := v.setup8(di, esi)
				v.blit8mono(src, dst, w, rows, v.mem[dsBlitCol])
				return
			}
			src, dst, w, rows := v.setup8(di, esi)
			v.blit8(src, dst, w, rows, true)
		case 0:
			src, dst, w, rows := v.setup4(di, esi)
			v.blit4(src-2, dst, w, rows, 0, true)
		case 0x12:
			v.mem[dsBlitCol] = v.mem[esi+6]
			src, dst, w, rows := v.setup4(di, esi)
			v.blit4(src, dst, w, rows, v.mem[dsBlitCol], false)
		case 0x16:
			src, dst, w, rows := v.setup8(di, esi)
			v.blit8(src, dst, w, rows, false)
		case 2:
			src, dst, w, rows := v.setup4(di, esi)
			v.blit4(src-2, dst, w, rows, 0, false)
		case 1:
			v.fill(di, esi)
		}
		return
	}
	switch t {
	case 0x10:
		v.mem[dsBlitCol] = v.mem[esi+6]
		src, dst, w, rows := v.setup4(di, esi)
		v.blit4x(src, dst+w, w, rows, v.mem[dsBlitCol], true)
	case 0x14:
		src, dst, w, rows := v.setup8(di, esi)
		v.blit8x(src, dst+w, w, rows, true)
	case 0:
		src, dst, w, rows := v.setup4(di, esi)
		v.blit4x(src-2, dst+w, w, rows, 0, true)
	case 0x12:
		v.mem[dsBlitCol] = v.mem[esi+6]
		src, dst, w, rows := v.setup4(di, esi)
		v.blit4x(src, dst+w, w, rows, v.mem[dsBlitCol], false)
	case 0x16:
		src, dst, w, rows := v.setup8(di, esi)
		v.blit8x(src, dst+w, w, rows, false)
	case 2:
		src, dst, w, rows := v.setup4(di, esi)
		v.blit4x(src-2, dst+w, w, rows, 0, false)
	case 1:
		v.fill(di, esi)
	}
}

// setup8 is 0x126d7: the clipped rectangle 0x15fd6 of 8-bit resource esi
// for sprite di: the first source and destination pixels, the width and
// the rows; the row skips go to 0x1bfc0 and 0x1bfc4.
func (v *VM) setup8(di uint16, esi uint32) (src, dst, w uint32, rows uint16) {
	cx1, cy1 := v.r16(dsClipx1), v.r16(dsClipy1)
	cx2, cy2 := v.r16(dsClipx2), v.r16(dsClipy2)
	rw := v.r16(esi + 2)
	ll := v.r16(dsLoglarg)
	flip := v.fs8(di, spNewf)&1 != 0
	v.w32(dsSrcSkip, int32(rw-cx2+cx1))
	ax := ll - 1 - cx2 + cx1
	if flip && v.mem[esi] != 1 {
		ax = -ax + ll + ll
	}
	v.w32(dsDstSkip, int32(ax))
	w = uint32(uint16(cx2 - cx1 + 1))
	rows = uint16(cy2 - cy1 + 1)
	var edx uint32
	if dy := cy1 - v.fs16(di, spNewy); dy > 0 {
		edx = uint32(uint16(dy)) * uint32(uint16(rw+1))
	}
	if dx := cx1 - v.fs16(di, spNewx); dx > 0 {
		if flip {
			dx = -dx
		}
		edx += uint32(int32(dx))
	}
	if flip {
		edx += uint32(uint16(rw)) + 1 - w
	}
	src = esi + edx + 8
	dst = v.ru32(dsWlogic) + uint32(uint16(cy1))*uint32(uint16(ll)) + uint32(int32(cx1))
	return
}

// setup4 is 0x127d4, setup8 for 4-bit resources: two pixels a byte, the
// first in the high nibble; 0x1bfc9 tells whether a row starts with a low
// nibble.
func (v *VM) setup4(di uint16, esi uint32) (src, dst, w uint32, rows uint16) {
	cx1, cy1 := v.r16(dsClipx1), v.r16(dsClipy1)
	cx2, cy2 := v.r16(dsClipx2), v.r16(dsClipy2)
	rw := v.r16(esi + 2)
	ll := v.r16(dsLoglarg)
	flip := v.fs8(di, spNewf)&1 != 0
	v.w32(dsSrcSkip, int32((rw-cx2+cx1)>>1))
	ax := ll - 1 - cx2 + cx1
	if flip {
		ax = -ax + ll + ll
	}
	v.w32(dsDstSkip, int32(ax))
	w = uint32(uint16(cx2 - cx1 + 1))
	rows = uint16(cy2 - cy1 + 1)
	v.mem[dsNibble] = 0
	var edx uint32
	if dy := cy1 - v.fs16(di, spNewy); dy > 0 {
		edx = uint32(uint16(dy)) * uint32(uint16(rw+1)) >> 1
	}
	if dx := cx1 - v.fs16(di, spNewx); dx > 0 {
		if dx&1 != 0 {
			v.mem[dsNibble] = 1
			if w&1 == 0 {
				v.w32(dsSrcSkip, v.r32(dsSrcSkip)-1)
			}
		}
		dx = int16(uint16(dx) >> 1)
		if flip {
			dx = -dx
			v.mem[dsNibble] = 0
		}
		edx += uint32(int32(dx))
	}
	if flip {
		if bx := rw + v.fs16(di, spNewx) - cx2; bx > 0 && bx&1 != 0 {
			v.mem[dsNibble] = 1
			if w&1 == 0 {
				edx--
			}
		}
		edx += uint32(int32(rw)+1-int32(w)) >> 1
	}
	src = esi + 8 + edx
	dst = v.ru32(dsWlogic) + uint32(uint16(cy1))*uint32(uint16(ll)) + uint32(int32(cx1))
	return
}

// blit8 is 0x1293d (transparent: colour 0 is not drawn) and 0x12a0f.
func (v *VM) blit8(src, dst, w uint32, rows uint16, transp bool) {
	m := v.mem
	ss, ds := v.ru32(dsSrcSkip), v.ru32(dsDstSkip)
	for {
		if transp {
			for range w {
				if c := m[src]; c != 0 {
					m[dst] = c
				}
				src++
				dst++
			}
		} else {
			for range w {
				m[dst] = m[src]
				src++
				dst++
			}
		}
		src += ss
		dst += ds
		if rows--; rows == 0 {
			return
		}
	}
}

// blit8mono is 0x12974: the opaque pixels in one colour.
func (v *VM) blit8mono(src, dst, w uint32, rows uint16, col uint8) {
	m := v.mem
	ss, ds := v.ru32(dsSrcSkip), v.ru32(dsDstSkip)
	for {
		for range w {
			if m[src] != 0 {
				m[dst] = col
			}
			src++
			dst++
		}
		src += ss
		dst += ds
		if rows--; rows == 0 {
			return
		}
	}
}

// blit8x is 0x129c7 and 0x12a24, mirrored: dst is the end of the first
// row.
func (v *VM) blit8x(src, dst, w uint32, rows uint16, transp bool) {
	m := v.mem
	ss, ds := v.ru32(dsSrcSkip), v.ru32(dsDstSkip)
	for {
		for range w {
			dst--
			if c := m[src]; c != 0 || !transp {
				m[dst] = c
			}
			src++
		}
		src += ss
		dst += ds
		if rows--; rows == 0 {
			return
		}
	}
}

// blit4 is 0x12a99 (transparent) and 0x12b6d: 4-bit pixels plus base.
func (v *VM) blit4(src, dst, w uint32, rows uint16, base uint8, transp bool) {
	m := v.mem
	ss, ds := v.ru32(dsSrcSkip), v.ru32(dsDstSkip)
	lowFirst := v.mem[dsNibble] != 0
	for {
		n := w
		low := lowFirst
		for {
			b := m[src]
			src++
			if !low {
				if c := b >> 4; c != 0 || !transp {
					m[dst] = c + base
				}
				dst++
				if n--; n == 0 {
					break
				}
			}
			low = false
			if c := b & 0xf; c != 0 || !transp {
				m[dst] = c + base
			}
			dst++
			if n--; n == 0 {
				break
			}
		}
		src += ss
		dst += ds
		if rows--; rows == 0 {
			return
		}
	}
}

// blit4x is 0x12af0 (transparent) and 0x12bba, mirrored. The opaque one
// steps back with dec di after a low nibble, so its offset wraps within 64
// KB there.
func (v *VM) blit4x(src, dst, w uint32, rows uint16, base uint8, transp bool) {
	m := v.mem
	ss, ds := v.ru32(dsSrcSkip), v.ru32(dsDstSkip)
	lowFirst := v.mem[dsNibble] != 0
	for {
		n := w
		low := lowFirst
		for {
			b := m[src]
			src++
			if !low {
				dst--
				if c := b >> 4; c != 0 || !transp {
					m[dst] = c + base
				}
				if n--; n == 0 {
					break
				}
			}
			low = false
			if transp {
				dst--
			} else {
				dst = dst&^0xffff | uint32(uint16(dst)-1)
			}
			if c := b & 0xf; c != 0 || !transp {
				m[dst] = c + base
			}
			if n--; n == 0 {
				break
			}
		}
		src += ss
		dst += ds
		if rows--; rows == 0 {
			return
		}
	}
}

// fill is 0x12bf3: the rectangle in the colour at +1.
func (v *VM) fill(di uint16, esi uint32) {
	v.mem[dsBlitCol] = v.mem[esi+1]
	_, dst, w, rows := v.setup8(di, esi)
	col := v.mem[dsBlitCol]
	ds := v.ru32(dsDstSkip)
	for {
		for range w {
			v.mem[dst] = col
			dst++
		}
		dst += ds
		if rows--; rows == 0 {
			return
		}
	}
}

// clearWindow is 0x124f9: the rectangle 0x15fcc of the work screen to 0,
// in dwords; a width that is not a multiple of 4 shifts the rows.
func (v *VM) clearWindow() {
	bp := v.r16(dsBlocy2) - v.r16(dsBlocy1)
	cx := uint16(v.r16(dsBlocx2) - v.r16(dsBlocx1) + 1)
	wl := v.ru16(dsWloglarg)
	skip := uint32(wl - cx)
	n := 4 * uint32(cx>>2)
	edi := v.ru32(dsWlogic) + uint32(v.ru16(dsBlocy1)*wl+v.ru16(dsBlocx1))
	for {
		clear(v.mem[edi : edi+n])
		edi += n + skip
		if bp--; bp < 0 {
			return
		}
	}
}

// copyRect is 0x1246f: the rectangle 0x15fcc of the work screen to video
// memory, in words.
func (v *VM) copyRect() {
	ll := v.ru16(dsLoglarg)
	off := uint32(v.ru16(dsBlocy1)*ll + v.ru16(dsBlocx1))
	src := v.ru32(dsWlogic) + off
	dst := v.ru32(dsPhysic) + off
	n := 2 * uint32(uint16(v.r16(dsBlocx2)-v.r16(dsBlocx1)+1)>>1)
	skip := uint32(int32(int16(ll - uint16(n))))
	rows := uint16(v.r16(dsBlocy2) - v.r16(dsBlocy1) + 1)
	for {
		copy(v.mem[dst:dst+n], v.mem[src:src+n])
		src += n + skip
		dst += n + skip
		if rows--; rows == 0 {
			return
		}
	}
}

// showRect is 0x123e8: copyRect with the cursor drawn into the logic
// screen while it is copied (0x1240a), the timer handler locked out.
func (v *VM) showRect() {
	shown := int8(v.mem[dsCurShown]) >= 0
	if shown && v.mem[dsCurLock] == 0xff {
		v.mem[dsCurLock] = 0
	}
	if shown {
		v.mem[dsCurVis] = v.cursorIn()
		if v.mem[dsCurVis] != 0 {
			phys := v.ru32(dsPhysic)
			v.wu32(dsPhysic, v.ru32(dsLogic))
			v.cursorDraw()
			v.wu32(dsPhysic, phys)
		}
	}
	v.copyRect()
	if shown && v.mem[dsCurVis] != 0 {
		phys := v.ru32(dsPhysic)
		v.wu32(dsPhysic, v.ru32(dsLogic))
		v.cursorRestore()
		v.wu32(dsPhysic, phys)
	}
	if shown {
		v.mem[dsCurLock] = 0xff
	}
}

// cursorIn is 0x9964: whether the cursor meets the rectangle 0x15fcc.
func (v *VM) cursorIn() uint8 {
	if int8(v.mem[dsCurShown]) < 0 {
		return 0
	}
	x, y := v.r16(dsCurX), v.r16(dsCurY)
	if x > v.r16(dsBlocx2) || y > v.r16(dsBlocy2) ||
		x+v.r16(dsCurW) < v.r16(dsBlocx1) || y+v.r16(dsCurH) < v.r16(dsBlocy1) {
		return 0
	}
	return 1
}

// cursorRect is the cursor's place on the screen 0x178eb, and its size:
// 16x16 cut at the right and bottom edges.
func (v *VM) cursorRect() (at uint32, cols, rows uint16) {
	x, y := v.r16(dsCurX), v.r16(dsCurY)
	at = uint32(uint16(y)*v.ru16(dsLoglarg)) + uint32(uint16(x))
	rows = 16
	if y >= 0xb8 {
		rows = uint16(200 - y)
	}
	cols = 16
	if x >= 0x130 {
		cols = uint16(320 - x)
	}
	return
}

// cursorSave is 0x13b28: what the cursor covers, after its offset and
// size, to the buffer 0x16070.
func (v *VM) cursorSave() {
	at, cols, rows := v.cursorRect()
	edi := v.ru32(dsCurSave)
	v.wu32(edi, at)
	v.wu16(edi+4, cols)
	v.wu16(edi+6, rows)
	edi += 8
	esi := v.ru32(dsPhysic) + at
	n := uint32(int32(int16(cols)))
	skip := uint32(int32(int16(v.ru16(dsLoglarg) - cols)))
	for {
		copy(v.mem[edi:edi+n], v.mem[esi:esi+n])
		edi += n
		esi += n + skip
		if rows--; rows == 0 {
			return
		}
	}
}

// cursorRestore is 0x13bbf.
func (v *VM) cursorRestore() {
	esi := v.ru32(dsCurSave)
	edi := v.ru32(dsPhysic) + v.ru32(esi)
	cols, rows := v.ru16(esi+4), v.ru16(esi+6)
	esi += 8
	n := uint32(int32(int16(cols)))
	skip := uint32(int32(int16(v.ru16(dsLoglarg) - cols)))
	for {
		copy(v.mem[edi:edi+n], v.mem[esi:esi+n])
		esi += n
		edi += n + skip
		if rows--; rows == 0 {
			return
		}
	}
}

// cursorDraw is 0x13a36: save, then draw the cursor sprite: type 0 is
// 4-bit, 0x10 4-bit plus a colour base, anything else 8-bit; colour 0 is
// clear.
func (v *VM) cursorDraw() {
	v.cursorSave()
	at, cols, rows := v.cursorRect()
	m := v.mem
	edi := v.ru32(dsPhysic) + at
	bp := uint32(int32(int16(v.ru16(dsLoglarg) - cols)))
	ax := uint16(16) - cols
	esi := v.ru32(dsCurSprite)
	var base uint8
	switch m[esi] {
	case 0:
		esi += 6
	case 0x10:
		base = m[esi+6]
		esi += 8
	default:
		esi += 8
		skip := uint32(int32(int16(ax)))
		n := uint32(int32(int16(cols)))
		for {
			for range n {
				if c := m[esi]; c != 0 {
					m[edi] = c
				}
				esi++
				edi++
			}
			esi += skip
			edi += bp
			if rows--; rows == 0 {
				return
			}
		}
	}
	skip := uint32(ax >> 1)
	for {
		n := uint32(uint8(cols))
		for {
			b := m[esi]
			esi++
			if c := b >> 4; c != 0 {
				m[edi] = c + base
			}
			edi++
			if n--; n == 0 {
				break
			}
			if c := b & 0xf; c != 0 {
				m[edi] = c + base
			}
			edi++
			if n--; n == 0 {
				break
			}
		}
		esi += skip
		edi += bp
		if rows--; rows == 0 {
			return
		}
	}
}
