package alis

// Darkness, angles, follow/view, films and terrain walking (alis opcodes.c,
// Robinson's Requiem DOS paths).

// cdarkpal sets the darkness colour of n palette entries from first
// (START.EXE 0x87e8: unsigned first/n, no range check; the port stops at
// the table's end) and flags darkness unless every word after entry 0 is
// 0x100 (alis checked only the first 0xb6 words).
func cdarkpal(v *VM) {
	var p [5]int16
	for i := range p {
		v.readexecOpername()
		p[i] = int16(v.varD7)
	}
	r, g, b, first, n := p[0], p[1], p[2], int(uint16(p[3])), int(uint16(p[4]))
	for i := first; i < first+n && i*3+2 < len(v.img.dkpalet); i++ {
		v.img.dkpalet[i*3+0] = r
		v.img.dkpalet[i*3+1] = g
		v.img.dkpalet[i*3+2] = b
	}
	v.img.dkpalet[0], v.img.dkpalet[1], v.img.dkpalet[2] = 0, 0, 0
	v.img.fdarkpal = 0
	for i := 3; i < 0x300; i++ {
		if v.img.dkpalet[i] != 0x100 {
			v.img.fdarkpal = 1
			break
		}
	}
}

func caset(v *VM) {
	vram := v.script.vramOrg
	for _, o := range []uint32{scrWCAX, scrWCAY, scrWCAZ} {
		v.readexecOpername()
		v.w16(vram+o, int16(v.varD7))
	}
}

func camov(v *VM) {
	vram := v.script.vramOrg
	for _, o := range []uint32{scrWCAX, scrWCAY, scrWCAZ} {
		v.readexecOpername()
		v.add16(vram+o, int16(v.varD7))
	}
}

func cfilm(v *VM) {
	if v.film.fh != nil { // 0x8e2f drops the handle and leaks the buffer
		v.film.fh.Close()
	}
	v.film = film{}
	v.filmnode = 0
	v.readexecOpername()
	v.film.id = int16(v.varD7)
	v.readexecOpername()
	addr := uint32(v.adresdes(v.varD7)) + 0x20 + 4
	v.film.addr1 = addr
	v.film.addr2 = addr
	v.readexecOpername()
	v.film.waitclock = int16(v.varD7)
	v.film.basemain = v.basemain
	v.inifilm()
	v.varD7 = int32(v.film.frames)
	v.cstoreContinue()
}

// The land opcodes' trigonometry uses the 9-bit sine tables at 0x19d42 and
// 0x19c8e, indexed by twice the angle in degrees.
func (v *VM) tsin(i int32) int16 { return v.r16(uint32(0x19d42 + i)) }
func (v *VM) tcos(i int32) int16 { return v.r16(uint32(0x19c8e + i)) }

// rolpos is a position as 16.16 value (rol of its dword: the integer word
// comes first).
func (v *VM) rolpos(a uint32) uint32 { return uint32(v.r32wc(a, 0)) }

// landTurn is 0xcf0d: (ax, bx, cx) turned by the pitch dx and the heading
// di, as 16.16 values; edx is left equal to ebx.
func (v *VM) landTurn(ax, bx, cx, dx, di int16) (eax, ebx, ecx uint32) {
	esi, ebp := int32(dx)*2, int32(di)*2
	ecx = uint32(mul16(cx, v.tsin(esi))+mul16(bx, v.tcos(esi))) << 7
	w := int16(mul16(v.tcos(ebp), v.tsin(esi)) >> 9)
	eax = uint32(mul16(ax, v.tsin(ebp))) - uint32(mul16(w, bx))<<7
	w = int16(mul16(v.tsin(esi), v.tsin(ebp)) >> 9)
	ebx = uint32(mul16(w, bx))<<7 + uint32(mul16(ax, v.tcos(ebp)))
	return
}

