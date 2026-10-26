package alis

import "encoding/binary"

// Collision forms, entity searches and messages (alis opcodes.c).

// form headers on the PC: byte 0 is the kind (negative: list of forms,
// 0: byte box, 1: word box), byte 1 the list length; offsets are signed.
func (v *VM) formptr(base, idx2 int32) int32 { return base + int32(v.r16(uint32(base+idx2))) }

func (v *VM) formkind(hdr int32) int { return int(v.r8(uint32(hdr))) }

func (v *VM) multifirm(entVram, entFormedata, formedata, a4 int32) bool {
	length := int(v.r8(uint32(entFormedata - 1)))
	for i := 0; i < length; i, entFormedata = i+1, entFormedata+2 {
		forme := int32(v.r16(uint32(entFormedata)))
		if forme > -1 {
			if !v.traitfirm(entVram, formedata, a4, forme+forme) {
				return false
			}
		}
	}
	return true
}

func (v *VM) traitfirm(entVram, formedata, entBaseform, forme int32) bool {
	entFormedata := v.formptr(entBaseform, forme)
	if int8(v.r8(uint32(entFormedata))) < 0 {
		return v.multifirm(entVram, entFormedata+2, formedata, entBaseform)
	}
	return v.monofirm(entVram, entFormedata+2)
}

// monofirm tests the other entity's box against px1..pz2; false is a hit.
func (v *VM) monofirm(entVram, entFormedata int32) bool {
	fd := uint32(entFormedata)
	if uint16(v.r16(fd))&uint16(v.matmask) == 0 {
		return true
	}
	var x1, y1, z1, x2, y2, z2 int16
	switch v.formkind(entFormedata - 2) {
	case 0:
		x1, y1, z1 = int16(int8(v.r8(fd+2))), int16(int8(v.r8(fd+3))), int16(int8(v.r8(fd+4)))
		x2, y2, z2 = int16(int8(v.r8(fd+5))), int16(int8(v.r8(fd+6))), int16(int8(v.r8(fd+7)))
	case 1:
		x1, y1, z1 = v.r16(fd+2), v.r16(fd+4), v.r16(fd+6)
		x2, y2, z2 = v.r16(fd+8), v.r16(fd+0xa), v.r16(fd+0xc)
	default:
		return true
	}
	v.goodmat = v.ru16(fd)
	ev := uint32(entVram)
	if v.xinv(ev) != 0 {
		x1, x2 = -x1, -x2
	}
	x1 += v.r16(ev + scrWCX)
	y1 += v.r16(ev + scrWCY)
	z1 += v.r16(ev + scrWCZ)
	x2 += x1
	y2 += y1
	z2 += z1

	tmp := y1
	if y2 <= y1 {
		tmp = y2
		y2 = y1
	}
	if v.py1 <= y2 && tmp <= v.py2 {
		tmp = z1
		if z2 <= z1 {
			tmp = z2
			z2 = z1
		}
		if v.pz1 <= z2 && tmp <= v.pz2 {
			tmp = x1
			if x2 <= x1 {
				tmp = x2
				x2 = x1
			}
			if v.px1 <= x2 && tmp <= v.px2 {
				return false
			}
		}
	}
	return true
}

func (v *VM) traitform(entVram, entFormedata, entBaseform, entWforme2x int32) bool {
	formedata := v.formptr(int32(v.baseform), entWforme2x)
	if int8(v.r8(uint32(formedata))) < 0 {
		return v.multiform(entVram, entFormedata, formedata+2, entBaseform)
	}
	return v.monoform(entVram, entFormedata, formedata+2, entBaseform, entWforme2x)
}

func (v *VM) multiform(entVram, entFormedata, formedata, entBaseform int32) bool {
	length := int(int8(v.r8(uint32(formedata - 1))))
	for i := 0; i < length; i, formedata = i+1, formedata+2 {
		wforme := int32(v.r16(uint32(formedata)))
		if wforme > -1 {
			if !v.traitform(entVram, entFormedata, entBaseform, wforme+wforme) {
				return false
			}
		}
	}
	return true
}

