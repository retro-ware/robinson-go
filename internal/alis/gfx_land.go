package alis

// The land scenes (START.EXE 0xbb70..0xfd00): a scene whose map lives in
// an entity's variables is drawn into its own buffer (+0x30) by the land
// renderer, with its elements interleaved by depth, and then copied into
// the logic screen.

// landTables is 0xfb76 (start-up): the sine and cosine tables are
// repeated so that angles from -720 to 719 index them directly.
func (v *VM) landTables() {
	for i := uint32(0); i < 0x168*2; i += 2 {
		w := v.ru16(0x19c8e + i)
		v.wu16(0x199be+i, w)
		v.wu16(0x196ee+i, w)
	}
	for i := uint32(0); i < 0x169*2; i += 2 {
		v.wu16(0x1a012+i, v.ru16(0x19d42+i))
	}
}

// landBase points gs at the land variables of scene a (0x472): 0xc00
// bytes below the map variable +0x42 of entity +0x40.
func (v *VM) landBase(a uint32) {
	e := v.ru32(v.atent + uint32(int32(v.r16(a+0x40))))
	v.wu32(dsGsBase, e+uint32(int32(v.r16(a+0x42)))-0xc00)
}

// openland is 0xbb70, for copensc: the scene's buffer and the shared land
// buffers are allocated, and the land set up.
func (v *VM) openland(a uint32) {
	v.landBase(a)
	eax := v.ru32(dsGsBase) // 0x472 keeps the gs base in eax; 0xfa29 gets it
	if v.ru32(a+0x30) == 0 {
		// 0xbba5 computes the width and drops it: rows of 0x140
		h := v.memalloc(0x140 * (uint32(v.ru16(a+0x14)+1) + 8))
		v.wu32(a+0x30, h)
		v.wu32(h, v.ru32(h)+0x500)
		eax = h
	}
	if v.ru32(0x1aa5c) == 0 {
		h := v.memalloc(uint32(v.gsu16(0x834))*4 + 0x18b20)
		v.wu32(0x1aa5c, h)
		p := v.ru32(h)
		v.wu32(0x1aa60, p)
		v.wu32(0x1aa58, p+0x16800)
		v.wu32(0x1aa70, p+0x16800+0x320)
		v.wu32(0x1aa50, p+0x16800+0x320+0x2000)
		eax = p + 0x16800 + 0x320 + 0x2000
	}
	v.wu16(a+0x1a, 0x14)
	v.wu16(a+0x34, 0)
	v.wu16(a+0x36, 0)
	v.wu16(a+0x38, 0)
	v.landFA29(a, eax)
	v.landF91C(a)
	v.landF7E6(a)
	v.mem[0x1aa74] = 0
}

// memalloc is 0x1c58 as the land code calls it: the handle, or 0.
func (v *VM) memalloc(n uint32) uint32 { return uint32(v.ioMalloc(int32(n))) }

// landWindow makes the scene's buffer the window being drawn
// (0xbc83..0xbcb5, 0xbe86..0xbeb2).
func (v *VM) landWindow(a uint32) {
	v.wu16(dsWloglarg, 0x140)
	buf := v.ru32(v.ru32(a + 0x30))
	v.wu32(dsWlogic, buf)
	v.wu32(dsBacklog, buf)
	v.wu16(dsWlogx1, v.ru16(a+0xe))
}

// landWindowEnd gives the logic screen back as the window (0xbe19).
func (v *VM) landWindowEnd() {
	v.wu32(dsWlogic, v.ru32(dsLogic))
	v.wu16(dsWlogx1, v.ru16(dsLogx1))
	v.wu16(dsWlogy1, v.ru16(dsLogy1))
	v.wu16(dsWloglarg, v.ru16(dsLoglarg))
}