// landAtan is 0xcff9: the angle and the length of the vector (ax, bx),
// from the arc tangent bytes at 0x1a2e4.
func (v *VM) landAtan(ax, bx int16) (int16, int16) {
	cx := ax
	si, di := ax, bx
	var bp int16
	switch {
	case si == 0:
		switch {
		case bx == 0:
			bx = 0
		case bx < 0:
			bp = 0xb4
		}
		goto done
	case di == 0:
		goto vertical
	}
	if si < 0 {
		si = -si
	}
	if di < 0 {
		di = -di
	}
	if di >= si {
		edx := uint32(int32(di) << 5)
		bp = 0x5a
		if uint16(si) > uint16(edx>>16) {
			if q := uint16(edx / uint32(uint16(si))); q < 0x740 {
				bp = int16(int8(v.mem[0x1a2e4+uint32(int32(int16(q)))]))
				if bp == 0 {
					goto vertical
				}
			}
		}
		if bx < 0 {
			bp = -bp
		}
		bp -= 0x5a
		if cx < 0 {
			bp = -bp
		}
		bx = v.idivMem(int32(bx)<<9, v.tsin(int32(bp)*2))
	} else {
		edx := uint32(int32(si) << 5)
		bp = 0x5a
		if uint16(di) > uint16(edx>>16) {
			if q := uint16(edx / uint32(uint16(di))); q < 0x740 {
				bp = int16(int8(v.mem[0x1a2e4+uint32(int32(int16(q)))]))
				if bp == 0 {
					bp = 0
					switch {
					case bx == 0:
						bx = 0
					case bx < 0:
						bp = 0xb4
					}
					goto done
				}
			}
		}
		if bx < 0 {
			bp = -bp + 0xb4
		}
		if cx >= 0 {
			bp = -bp
		}
		bx = v.idivMem(int32(cx)<<9, v.tcos(int32(bp)*2))
	}
	goto done
vertical:
	bx = cx
	bp = -0x5a
	if cx < 0 {
		bp = 0x5a
	}
done:
	if bx < 0 {
		bx = -bx
	}
	return bp, bx
}

// landAim is 0xcfc6: the heading, the pitch and the distance from (dx, si,
// di) to (ax, bx, cx).
func (v *VM) landAim(ax, bx, cx, dx, si, di int16) (int16, int16, int16) {
	ax, bx, cx = ax-dx, bx-si, cx-di
	head, l := v.landAtan(ax, bx)
	var pitch int16
	if cx != 0 {
		pitch, l = v.landAtan(-cx, l)
	}
	return head, pitch, l
}

// landAlti is 0xd5e6: the terrain height (cx) and map cell (dx) at the map
// position (ax, bx) of the land at gs, for the entity at ebp. Outside the
// map cx is the map shift and dx is kept.
func (v *VM) landAlti(ax, bx, dx uint16, ebp uint32) (uint16, uint16) {
	cx := v.gsu16(0x840)
	ax = uint16(int16(ax) >> (cx & 31))
	bx = uint16(int16(bx) >> (cx & 31))
	if bx > v.gsu16(0x96a) || ax > v.gsu16(0x968) {
		return cx, dx
	}
	eax := v.ru32(uint32(ax)*4 + v.ru32(0x1aa50))
	eax = eax&0xffff0000 | uint32(uint16(eax)+bx*2)
	dx = v.ru16(eax)
	cx = uint16(uint8(dx))
	di := uint32(dx&0x3f00) >> 3
	f := int8(v.mem[v.gs(di+0x14)])
	switch {
	case f == 0:
	case f < 0:
		eax += v.gs32(di + 0x10)
		b := uint16(v.mem[eax])
		d := b - cx
		t := -d + v.gsu16(di+0x16)
		if int16(t) >= int16(d) || int16(b-t) <= v.r16(ebp+scrWCZ) {
			dx = v.ru16(eax)
			cx = uint16(uint8(dx))
		}
	case v.matmask() == 0 || v.gsu16(di+0x1a) == 0:
	case f == 1:
		if v.gs32(di+0x10) != 0 {
			cx += 0xff
		}
	default:
		cx += v.gsu16(di + 0x16)
	}
	return cx, dx
}

// landPitch is 0xd59c: the entity's pitch turns towards ax by a quarter of
// the difference, 1 to 15 degrees, within ±0x4b.
func (v *VM) landPitch(ax int16, ebp uint32) {
	ax -= v.r16(ebp + scrWCAX)
	if ax == 0 {
		return
	}
	ax >>= 2
	if ax < 0 {
		ax = max(ax, -0xf)
	} else {
		if ax == 0 {
			ax = 1
		}
		ax = min(ax, 0xf)
	}
	ax += v.r16(ebp + scrWCAX)
	v.w16(ebp+scrWCAX, min(max(ax, -0x4b), 0x4b))
}

