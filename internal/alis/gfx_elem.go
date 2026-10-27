package alis

// Elements: the sprites an entity puts into its scene (START.EXE
// 0x5361..0x5ddd), and the scene list (0x423a, 0x46f4, 0x523e..0x534e).

// putAt is the put opcodes (0x5361..0x5479): invx mirrors the element,
// main takes the resource from the main script, at reads the offset and
// the element number.
func (v *VM) putAt(main bool, invx uint8, at bool) {
	v.mem[dsFlagmain] = 0
	if main {
		v.mem[dsFlagmain] = 1
	}
	v.mem[dsInvx] = invx
	if !at {
		v.readexecOpername()
		v.wu16(dsDepx, 0)
		v.wu16(dsDepy, 0)
		v.wu16(dsDepz, 0)
		v.wu16(dsNumelem, 0)
	} else {
		v.readexecOpernameSaveD7()
		v.w16(dsDepx, int16(v.varD7))
		v.readexecOpernameSaveD7()
		v.w16(dsDepy, int16(v.varD7))
		v.readexecOpernameSaveD7()
		v.w16(dsDepz, int16(v.varD7))
		v.readexecOpernameSaveD7()
		v.readexecOpernameSaveD6()
		v.w16(dsNumelem, int16(v.varD6))
	}
	v.mem[dsPutSub] = 0
	v.putin(uint16(v.varD7))
}

func cput(v *VM)    { v.putAt(false, v.xinv(v.script.vramOrg), false) }
func cputnat(v *VM) { v.putAt(false, v.xinv(v.script.vramOrg), true) }
func cxput(v *VM)   { v.putAt(false, v.xinv(v.script.vramOrg)^1, false) }
func cxputat(v *VM) { v.putAt(false, v.xinv(v.script.vramOrg)^1, true) }
func cmput(v *VM)   { v.putAt(true, v.xinv(v.script.vramOrg), false) }
func cmputat(v *VM) { v.putAt(true, v.xinv(v.script.vramOrg), true) }

// cmxput does not mirror: 0x5460 xors bx but stores al.
func cmxput(v *VM)   { v.putAt(true, v.xinv(v.script.vramOrg), false) }
func cmxputat(v *VM) { v.putAt(true, v.xinv(v.script.vramOrg)^1, true) }

// put puts resource idx at offset 0 as element 0 of the running entity.
func (v *VM) put(idx uint16) {
	v.mem[dsInvx] = v.xinv(v.script.vramOrg)
	v.wu16(dsDepx, 0)
	v.wu16(dsDepy, 0)
	v.wu16(dsDepz, 0)
	v.wu16(dsNumelem, 0)
	v.mem[dsPutSub] = 0
	v.putin(idx)
}

// resVram is the entity whose resources 0x5c01 and 0x5c46 index.
func (v *VM) resVram() uint32 {
	if v.mem[dsFlagmain] != 0 {
		return v.basemain
	}
	return v.script.vramOrg
}

// resentry is the address of the resource table entry of idx (0x5c46);
// resaddr the resource itself (0x5c01). ok is false after error 0xf.
func (v *VM) resentry(idx uint16) (uint32, bool) {
	ebx := v.ru32(v.resVram() - 0x14)
	ebx += v.ru32(ebx + 0xe)
	if idx >= v.ru16(ebx+4) {
		v.alisError(0xf)
		return 0, false
	}
	ebx += v.ru32(ebx)
	return ebx + uint32(int32(int16(idx)))<<2, true
}

func (v *VM) resaddr(idx uint16) (uint32, bool) {
	ebx, ok := v.resentry(idx)
	if !ok {
		return 0, false
	}
	return ebx + v.ru32(ebx), true
}