// monoform sets px1..pz2 to the own box and tests it against the other
// entity's forms.
func (v *VM) monoform(entVram, entFormedata, formedata, entBaseform, entWforme2x int32) bool {
	fd := uint32(formedata)
	var fx, fy, fz int16
	switch v.formkind(formedata - 2) {
	case 0:
		fx, fy, fz = int16(int8(v.r8(fd+2))), int16(int8(v.r8(fd+3))), int16(int8(v.r8(fd+4)))
		v.px2, v.py2, v.pz2 = int16(int8(v.r8(fd+5))), int16(int8(v.r8(fd+6))), int16(int8(v.r8(fd+7)))
	case 1:
		fx, fy, fz = v.r16(fd+2), v.r16(fd+4), v.r16(fd+6)
		v.px2, v.py2, v.pz2 = v.r16(fd+8), v.r16(fd+0xa), v.r16(fd+0xc)
	default:
		return true
	}
	if v.xinv(v.script.vramOrg) != 0 {
		fx = -fx
		v.px2 = -v.px2
	}
	fx += v.wcx
	fy += v.wcy
	fz += v.wcz
	v.px2 += fx
	v.py2 += fy
	v.pz2 += fz

	v.py1 = fy
	if v.py2 <= fy {
		v.py1 = v.py2
		v.py2 = fy
	}
	v.pz1 = fz
	if v.pz2 <= fz {
		v.pz1 = v.pz2
		v.pz2 = fz
	}
	v.px1 = fx
	if v.px2 <= fx {
		v.px1 = v.px2
		v.px2 = fx
	}

	test := int8(v.r8(uint32(entFormedata)))
	entFormedata += 2
	if test < 0 {
		return v.multifirm(entVram, entFormedata, formedata, entBaseform)
	}
	return v.monofirm(entVram, entFormedata)
}

func tabentFull(idx int) bool { return idx >= 255 }

// clipform lists the entities whose forms intersect the own form wforme
// at wcx/wcy/wcz.
func (v *VM) clipform() {
	v.ptrent = 0
	if v.wforme > -1 {
		val := uint32(v.r32(v.script.vramOrg - 0x14))
		addr := uint32(v.r32(val+0xe)) + val
		v.baseform = uint32(v.r32(addr+6)) + addr
		formedata := v.formptr(int32(v.baseform), int32(uint32(int32(v.wforme))*2))
		ent := int16(0)
		inRange := uint16(v.wforme) < v.ru16(addr+0xa)

		for {
			entVram := uint32(v.r32(v.atent + uint32(int32(ent))))
			if inRange && v.r16(entVram+scrGroup) == v.r16(v.script.vramOrg+scrGroup) && v.cforme(entVram) > -1 && v.script.vramOrg != entVram {
				entVal := uint32(v.r32(entVram - 0x14))
				entAddr := uint32(v.r32(entVal+0xe)) + entVal
				entBaseform := int32(uint32(v.r32(entAddr+6)) + entAddr)
				if uint16(v.cforme(entVram)) >= v.ru16(entAddr+0xa) {
					ent = v.r16(v.atent + 4 + uint32(int32(ent)))
					if ent == 0 {
						break
					}
					continue
				}
				entWforme2x := int32(v.cforme(entVram)) * 2
				entFormedata := v.formptr(entBaseform, entWforme2x)
				var ok bool
				if int8(v.r8(uint32(formedata))) < 0 {
					ok = v.multiform(int32(entVram), entFormedata, formedata+2, entBaseform)
				} else {
					ok = v.monoform(int32(entVram), entFormedata, formedata+2, entBaseform, entWforme2x)
				}
				if !ok {
					if v.witmov != 0 {
						v.fmitmov = 1
						return
					}
					if tabentFull(v.ptrent) {
						break
					}
					v.matent[v.ptrent] = int16(v.goodmat)
					v.tablent[v.ptrent] = ent
					v.ptrent++
					if v.fallent == 0 {
						break
					}
				}
			}
			ent = v.r16(v.atent + 4 + uint32(int32(ent)))
			if ent == 0 {
				break
			}
		}
	}
	if v.witmov == 0 {
		v.matent[v.ptrent] = 0
		v.tablent[v.ptrent] = -1
		v.ptrent++
	}
}

func csuccent(v *VM) {
	v.varD7 = int32(v.tablent[v.ptrent])
	if v.tablent[v.ptrent] > -1 {
		v.ptrent++
	}
	v.cstoreContinue()
}