// landWalk is 0xd127: the entity at ebp steps by its x/y speed turned by
// its heading onto the terrain, unless the height changes by more than
// [0x1aad8]; [0x1aada] > 0 also pitches it along the slope ahead. It
// returns the height change.
func (v *VM) landWalk(ebp uint32) int16 {
	const (
		d0, d4, nx, ny, nz, z = 0x1aad0, 0x1aad4, 0x1aac4, 0x1aac8, 0x1aacc, 0x1aace
		haut, tete            = 0x1aad8, 0x1aada
	)
	v.setMatmask(0)
	if v.r16(tete) < 0 {
		v.setMatmask(v.r16(tete))
	}
	edi := int32(v.r16(ebp+scrWCAZ)) * 2
	bx, cx := int16(int8(v.mem[ebp+scrWCX2])), int16(int8(v.mem[ebp+scrWCY2]))
	v.wu32(d0, uint32(mul16(v.tsin(edi), bx)-mul16(v.tcos(edi), cx))<<7)
	v.wu32(nx, v.ru32(d0)+v.rolpos(ebp+scrWCX))
	v.wu32(d4, uint32(mul16(v.tsin(edi), cx)+mul16(v.tcos(edi), bx))<<7)
	v.wu32(ny, v.ru32(d4)+v.rolpos(ebp+scrWCY))
	v.wu16(z, v.ru16(ebp+scrWCZ))
	alti := func(x, y uint32) uint16 {
		c, _ := v.landAlti(uint16(x>>16), uint16(y>>16), 0, ebp)
		return c
	}
	h := alti(v.ru32(nx), v.ru32(ny))
	v.wu16(nz, h)
	h -= v.ru16(z)
	// the corners and the cells ahead: a wall (0xff and up) stops the step
	px, py := v.rolpos(ebp+scrWCX), v.rolpos(ebp+scrWCY)
	a, b := v.ru32(d0), v.ru32(d4)
	for _, p := range [][2]uint32{
		{px + a + b, py + b + a}, {px + a - b, py + b - a}, {px + 2*a, py + 2*b},
		{px + 2*a + b, py + 2*b + a}, {px + 2*a - b, py + 2*b - a},
	} {
		if c := alti(p[0], p[1]); int16(c) >= 0xff {
			v.wu16(nz, c)
			h = c - v.ru16(z)
		}
	}
	if v.r16(tete) > 0 {
		ah := h
		if int16(ah) < 0 {
			ah = -ah
		}
		if int16(ah) <= v.r16(haut) {
			g := v.r16(v.gs(0x80c))
			a = uint32(mul16(-g, v.tcos(edi))) << 7
			b = uint32(mul16(g, v.tsin(edi))) << 7
			v.wu32(d0, a)
			v.wu32(d4, b)
			si := (alti(px+a, py+b) - v.ru16(z)) << 2
			si += (alti(px+2*a, py+2*b) - v.ru16(z)) << 1
			si += alti(px+4*a, py+4*b) - v.ru16(z)
			ax := si
			if int16(ax) < 0 {
				ax = -ax
			}
			// the slope's sign is lost: ax stays |si| (0x3c0 only clears si)
			p := mul16(int16(ax), int16(int8(v.mem[tete+1])))
			// edx holds p>>2 word-swapped, eax's upper half the low word of
			// px+4a from 0xd4df (0xd5e6 keeps eax)
			e := uint32(p >> 2)
			eax, _ := v.xdiv(DivSigned|DivWord, rol16(e), uint32(uint16(px+4*a))<<16|e&0xffff, 0xc)
			q := int16(eax)
			c := q
			if p>>2 < 0 { // idiv leaves the flags of the sar before it
				c = -c
			}
			if uint8(c) < v.mem[tete] {
				q = 0
			}
			v.landPitch(q, ebp)
		}
	}
	d := v.ru16(nz) - v.ru16(z)
	if int16(d) < 0 {
		d = -d
	}
	if int16(d) <= v.r16(haut) {
		v.w32wc(ebp, scrWCX, int32(v.ru32(nx)))
		v.w32wc(ebp, scrWCY, int32(v.ru32(ny)))
		v.wu16(ebp+scrWCZ, v.ru16(nz))
	}
	return int16(v.ru16(nz) - v.ru16(z))
}

// cwalkmap (0x4e14) walks the entity on the land of a map variable; D7 is
// the height change.
func cwalkmap(v *VM) {
	a := v.readVaddr()
	v.wu16(0x1aad8, uint16(v.getval()))
	v.wu16(0x1aada, uint16(v.getval()))
	v.wu16(0x1aadc, uint16(v.getval()))
	v.wu32(dsGsBase, a-0xc00)
	v.varD7 = int32(v.landWalk(v.script.vramOrg))
	v.cstoreContinue()
}