// putin is 0x5480: a sprite becomes (or updates) an element, a group puts
// its members with their offsets.
func (v *VM) putin(idx uint16) {
	defer func() { v.mem[dsPutAgain] = 0 }()
	ebx, ok := v.resaddr(idx)
	if !ok || ebx > v.ru32(0x15f2a) {
		return
	}
	switch t := v.mem[ebx]; {
	case int8(t) >= 0:
		v.putone(idx)
	case t == 0xfe:
		// the original jumps to the palette code (0x8988), whose ret
		// would not return here
		v.dosPutinPalette(ebx)
		return
	default:
		p := ebx + 2
		for n := v.mem[ebx+1]; n != 0; n-- {
			saved := v.ru16(dsInvx)
			dx, dy, dz := v.r16(dsDepx), v.r16(dsDepy), v.r16(dsDepz)
			v.mem[dsPutSub] = 1
			el := v.ru16(p)
			ax := v.r16(p + 2)
			if v.mem[dsInvx] != 0 {
				ax = -ax
			}
			v.w16(dsDepx, dx+ax)
			v.w16(dsDepy, dy+v.r16(p+4))
			v.w16(dsDepz, dz+v.r16(p+6))
			p += 8
			if int16(el) < 0 {
				el &= 0x7fff
				v.mem[dsInvx] ^= 1
			}
			v.putin(el)
			v.w16(dsDepz, dz)
			v.w16(dsDepy, dy)
			v.w16(dsDepx, dx)
			v.wu16(dsInvx, saved)
		}
	}
	v.putend()
}

// putone is 0x5555: find the element (a group member takes the first one
// not put yet) or create it, then fill it in.
func (v *VM) putone(idx uint16) {
	var di, bx uint16
	var found, ok bool
	if v.mem[dsPutAgain] != 0 || v.mem[dsPutSub] != 0 {
		di, bx, found = v.searchelem()
		for found && v.fs8(di, spState) != 0 {
			di, bx, found = v.nextsame(di)
		}
		if found {
			v.setFs8(di, spState, 2)
		} else if di, ok = v.createlem(di, bx); !ok {
			return
		}
	} else {
		if di, bx, found = v.searchelem(); !found {
			if di, ok = v.createlem(di, bx); !ok {
				return
			}
		}
		if int8(v.fs8(di, spState)) >= 0 {
			v.setFs8(di, spState, 2)
		}
	}
	entry, ok := v.resentry(idx)
	if !ok {
		return
	}
	v.putfill(di, entry)
}

// putfill is 0x55a3: element di shows the image at entry, placed and
// drawn as the running entity says.
func (v *VM) putfill(di uint16, entry uint32) {
	vram := v.script.vramOrg
	v.setFsu32(di, spData, entry)
	v.setFs8(di, spInvx, v.mem[dsInvx])
	v.setFsu16(di, spClinking, v.ru16(vram-0x2a))
	v.setFs8(di, spCordspr, v.mem[vram-0x2b])
	v.setFs8(di, spZoom, 0)
	v.setFsu16(di, spColour, v.ru16(vram+0x28))
	v.setFsu16(di, spEntity, v.ru16(vram-0xe))
	cl := v.mem[vram-0x25]
	v.setFs8(di, spCredon, cl)
	if cl != 0x80 {
		v.setFs8(di, spCreduc, v.mem[vram-0x27])
		v.setFs8(di, spCredon, v.mem[vram-0x26])
	} else {
		v.setFsu16(di, spZoomy, v.ru16(vram-0x36))
		v.setFsu16(di, spZoomx, v.ru16(vram-0x38))
		v.setFs8(di, spCreduc, 0)
		v.setFs8(di, spZoom, 1)
	}
	if v.mem[dsMonoOn] != 0 && v.mem[dsMonoCol] != 0 {
		v.setFsu16(di, spColour, 0x8000|uint16(v.mem[dsMonoCol]))
	}
	v.setFs16(di, spDepx, v.r16(dsOldcx)+v.r16(dsDepx))
	v.setFs16(di, spDepy, v.r16(dsOldcy)+v.r16(dsDepy))
	v.setFs16(di, spDepz, v.r16(dsOldcz)+v.r16(dsDepz))
}

// putend (0x568c): after a put that is not a group member, the elements of
// that number that were not put again are erased.
func (v *VM) putend() {
	if v.mem[dsPutSub] != 0 {
		return
	}
	di, bx, found := v.searchelem()
	for found {
		if v.fs8(di, spState) != 0 {
			di, bx, found = v.nextsame(di)
			continue
		}
		if di = v.killelem(di, bx); di == 0 {
			return
		}
		found = v.samenum(di)
	}
}