// landFull marks whether the window is the whole scene (0x1aa74).
func (v *VM) landFull(a uint32) {
	v.mem[0x1aa74] = 0
	if v.ru16(a+0x12)+1 == v.ru16(dsFenl) && v.ru16(a+0x14)+1 == v.ru16(dsFenh) {
		v.mem[0x1aa74] = 1
	}
}

// affiland is 0xbc6e: a changed land scene is drawn whole.
func (v *VM) affiland(a uint32) {
	v.wu32(0x1aa48, a)
	v.landWindow(a)
	v.wu16(dsWlogy1, v.ru16(a+0x10))
	v.landBase(a)
	x, y := v.ru16(a+0xe), v.ru16(a+0x10)
	v.wu16(dsFenx1, x)
	v.wu16(dsFeny1, y)
	v.wu16(0x1ab38, y)
	v.wu16(dsBlocx1, x)
	v.wu16(dsBlocy1, y)
	x += v.ru16(a + 0x12)
	y += v.ru16(a + 0x14)
	v.wu16(dsFenx2, x)
	v.wu16(0x1ab3a, x)
	v.wu16(dsFeny2, y)
	v.wu16(dsBlocx2, x)
	v.wu16(dsBlocy2, y)
	v.wu16(dsFenl, x-v.ru16(dsFenx1)+1)
	h := y - v.ru16(dsFeny1) + 1
	v.wu16(dsFenh, h)
	v.wu16(0x1ab3c, h)
	if v.mem[0x1aae2] != 0 { // pointpix
		v.landF445(a)
		v.landE648(a)
		v.landWindowEnd()
		return
	}
	if v.mem[a]&0x80 == 0 {
		v.landWindowEnd()
		return
	}
	v.landC365(a)
	v.landSort(a, v.ru16(a+2))
	v.landF445(a)
	if v.mem[a+1]&0x40 == 0 && v.ru16(a+0xa2) == 0 {
		v.clearWindow()
	}
	v.landE648(a)
	v.land135EA()
	v.landFull(a)
	v.mem[0x1aa76] = 1
	v.landWindowEnd()
}

// landRedraw is 0xbe63: a land scene within the rectangle being redrawn;
// the land is drawn again only when the rectangle is not the whole scene
// or the scene was not drawn whole yet.
func (v *VM) landRedraw(a uint32, id, di uint16) {
	feny1, fenh := v.ru16(dsFeny1), v.ru16(dsFenh)
	v.wu32(0x1aa48, a)
	v.landWindow(a)
	v.wu16(0x1ab3a, v.ru16(a+0xe)+v.ru16(a+0x12))
	y := v.ru16(a + 0x10)
	v.wu16(dsWlogy1, y)
	v.wu16(0x1ab38, y)
	v.wu16(dsFeny1, y)
	h := v.ru16(dsFeny2) - y + 1
	v.wu16(0x1ab3c, h)
	v.wu16(dsFenh, h)
	if v.mem[0x1aa74] == 0 || v.mem[dsLandRd] != 0 {
		v.landFull(a)
		v.landBase(a)
		v.landF445(a)
		if v.mem[a+1]&0x40 == 0 && v.ru16(a+0xa2) == 0 {
			v.clearWindow()
		}
		v.landE648(a)
		v.mem[dsLandRd] = 0
	}
	v.wu16(dsFenh, fenh)
	v.wu16(dsFeny1, feny1)
	v.land135EA()
	v.landWindowEnd()
}

// landSort is 0xbfc8: the scene's changed elements are placed and sorted
// into its draw list, the erased ones freed.
func (v *VM) landSort(a uint32, cx uint16) {
	for di := cx; ; {
		bx := di
		if di = v.fsu16(di, spLink); di == 0 {
			return
		}
		switch v.fs8(di, spState) {
		case 0:
		case 1:
			v.setFsu16(bx, spLink, v.fsu16(di, spLink))
			v.freeSprite(di)
			di = bx
		default:
			v.landPosition(a, di)
			v.storeNew(di, true)
			v.setFsu16(bx, spLink, v.fsu16(di, spLink))
			v.insert(di, cx)
			di = bx
		}
	}
}