func crstent(v *VM) {
	v.ptrent = 0
	csuccent(v)
}

func ctstmov(v *VM) {
	vram := v.script.vramOrg
	v.readexecOpername()
	v.wcx = v.r16(vram+scrWCX) + int16(v.varD7)
	v.readexecOpername()
	v.wcy = v.r16(vram+scrWCY) + int16(v.varD7)
	v.readexecOpername()
	v.wcz = v.r16(vram+scrWCZ) + int16(v.varD7)
	v.readexecOpername()
	v.matmask = int16(v.varD7)
	v.wforme = v.cforme(vram)
	v.clipform()
	crstent(v)
}

// TODO(START.EXE): alis has no v3 ctstset (it consumes no operands).
func ctstset(v *VM) {}

func cftstmov(v *VM) {
	vram := v.script.vramOrg
	v.readexecOpername()
	v.wcx = v.r16(vram+scrWCX) + int16(v.varD7)
	v.readexecOpername()
	v.wcy = v.r16(vram+scrWCY) + int16(v.varD7)
	v.readexecOpername()
	v.wcz = v.r16(vram+scrWCZ) + int16(v.varD7)
	v.readexecOpername()
	v.matmask = int16(v.varD7)
	v.readexecOpername()
	v.wforme = int16(v.varD7)
	v.clipform()
	crstent(v)
}

func cftstset(v *VM) {
	v.readexecOpername()
	v.wcx = int16(v.varD7)
	v.readexecOpername()
	v.wcy = int16(v.varD7)
	v.readexecOpername()
	v.wcz = int16(v.varD7)
	v.readexecOpername()
	v.matmask = int16(v.varD7)
	v.readexecOpername()
	v.wforme = int16(v.varD7)
	v.clipform()
	crstent(v)
}

func cpredent(v *VM) {
	if v.ptrent != 0 {
		v.ptrent--
		v.varD7 = int32(v.tablent[v.ptrent])
	} else {
		v.varD7 = -1
	}
	v.cstoreContinue()
}

// calcnear is false when target is within valnorme (and the view cone).
func (v *VM) calcnear(source, target uint32, val *int32) bool {
	wcx := int32(v.r16(target+scrWCX)) - int32(v.r16(source+scrWCX))
	wcy := int32(v.r16(target+scrWCY)) - int32(v.r16(source+scrWCY))
	wcz := int32(v.r16(target+scrWCZ)) - int32(v.r16(source+scrWCZ))
	*val = wcx*wcx + wcy*wcy + wcz*wcz
	if uint32(*val) <= v.valnorme {
		if v.fview == 0 {
			return false
		}
		dot := int32(int8(v.r8(source+scrWCX2)))*wcx + int32(int8(v.r8(source+scrWCY2)))*wcy + int32(int8(v.r8(source+scrWCZ2)))*wcz
		if *val <= dot*int32(v.valchamp) {
			return false
		}
	}
	return true
}

func (v *VM) buf32(i int) int32 { return int32(binary.LittleEndian.Uint32(v.buffer[i*4:])) }
func (v *VM) setBuf32(i int, x int32) {
	binary.LittleEndian.PutUint32(v.buffer[i*4:], uint32(x))
}

// trinorme sorts the n found entities by distance.
// TODO(START.EXE): alis' version never terminates once it swaps; this is a
// plain stable bubble sort.
func (v *VM) trinorme(n int) {
	for swapped := true; swapped; {
		swapped = false
		for i := 1; i < n; i++ {
			a, b := v.buf32(i-1), v.buf32(i)
			if b < a {
				v.setBuf32(i-1, b)
				v.setBuf32(i, a)
				v.tablent[i-1], v.tablent[i] = v.tablent[i], v.tablent[i-1]
				swapped = true
			}
		}
	}
}

// nearest-first collection shared by the sview* searches
func (v *VM) sviewStart() int {
	if v.fallent == 0 {
		v.setBuf32(0, 0x7fffffff)
		v.tablent[0] = -1
		return 1
	}
	return 0
}

// sviewAdd records a match; false when the table is full.
func (v *VM) sviewAdd(idx *int, ent int16, value int32) {
	if v.fallent == 0 {
		if v.buf32(*idx-1) <= value {
			return
		}
		*idx--
	} else if tabentFull(*idx) {
		return
	}
	v.setBuf32(*idx, value)
	v.tablent[*idx] = ent
	*idx++
}