// samenum is 0x56d7: di is an element of the running entity's scene with
// the element number being put.
func (v *VM) samenum(di uint16) bool {
	return v.fsu16(di, spScreen) == v.curScreenID() && v.fs8(di, spNumelem) == v.mem[dsNumelem]
}

// nextsame is 0x56c9: the next element, if it has the same number.
func (v *VM) nextsame(di uint16) (next, prev uint16, ok bool) {
	prev = di
	if next = v.fsu16(di, spToNext); next == 0 {
		return 0, prev, false
	}
	return next, prev, v.samenum(next)
}

// searchelem is 0x56f7: the entity's elements are sorted by scene and
// number, both descending. Without a match di is where the element goes
// and bx the one before it.
func (v *VM) searchelem() (di, bx uint16, found bool) {
	di = v.elemHead()
	sc := int16(v.curScreenID())
	n := int8(v.mem[dsNumelem])
	for di != 0 {
		s := v.fs16(di, spScreen)
		if sc < s || sc == s && n < int8(v.fs8(di, spNumelem)) {
			bx, di = di, v.fsu16(di, spToNext)
			continue
		}
		return di, bx, sc == s && n == int8(v.fs8(di, spNumelem))
	}
	return 0, bx, false
}

// searchtete is 0x5737: the first element in the running entity's scene.
func (v *VM) searchtete() (di, bx uint16, found bool) {
	di = v.elemHead()
	sc := int16(v.curScreenID())
	for di != 0 {
		s := v.fs16(di, spScreen)
		if sc < s {
			bx, di = di, v.fsu16(di, spToNext)
			continue
		}
		return di, bx, sc == s
	}
	return 0, bx, false
}

// createlem is 0x5769: a free sprite goes into the entity's list before di
// and at the head of its scene's draw list.
func (v *VM) createlem(di, bx uint16) (uint16, bool) {
	si := v.ru16(dsLibsprit)
	if si == 0 {
		v.alisError(0xa)
		return di, false
	}
	v.wu16(dsLibsprit, v.fsu16(si, spToNext))
	if bx == 0 {
		v.wu16(v.script.vramOrg-0x18, si)
	} else {
		v.setFsu16(bx, spToNext, si)
	}
	v.setFsu16(si, spToNext, di)
	di = si
	sc := v.curScreenID()
	v.setFsu16(di, spScreen, sc)
	v.setFs8(di, spNumelem, v.mem[dsNumelem])
	v.setFs8(di, spState, 0xff)
	head := v.ru16(v.sca(sc) + 2)
	v.setFsu16(di, spLink, v.fsu16(head, spLink))
	v.setFsu16(head, spLink, di)
	return di, true
}

// killelem is 0x5d16: a drawn element is marked to be erased by the next
// draw, a new one (or any while ferase) is freed at once. It returns the
// element after it.
func (v *VM) killelem(di, bx uint16) uint16 {
	next := v.fsu16(di, spToNext)
	if v.mem[dsFerase] == 0 && int8(v.fs8(di, spState)) >= 0 {
		v.setFs8(di, spState, 1)
		v.setElemNext(bx, next)
	} else {
		v.setElemNext(bx, next)
		v.setFsu16(di, spToNext, v.ru16(dsLibsprit))
		v.wu16(dsLibsprit, di)
		v.unlinkScene(di)
	}
	if bx == 0 {
		return v.elemHead()
	}
	return v.fsu16(bx, spToNext)
}

func (v *VM) setElemNext(bx, next uint16) {
	if bx == 0 {
		v.wu16(v.script.vramOrg-0x18, next)
	} else {
		v.setFsu16(bx, spToNext, next)
	}
}

// unlinkScene is 0x5d94: di leaves its scene's draw list.
func (v *VM) unlinkScene(di uint16) {
	a := v.sca(v.fsu16(di, spScreen)) + 2
	si := v.ru16(a)
	if di == si {
		v.wu16(a, v.fsu16(di, spLink))
		return
	}
	for n := 0; v.fsu16(si, spLink) != di; n++ {
		if n > 0x10000 {
			// the original loops forever
			v.fatal("unlinkScene: element not in its scene")
			return
		}
		si = v.fsu16(si, spLink)
	}
	v.setFsu16(si, spLink, v.fsu16(di, spLink))
}