// landPosition is 0xc0b3: where element di of land scene a is drawn,
// into 0x16014..0x16026.
func (v *VM) landPosition(a uint32, di uint16) {
	ecx := v.fsu32(di, spData)
	if n := int8(v.fs8(di, spCreduc)); n > 1 {
		ax := v.fsu16(di, spColour) - v.ru16(a+0x38) + 0x384
		if int16(ax) >= 0 {
			for int16(ax) >= 0 {
				ax -= 0x168
			}
		}
		ax += 0x168
		p := uint32(ax)*uint32(uint16(int16(n))) + 0xb4
		ax = uint16(p / 0x168) // ax < 0x168 and n < 0x80: it fits
		if int8(ax) >= n {
			ax = 0
		}
		ecx += uint32(int32(int16(ax << 2)))
	}
	esi := uint32(0x1ab60)
	ebx := ecx + v.ru32(ecx)
	if ebx > v.ru32(0x15f2a) {
		return
	}
	if v.mem[ebx] == 3 {
		esi = ebx
		ecx += uint32(int32(v.r16(ebx+2))) * 4
		ebx = ecx + v.ru32(ecx)
	}
	v.wu32(dsNewad, ecx)
	v.mem[dsNewf] = v.fs8(di, spInvx) ^ v.mem[esi+1]
	var cl uint8
	if v.fs8(di, spCredon) == 0x80 {
		v.w16(dsNewx, v.fs16(di, spDepx)+v.r16(a+0xa))
		v.w16(dsNewy, v.r16(a+0xc)-v.fs16(di, spDepz))
		v.w16(dsNewd, v.fs16(di, spDepy))
		v.wu16(dsNewzoomy, v.fsu16(di, spZoomy))
		v.wu16(dsNewzoomx, v.fsu16(di, spZoomx))
	} else {
		p := v.landProj(a, v.fs16(di, spDepx)-v.r16(a+0x16), v.fs16(di, spDepy)-v.r16(a+0x18),
			v.fs16(di, spDepz)-v.r16(a+0x1a))
		v.w16(dsNewx, p.si+v.r16(a+0xa))
		v.w16(dsNewy, p.di+v.r16(a+0xc))
		v.wu16(dsNewd, uint16(p.ebx))
		v.w16(dsNewzoomy, p.dx)
		v.w16(dsNewzoomx, p.dx)
		cl = v.fs8(di, spCredon)
		if int8(cl) >= 8 {
			z := uint16(uint32(uint16(p.dx))*uint32(cl-8)) >> 3
			v.wu16(dsNewzoomy, z)
			v.wu16(dsNewzoomx, z)
			cl = 3
		}
	}
	cl += 9
	rnd := v.ru32(0x1aa04 + uint32(cl)*4)
	sh := cl & 31
	size := func(n int16, zoom uint32) int16 {
		e := (mul16(n+1, v.r16(zoom))*2 + int32(rnd)) >> sh
		if e-1 < 0 {
			return 0
		}
		return int16(e - 1)
	}
	v.w16(dsNewh, size(v.r16(ebx+4), dsNewzoomx))
	v.w16(dsNewl, size(v.r16(ebx+2), dsNewzoomy))
	off := func(n uint16, o int16, zoom uint32) int16 {
		return int16(-mul16(int16(n>>1)-o, v.r16(zoom)) * 2 >> sh)
	}
	v.w16(dsNewy, v.r16(dsNewy)+off(v.ru16(ebx+4), v.r16(esi+6), dsNewzoomx))
	v.w16(dsNewx, v.r16(dsNewx)+off(v.ru16(ebx+2), v.r16(esi+4), dsNewzoomy))
}