func (v *VM) sviewEnd(idx int) {
	v.tablent[idx] = -1
	if v.fallent != 0 {
		v.trinorme(idx)
	}
	v.fallent = 0
	crstent(v)
}

func (v *VM) sviewtyp() {
	id := int16(v.varD7)
	if id < 0 {
		id &= 0xff
	}
	idx := v.sviewStart()
	ent := int16(0)
	src := v.script.vramOrg
	for {
		tgt := uint32(v.r32(v.atent + uint32(int32(ent))))
		if int32(id) == int32(v.ru16(tgt-0x10)) && v.r16(tgt+scrGroup) == v.r16(src+scrGroup) && src != tgt {
			var value int32
			if !v.calcnear(src, tgt, &value) {
				if v.fallent != 0 || v.buf32(idx-1) > value {
					if v.fallent == 0 {
						idx--
					}
					if tabentFull(idx) {
						break
					}
					v.setBuf32(idx, value)
					v.tablent[idx] = ent
					idx++
				}
			}
		}
		ent = v.r16(v.atent + 4 + uint32(int32(ent)))
		if ent == 0 {
			break
		}
	}
	v.sviewEnd(idx)
}

// TODO(START.EXE): multimat/traitmat/clipmat read the form headers like the
// big-endian builds (see NOTES.md).
func (v *VM) multimat(formeaddr, baseform int32, wforme int16) int16 {
	length := int16(v.r8(uint32(formeaddr - 2)))
	if length == 0 {
		return wforme
	}
	for i := int32(0); i < int32(length); i++ {
		wforme = v.r16(uint32(formeaddr + i*2))
		if wforme > -1 {
			wforme = int16(v.traitmat(baseform, int32(wforme)))
			if wforme != 0 {
				return wforme
			}
		}
	}
	return wforme
}

func (v *VM) traitmat(baseform, wforme int32) uint32 {
	wforme *= 2
	formeaddr := int16(v.r32(uint32(baseform+wforme)) + baseform)
	fa := int32(formeaddr)
	if v.r16(uint32(fa)) < 0 {
		return uint32(int32(v.multimat(fa+2, baseform, int16(wforme))))
	}
	return uint32(int32(v.monomat(fa + 2)))
}

func (v *VM) monomat(formeaddr int32) int16 {
	v.goodmat = v.ru16(uint32(formeaddr))
	return v.matmask & int16(v.goodmat)
}

func (v *VM) clipmat(src, tgt uint32) int16 {
	wforme := v.cforme(tgt)
	if wforme > -1 && src != tgt {
		org := uint32(v.r32(tgt - 0x14))
		addr := v.r32(org+0xe) + int32(org)
		baseform := v.r32(uint32(addr+6)) + addr
		formeaddr := int32(v.r16(uint32(baseform+int32(wforme)*2))) + baseform
		if v.r16(uint32(formeaddr)) < 0 {
			return v.multimat(formeaddr+2, baseform, wforme)
		}
		return v.monomat(formeaddr + 2)
	}
	return wforme
}

func (v *VM) sviewmat() {
	idx := v.sviewStart()
	ent := int16(0)
	src := v.script.vramOrg
	for {
		tgt := uint32(v.r32(v.atent + uint32(int32(ent))))
		if v.r16(src+scrGroup) == v.r16(tgt+scrGroup) && v.clipmat(src, tgt) != 0 {
			var value int32
			if !v.calcnear(src, tgt, &value) {
				v.sviewAdd(&idx, ent, value)
			}
		}
		ent = v.r16(v.atent + 4 + uint32(int32(ent)))
		if ent == 0 {
			break
		}
	}
	v.sviewEnd(idx)
}

func (v *VM) sviewent() {
	if v.fallent != 0 {
		v.varD7 = -1
	}
	idx := v.sviewStart()
	ent := int16(0)
	src := v.script.vramOrg
	for {
		tgt := uint32(v.r32(v.atent + uint32(int32(ent))))
		if (v.varD7 < 0 || v.varD7 == int32(v.ru16(tgt-0x0e))) && v.r16(tgt+0x30) == v.r16(src+0x30) && src != tgt {
			var value int32
			if !v.calcnear(src, tgt, &value) {
				v.sviewAdd(&idx, ent, value)
			}
		}
		ent = v.r16(v.atent + 4 + uint32(int32(ent)))
		if ent == 0 {
			break
		}
	}
	v.sviewEnd(idx)
}