// cerasen (0x5c96) erases the elements of a number; ccancen (0x5c89)
// frees them at once.
func cerasen(v *VM) {
	v.readexecOpername()
	v.mem[dsNumelem] = uint8(v.varD7)
	for {
		di, bx, found := v.searchelem()
		if !found {
			break
		}
		v.killelem(di, bx)
	}
	v.mem[dsFerase] = 0
}

func ccancen(v *VM) {
	v.mem[dsFerase] = 1
	cerasen(v)
}

// cerase (0x5cc4) erases the elements in the entity's scene; ccancel
// (0x5cb7) frees them.
func cerase(v *VM) {
	if di, bx, found := v.searchtete(); found {
		for {
			if di = v.killelem(di, bx); di == 0 || v.curScreenID() != v.fsu16(di, spScreen) {
				break
			}
		}
	}
	v.mem[dsFerase] = 0
}

func ccancel(v *VM) {
	v.mem[dsFerase] = 1
	cerase(v)
}

// cerasall (0x5cf7) erases all elements of the entity; ccancall (0x5cea)
// frees them.
func cerasall(v *VM) {
	for di := v.elemHead(); di != 0; {
		di = v.killelem(di, 0)
	}
	v.mem[dsFerase] = 0
}

func ccancall(v *VM) {
	v.mem[dsFerase] = 1
	cerasall(v)
}

// savecoord (0x605e) and updtcoord (0x60a1) bracket an entity's run: a
// moved or turned entity moves its elements.
func (v *VM) savecoord(vram uint32) {
	v.wu16(dsOldcx, v.ru16(vram+scrWCX))
	v.wu16(dsOldcy, v.ru16(vram+scrWCY))
	v.wu16(dsOldcz, v.ru16(vram+scrWCZ))
	v.wu16(dsOldacx, v.ru16(vram+scrWCAX))
	v.wu16(dsOldacy, v.ru16(vram+scrWCAY))
	v.wu16(dsOldacz, v.ru16(vram+scrWCAZ))
}

func normAngle(angle int16) int16 {
	if 0x168 < angle {
		angle -= 0x168
		if 0x168 < angle {
			angle %= 0x168
		}
	}
	if angle < -0x168 {
		angle += 0x168
		if angle < -0x168 {
			angle %= 0x168
		}
	}
	return angle
}

func (v *VM) updtcoord(vram uint32) {
	for i, old := range [3]uint32{dsOldacx, dsOldacy, dsOldacz} {
		a := vram + scrWCAX + uint32(i)*8
		if angle := v.r16(a); angle != v.r16(old) {
			v.w16(a, normAngle(angle))
		}
	}
	dx := v.r16(vram+scrWCX) - v.r16(dsOldcx)
	dy := v.r16(vram+scrWCY) - v.r16(dsOldcy)
	dz := v.r16(vram+scrWCZ) - v.r16(dsOldcz)
	az := v.ru16(vram + scrWCAZ)
	if az == v.ru16(dsOldacz) && dz == 0 && dx == 0 && dy == 0 {
		return
	}
	for di := v.ru16(vram - 0x18); di != 0; di = v.fsu16(di, spToNext) {
		if v.fs8(di, spState) == 0 {
			v.setFs8(di, spState, 2)
		}
		v.setFs16(di, spDepx, v.fs16(di, spDepx)+dx)
		v.setFs16(di, spDepy, v.fs16(di, spDepy)+dy)
		v.setFs16(di, spDepz, v.fs16(di, spDepz)+dz)
		v.setFsu16(di, spColour, az)
	}
}

// cdefsc defines a scene (0x423a): flags, 32 bytes of its record, and a
// sprite for its window. The free list is not checked.
func cdefsc(v *VM) {
	id := uint16(v.read16())
	a := v.sca(id)
	v.mem[a] |= 0x40
	v.mem[a+1] = v.read8()
	for i := range uint32(0x20) {
		v.mem[a+6+i] = v.read8()
	}
	di := v.ru16(dsLibsprit)
	v.wu16(dsLibsprit, v.fsu16(di, spToNext))
	v.wu16(a+2, di)
	v.setFsu16(di, spLink, 0)
	v.wu16(a+4, 0)
	v.setFs8(di, spNumelem, v.mem[a+1])
	v.scwindow(a)
	v.wu16(a+0x2a, 0)
	v.wu16(a+0x2c, 0)
	v.wu16(a+0x2e, 0)
	v.scadd(id)
	v.vectoriel(a)
}