// landSprites is 0xc2ee: the elements from 0x1ab02 on that lie deeper than
// d are drawn into the land; it stops at the first one that does not.
func (v *VM) landSprites(d int16) {
	di := v.ru16(0x1ab02)
	v.wu16(0x1ab28, 0)
	ax := uint16(0x8000)
	for ; di != 0; di = v.fsu16(di, spLink) {
		nd := v.fs16(di, spNewd)
		if nd <= d {
			ax = uint16(nd)
			break
		}
		if int8(v.fs8(di, spState)) < 0 || int8(v.fs8(di, spNewf)) < 0 || nd < 0 {
			continue
		}
		v.landSprite(v.ru32(dsFsBase) + uint32(di))
	}
	v.wu16(0x1ab02, di)
	v.wu16(0x1ab00, ax)
}

// landC365 is 0xc365: the camera of scene a in the land variables: the
// eye, the heading and the corners of the view on the map.
func (v *VM) landC365(a uint32) {
	v.w16(a+0x94, v.r16(a+0x6c)+2*v.gs16(0x80e))
	v.setGs16(0x990, v.r16(a+0xa))
	v.setGs16(0x992, v.r16(a+0xc))
	for _, o := range []struct{ sc, g1, g2 uint32 }{{0x16, 0x994, 0x89a}, {0x18, 0x996, 0x89c}, {0x1a, 0x998, 0x89e}} {
		x := v.r16(a + o.sc)
		v.setGs16(o.g1, x)
		v.setGs16(o.g2, x)
	}
	t := uint32(int32(v.r16(a+0x34))) * 2
	v.wu16(a+0xaa, v.ru16(0x19d42+t))
	v.setGsu16(0x96e, v.ru16(0x19d42+t))
	v.wu16(a+0xac, v.ru16(0x19c8e+t))
	v.setGsu16(0x96c, v.ru16(0x19c8e+t))
	t = uint32(int32(v.r16(a+0x38))) * 2
	v.wu16(a+0xb2, v.ru16(0x19d42+t))
	v.setGsu16(0x972, v.ru16(0x19d42+t))
	v.setGsu16(0x970, v.ru16(0x19c8e+t))
	v.wu16(a+0xb4, v.ru16(0x19c8e+t))

	m := v.gs16(0x80c)
	corner := func(ax, bx int16, g, cell uint32) {
		r := v.landRot(ax, bx, 0)
		v.setGsu16(g, uint16(r.eax))
		v.setGsu16(g+2, uint16(r.ebx))
		v.setGs16(cell, r.si)
		v.setGs16(cell+2, r.di)
	}
	w, far := v.r16(a+0x6a)>>1, v.r16(a+0x6c)
	corner(-w-m-m, far, 0x882, 0x85e)
	corner(w-m-m, far, 0x888, 0x864)
	w, near := v.r16(a+0x64)>>1, v.r16(a+0x66)
	corner(-w-m-m, near, 0x88e, 0x86a)
	corner(w-m-m, near, 0x894, 0x870)
	corner(-m, far, 0x8a6, 0x87c)
	corner(-m, near, 0x8a0, 0x876)

	sh := uint(v.gsu16(0x840)) & 31
	proj := func(cell, g, s uint32) landProj {
		x := int16(v.gsu16(cell)<<sh) - v.r16(a+0x16)
		y := int16(v.gsu16(cell+2)<<sh) - v.r16(a+0x18)
		p := v.landProj(a, x, y, -v.r16(a+0x1a))
		v.setGs32(g, p.eax)
		v.setGs32(g+4, p.ebx)
		v.setGs32(g+8, p.ecx)
		v.setGs16(s, p.si)
		v.setGs16(s+2, p.di)
		v.setGsu16(s+4, uint16(p.ebx))
		return p
	}
	proj(0x85e, 0x8d0, 0x8ac)
	proj(0x864, 0x8dc, 0x8b2)
	proj(0x86a, 0x8e8, 0x8b8)
	proj(0x870, 0x8f4, 0x8be)
	// 0xc75a and 0xc817 compute the middle of two corners and drop it
	store := func(g, s uint32, p landProj) {
		v.setGs32(g, p.eax)
		v.setGs32(g+4, p.ebx)
		v.setGs32(g+8, p.ecx)
		v.setGs16(s, p.si)
		v.setGs16(s+2, p.di)
		v.setGsu16(s+4, uint16(p.ebx))
	}
	store(0x900, 0x8c4, v.landProj(a, v.gs16(0x8a6)-v.r16(a+0x16), v.gs16(0x8a8)-v.r16(a+0x18), -0x100))
	p := v.landProj(a, int16(v.gsu16(0x87c)<<sh)-v.r16(a+0x16), int16(v.gsu16(0x87e)<<sh)-v.r16(a+0x18), -0x100)
	v.wu32(0x1ab04, p.ebx)
	store(0x90c, 0x8ca, v.landProj(a, v.gs16(0x8a0)-v.r16(a+0x16), v.gs16(0x8a2)-v.r16(a+0x18), -0x100))
	v.w16(a+0x94, v.r16(a+0x6c)-v.gs16(0x80e))
}