func (v *VM) readNorme() {
	v.readexecOpername()
	v.valnorme = uint32(v.varD7) * uint32(v.varD7)
}

func cnearent(v *VM) {
	v.fview = 0
	v.readNorme()
	v.readexecOpername()
	v.sviewent()
}

func cneartyp(v *VM) {
	v.fview = 0
	v.readNorme()
	v.varD7 = int32(v.read16())
	v.sviewtyp()
}

func cnearmat(v *VM) {
	v.fview = 0
	v.readNorme()
	v.readexecOpername()
	v.matmask = int16(v.varD7)
	v.sviewmat()
}

func cviewent(v *VM) {
	v.fview = 1
	v.readNorme()
	v.readexecOpername()
	v.valchamp = int16(v.varD7)
	v.readexecOpername()
	v.sviewent()
}

func cviewtyp(v *VM) {
	v.fview = 1
	v.readNorme()
	v.readexecOpername()
	v.valchamp = int16(v.varD7)
	v.varD7 = int32(v.read16())
	v.sviewtyp()
}

func cviewmat(v *VM) {
	v.fview = 1
	v.readNorme()
	v.readexecOpername()
	v.valchamp = int16(v.varD7)
	v.readexecOpername()
	v.matmask = int16(v.varD7)
	v.sviewmat()
}

func corient(v *VM) {
	v.readexecOpername()
	v.varD7 *= 3
	vram := v.script.vramOrg
	var addr int32
	if sv := v.ru16(vram - 0x20); sv == 0 {
		addr = int32(v.vstandard) + 1
	} else {
		addr = int32(sv) + v.r32(vram-0x14) + 1
	}
	v.w8(vram+scrWCX2, v.r8(uint32(addr+0+v.varD7)))
	v.w8(vram+scrWCY2, v.r8(uint32(addr+1+v.varD7)))
	v.w8(vram+scrWCZ2, v.r8(uint32(addr+2+v.varD7)))
}

func csend(v *VM) {
	length := int16(v.read8())
	if v.csendTo(&length) {
		return
	}
	for {
		l := length
		length--
		if l == -1 {
			break
		}
		v.readexecOpername()
	}
}

// csendTo queues the messages; true when all operands were consumed.
func (v *VM) csendTo(length *int16) bool {
	v.readexecOpername()
	entry := int16(v.varD7)
	if entry == -1 {
		return false
	}
	vram := uint32(v.r32(uint32(int32(v.atent) + int32(entry))))
	if vram == 0 || v.r8(vram-0x24)&1 != 0 {
		return false
	}
	const limit = -0x3f
	offset := v.r16(uint32(v.r32(vram-0x14)) + 0x16)
	scanclr1 := v.r16(vram - 0x1c)
	var scanclr2 int16
	for {
		v.readexecOpername()
		scanclr2 = scanclr1 + 2
		if limit < scanclr2 {
			scanclr2 -= offset
		}
		if scanclr2 == v.r16(vram-0x1e) {
			*length--
			break
		}
		v.w16(uint32(int32(vram)+int32(scanclr1)), int16(v.varD7))
		scanclr1 = scanclr2
		*length--
		if *length == -1 {
			v.w16(vram-0x1c, scanclr2)
			v.mem[vram-0x24] |= 0x80
			return true
		}
	}
	v.w16(vram-0x1c, scanclr2)
	return false
}

func cscanclr(v *VM) {
	vram := v.script.vramOrg
	v.w16(vram-0x1e, v.r16(vram-0x1c))
	v.mem[vram-0x24] &= 0x7f
}

func cscanon(v *VM) { v.mem[v.script.vramOrg-0x24] &= 0xfe }

func cscanoff(v *VM) {
	v.mem[v.script.vramOrg-0x24] |= 1
	cscanclr(v)
}

func cinteron(v *VM)   { v.mem[v.script.vramOrg-0x24] &= 0xfd }
func cinteroff(v *VM)  { v.mem[v.script.vramOrg-0x24] |= 2 }
func callentity(v *VM) { v.fallent = 1 }