// scwindow is 0x46f4: the scene's sprite holds its window, 16 pixel
// aligned.
func (v *VM) scwindow(a uint32) {
	di := v.ru16(a + 2)
	x, y := v.r16(a+0xe), v.r16(a+0x10)
	v.setFs16(di, spNewx, x&^0xf)
	v.setFs16(di, spNewy, y)
	v.setFsu32(di, spNewd, 0x7fff)
	v.setFs16(di, spDepx, (x+v.r16(a+0x12))|0xf)
	v.setFs16(di, spDepy, y+v.r16(a+0x14))
}

func cscreen(v *VM) { v.wu16(v.script.vramOrg-0x16, uint16(v.read16())) }

// cscheart (0x4665), cscsize (0x4690) and cscpos (0x46c4) set the scene of
// the running entity.
func (v *VM) scOperands() (uint32, int16, int16, bool) {
	v.readexecOpername()
	v.readexecOpernameSaveD6()
	id := v.curScreenID()
	if id == 0 {
		return 0, 0, 0, false
	}
	return v.sca(id), int16(v.varD7), int16(v.varD6), true
}

func cscheart(v *VM) {
	if a, x, y, ok := v.scOperands(); ok {
		v.w16(a+0xa, x)
		v.w16(a+0xc, y)
		v.mem[a] |= 0x80
	}
}

func cscsize(v *VM) {
	if a, w, h, ok := v.scOperands(); ok {
		v.w16(a+0x12, w-1)
		v.w16(a+0x14, h-1)
		v.scwindow(a)
		v.mem[a] |= 0x80
	}
}

func cscpos(v *VM) {
	if a, x, y, ok := v.scOperands(); ok {
		v.w16(a+0xe, x)
		v.w16(a+0x10, y)
		v.scwindow(a)
		v.mem[a] |= 0x80
	}
}

// copensc (0x523e) shows a scene and moves it to the end of the list.
func copensc(v *VM) {
	id := uint16(v.read16())
	a := v.sca(id)
	v.mem[a] = v.mem[a]&0xbf | 0x80
	v.scbreak(id)
	v.scadd(id)
	if v.mem[a+1]&2 != 0 {
		v.openland(a)
	}
}

func cclosesc(v *VM) { v.mem[v.sca(uint16(v.read16()))] |= 0x40 }

// scadd (0x5283) appends a scene to the list, scbreak (0x52b9) takes it
// out.
func (v *VM) scadd(id uint16) {
	v.wu16(v.sca(id)+4, 0)
	cur := v.ru16(dsPtscreen)
	if cur == 0 {
		v.wu16(dsPtscreen, id)
		return
	}
	for {
		next := v.ru16(v.sca(cur) + 4)
		if next == 0 {
			break
		}
		cur = next
	}
	v.wu16(v.sca(cur)+4, id)
}

func (v *VM) scbreak(id uint16) {
	cur := v.ru16(dsPtscreen)
	if cur == 0 {
		return
	}
	if id == cur {
		v.wu16(dsPtscreen, v.ru16(v.sca(id)+4))
		return
	}
	for {
		next := v.ru16(v.sca(cur) + 4)
		if next == id {
			v.wu16(v.sca(cur)+4, v.ru16(v.sca(id)+4))
			return
		}
		if cur = next; cur == 0 {
			return
		}
	}
}

// vectoriel (0x52fd) completes the scene's matrix with the cross product
// of its first two rows; the words overlap, so +0x29 gets the high byte
// of the last one.
func (v *VM) vectoriel(a uint32) {
	m := func(i uint32) int16 { return int16(int8(v.mem[a+0x20+i])) }
	v.w16(a+0x26, m(1)*m(5)-m(4)*m(2))
	v.w16(a+0x27, m(2)*m(3)-m(5)*m(0))
	v.w16(a+0x28, m(0)*m(4)-m(3)*m(1))
}