// pointpix is 0xcdc0, for cpointpix: the land of the running entity's
// scene is run without drawing to find what lies under screen point
// (x, y): the entity and resource hit (0x1aaf2, 0x1aaf0) and the map
// point (0x1aae8..0x1aaec).
func (v *VM) pointpix(x, y int16) {
	a := v.sca(v.curScreenID())
	v.wu16(0x1aaee, v.ru16(a+0x94))
	v.w16(0x1aae4, x)
	v.w16(0x1aae6, y)
	v.wu16(0x1aaf2, 0xffff)
	v.wu16(0x1aaf0, 0xffff)
	v.mem[0x1aae2] = 1
	saved := v.ru16(a)
	if v.mem[a]&0x40 == 0 {
		v.mem[a] |= 0x80
		v.affiland(a)
		if e := v.r16(0x1aaf2); e >= 0 {
			vram := v.ru32(v.atent + uint32(int32(e)))
			ebx := v.ru32(vram - 0x14)
			ebx += v.ru32(ebx + 0xe)
			n := int32(v.ru32(0x1aaf4)-(ebx+v.ru32(ebx))) >> 2
			if int16(n) >= v.r16(ebx+4) {
				m := v.basemain
				m += v.ru32(m + 0xe)
				n = int32(v.ru32(0x1aaf4)-(m+v.ru32(m))) >> 2
			}
			v.wu16(0x1aaf0, uint16(n))
		}
	}
	v.landBase(a)
	px, py, pz := v.landUnproj(v.r16(0x1aae4), v.r16(0x1aae6), v.r16(0x1aaee))
	v.w16(0x1aae8, px)
	v.w16(0x1aaea, py)
	v.w16(0x1aaec, pz)
	v.mem[0x1aae2] = 0
	v.wu16(a, saved)
}

// land135EA is 0x135ea: the window 0x15fea of the scene's buffer copied
// to the logic screen, a dword at a time.
func (v *VM) land135EA() {
	lo, _ := mulw(v.ru16(dsFeny1)-v.ru16(dsWlogy1), v.ru16(dsWloglarg))
	esi := v.ru32(dsWlogic) + uint32(lo+v.ru16(dsFenx1))
	lo, _ = mulw(v.ru16(dsFeny1), v.ru16(dsLoglarg))
	edi := v.ru32(dsLogic) + uint32(lo+v.ru16(dsFenx1))
	cx := v.ru16(dsFenx2) - v.ru16(dsFenx1) + 1
	skip := uint32(int32(int16(v.ru16(dsLoglarg) - cx)))
	n := uint32(cx >> 2)
	for bp := v.ru16(dsFeny2) - v.ru16(dsFeny1) + 1; ; {
		for range n {
			v.wu32(edi, v.ru32(esi))
			edi += 4
			esi += 4
		}
		edi += skip
		esi += skip
		if bp--; bp == 0 {
			break
		}
	}
}