// catstmap (0x4e57) stores the map cell under the entity's position moved
// by (x, y, z) turned by its angles; D6 is the height there.
func catstmap(v *VM) {
	a := v.readVaddr()
	ax := v.getval()
	bx := v.getval()
	v.readexecOpernameSaveD6()
	vram := v.script.vramOrg
	eax, ebx, _ := v.landTurn(ax, bx, int16(v.varD6), v.r16(vram+scrWCAX), v.r16(vram+scrWCAZ))
	v.wu32(dsGsBase, a-0xc00)
	x, y := eax+v.rolpos(vram+scrWCX), ebx+v.rolpos(vram+scrWCY)
	v.setMatmask(0)
	cx, dx := v.landAlti(uint16(x>>16), uint16(y>>16), uint16(ebx), vram)
	v.varD6 = int32(int16(cx))
	v.varD7 = int32(int16(dx))
	v.cstoreContinue()
}

// cavtstmov (0x73f5) tests the entity's forms at its position moved by its
// speed turned by its angles.
func cavtstmov(v *VM) {
	vram := v.script.vramOrg
	eax, ebx, ecx := v.landTurn(int16(int8(v.mem[vram+scrWCX2])), int16(int8(v.mem[vram+scrWCY2])),
		int16(int8(v.mem[vram+scrWCZ2])), v.r16(vram+scrWCAX), v.r16(vram+scrWCAZ))
	v.wcx = int16((eax + v.rolpos(vram+scrWCX)) >> 16)
	v.wcy = int16((ebx + v.rolpos(vram+scrWCY)) >> 16)
	v.wcz = int16((ecx + v.rolpos(vram+scrWCZ)) >> 16)
	v.setMatmask(v.getval())
	v.wforme = v.cforme(vram)
	v.clipform()
	crstent(v)
}

// cavmov (0x5e7c) moves the entity by the integer part of its speed turned
// by its angles; dx is left with the low word of the y move, cx with z.
func cavmov(v *VM) {
	vram := v.script.vramOrg
	eax, ebx, ecx := v.landTurn(int16(int8(v.mem[vram+scrWCX2])), int16(int8(v.mem[vram+scrWCY2])),
		int16(int8(v.mem[vram+scrWCZ2])), v.r16(vram+scrWCAX), v.r16(vram+scrWCAZ))
	v.add16(vram+scrWCX, int16(eax>>16))
	v.add16(vram+scrWCY, int16(ebx>>16))
	v.add16(vram+scrWCZ, int16(ecx>>16))
	v.varD6 = int32(int16(ecx >> 16))
	v.varD7 = int32(int16(ebx))
}

// caim (0x5f57) stores the pitch, the heading turned towards (x, y, z) by
// at most the fourth operand (none from 0xb4), and the distance.
func caim(v *VM) {
	ax := v.getval()
	bx := v.getval()
	cx := v.getval()
	vram := v.script.vramOrg
	head, pitch, dist := v.landAim(ax, bx, cx, v.r16(vram+scrWCX), v.r16(vram+scrWCY), v.r16(vram+scrWCZ))
	if dist == 0 {
		head = v.r16(vram + scrWCAZ)
	}
	dx := v.getval()
	ax = head
	if uint16(dx) < 0xb4 {
		bx := v.r16(vram + scrWCAZ)
		ax = head - bx
		up := func() { // 0x5fc4
			ax = min(ax, dx) + bx
		}
		down := func() { // 0x5fdd
			ax = max(ax, -dx) + bx
		}
		switch {
		case ax >= 0 && ax < 0xb4:
			up()
		case ax >= 0:
			if ax -= 0x168; ax < 0 {
				down()
			} else {
				up()
			}
		case ax > -0xb4:
			down()
		default:
			if ax += 0x168; ax >= 0 {
				up()
			} else {
				down()
			}
		}
	}
	if ax >= 0x168 {
		ax -= 0x168
	} else if ax <= -0x168 {
		ax += 0x168
	}
	v.varD6 = int32(head)
	for _, x := range []int16{pitch, ax, dist} {
		v.varD7 = int32(x)
		v.cstoreContinue()
	}
}

func cpointpix(v *VM) {
	v.getval()
	v.readexecOpernameSaveD6()
	v.pointpix(int16(v.varD7), int16(v.varD6))
	for _, a := range []uint32{0x1aae8, 0x1aaea, 0x1aaec, 0x1aaee, 0x1aaf0, 0x1aaf2} {
		v.varD7 = int32(v.r16(a))
		v.cstoreContinue()
	}
}
