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
	if uint16(v.r16(fd))&uint16(v.matmask()) == 0 {
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
	if v.tstinv != 0 {
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
	v.tstinv = v.xinv(v.script.vramOrg)
	vram := v.script.vramOrg
	v.readexecOpername()
	v.wcx = v.r16(vram+scrWCX) + int16(v.varD7)
	v.readexecOpername()
	v.wcy = v.r16(vram+scrWCY) + int16(v.varD7)
	v.readexecOpername()
	v.wcz = v.r16(vram+scrWCZ) + int16(v.varD7)
	v.readexecOpername()
	v.setMatmask(int16(v.varD7))
	v.wforme = v.cforme(vram)
	v.clipform()
	crstent(v)
}

func ctstset(v *VM) {
	v.tstinv = v.xinv(v.script.vramOrg)
	v.readexecOpername()
	v.wcx = int16(v.varD7)
	v.readexecOpername()
	v.wcy = int16(v.varD7)
	v.readexecOpername()
	v.wcz = int16(v.varD7)
	v.readexecOpername()
	v.setMatmask(int16(v.varD7))
	v.wforme = v.cforme(v.script.vramOrg)
	v.clipform()
	crstent(v)
}

func cftstmov(v *VM) {
	v.tstinv = v.xinv(v.script.vramOrg)
	vram := v.script.vramOrg
	v.readexecOpername()
	v.wcx = v.r16(vram+scrWCX) + int16(v.varD7)
	v.readexecOpername()
	v.wcy = v.r16(vram+scrWCY) + int16(v.varD7)
	v.readexecOpername()
	v.wcz = v.r16(vram+scrWCZ) + int16(v.varD7)
	v.readexecOpername()
	v.setMatmask(int16(v.varD7))
	v.readexecOpername()
	v.wforme = int16(v.varD7)
	v.clipform()
	crstent(v)
}

func cftstset(v *VM) {
	v.tstinv = v.xinv(v.script.vramOrg)
	v.readexecOpername()
	v.wcx = int16(v.varD7)
	v.readexecOpername()
	v.wcy = int16(v.varD7)
	v.readexecOpername()
	v.wcz = int16(v.varD7)
	v.readexecOpername()
	v.setMatmask(int16(v.varD7))
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

// calcnear is START.EXE 0x82bf: the squared distance to the target and
// whether it lies within the norm (and, for the view searches, the cone).
// The differences are 16-bit, the sums and compares signed 32-bit.
func (v *VM) calcnear(src, tgt uint32) (int32, bool) {
	dx := v.r16(tgt+scrWCX) - v.r16(src+scrWCX)
	dy := v.r16(tgt+scrWCY) - v.r16(src+scrWCY)
	dz := v.r16(tgt+scrWCZ) - v.r16(src+scrWCZ)
	dist := int32(dx)*int32(dx) + int32(dy)*int32(dy) + int32(dz)*int32(dz)
	if dist > int32(v.valnorme) {
		return dist, false
	}
	if v.fview == 0 {
		return dist, true
	}
	dot := int32(int8(v.mem[src+scrWCX2]))*int32(dx) + int32(int8(v.mem[src+scrWCY2]))*int32(dy) +
		int32(int8(v.mem[src+scrWCZ2]))*int32(dz)
	return dist, dist <= int32(uint32(dot)*uint32(uint16(v.valchamp)))
}

func (v *VM) buf32(i int) int32 { return int32(binary.LittleEndian.Uint32(v.buffer[i*4:])) }
func (v *VM) setBuf32(i int, x int32) {
	binary.LittleEndian.PutUint32(v.buffer[i*4:], uint32(x))
}

// trinorme sorts the n found entities by distance (START.EXE 0x7ceb: a
// bubble sort from the end, swapping only on a smaller distance).
// alis' version never terminates once it swaps.
func (v *VM) trinorme(n int) {
	for swapped := true; swapped; {
		swapped = false
		for i := n - 1; i > 0; i-- {
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

// sviewSearch is the loop shared by cnear*/cview* (START.EXE 0x7dc4): every
// entity that passes match and calcnear is listed, or only the nearest one
// when fallent is clear. The distance list sits in the buffer.
func (v *VM) sviewSearch(match func(src, tgt uint32) bool) {
	idx := 0
	if v.fallent == 0 {
		v.setBuf32(0, 0x7fffffff)
		v.tablent[0] = -1
		idx = 1
	}
	src := v.script.vramOrg
	for ent := int16(0); ; {
		tgt := uint32(v.r32(v.atent + uint32(int32(ent))))
		if match(src, tgt) {
			if dist, ok := v.calcnear(src, tgt); ok {
				if v.fallent == 0 {
					if dist < v.buf32(idx-1) {
						idx--
						v.setBuf32(idx, dist)
						v.tablent[idx] = ent
						idx++
					}
				} else if !tabentFull(idx) {
					v.setBuf32(idx, dist)
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
	v.tablent[idx] = -1
	if v.fallent != 0 {
		v.trinorme(idx)
	}
	v.fallent = 0
	crstent(v)
}

// matcher is START.EXE 0x7ac6: whether the target's current form carries a
// material of matmask; mono forms leave their material in goodmat. The
// self test compares only the low 16 bits of the addresses.
func (v *VM) matcher(src, tgt uint32) bool {
	f := v.cforme(tgt)
	if f < 0 || uint16(src) == uint16(tgt) {
		return false
	}
	org := uint32(v.r32(tgt - 0x14))
	base := org + uint32(v.r32(org+0xe))
	base += uint32(v.r32(base + 6))
	form := base + uint32(int32(v.r16(base+uint32(int32(f))*2)))
	if int8(v.mem[form]) < 0 {
		return v.multimat(base, form+2)
	}
	return v.monomat(form + 2)
}

// multimat (0x7b2e) tries the count+1 sub-forms whose index is not negative;
// the count is the header's high byte.
func (v *VM) multimat(base, at uint32) bool {
	n := v.mem[at-1]
	if n == 0 {
		return false
	}
	for {
		n--
		sub := v.r16(at)
		at += 2
		if sub >= 0 && v.submat(base, sub) {
			return true
		}
		if n == 0xff {
			return false
		}
	}
}

// submat (0x7b5b) tests the whole header word's sign, unlike matcher.
func (v *VM) submat(base uint32, sub int16) bool {
	at := base + uint32(int32(v.r16(base+uint32(int32(sub))*2)))
	if v.r16(at) >= 0 {
		return v.multimat(base, at+2)
	}
	return v.monomat(at + 2)
}

// monomat is 0x7b74.
func (v *VM) monomat(at uint32) bool {
	v.goodmat = v.ru16(at)
	return v.goodmat&uint16(v.matmask()) != 0
}

// setMatmaskByte: the searches store only the low byte of the mask.
func (v *VM) setMatmaskByte() {
	v.setMatmask(int16(uint16(v.matmask())&0xff00 | uint16(uint8(v.varD7))))
}

// readNorme squares the 16-bit operand unsigned (mul dx).
func (v *VM) readNorme() {
	v.readexecOpername()
	d := uint32(uint16(v.varD7))
	v.valnorme = d * d
}

func (v *VM) readChamp() {
	v.readexecOpername()
	v.valchamp = int16(v.varD7)
}

func (v *VM) nearent() {
	v.readexecOpername()
	if v.fallent != 0 {
		v.varD7 = -1
	}
	id := int16(v.varD7)
	v.sviewSearch(func(src, tgt uint32) bool {
		return (id < 0 || id == v.r16(tgt-0x0e)) && v.r16(tgt+scrGroup) == v.r16(src+scrGroup) && src != tgt
	})
}

func (v *VM) neartyp() {
	id := v.read16()
	v.varD7 = int32(id)
	v.sviewSearch(func(src, tgt uint32) bool {
		return id == v.r16(tgt-0x10) && v.r16(tgt+scrGroup) == v.r16(src+scrGroup) && src != tgt
	})
}

func (v *VM) nearmat() {
	v.sviewSearch(func(src, tgt uint32) bool {
		return v.r16(tgt+scrGroup) == v.r16(src+scrGroup) && v.matcher(src, tgt)
	})
}

// nearcla: in START.EXE (0x7fb5) the nearest-only path swaps bx/dx as 16-bit
// registers and then corrupts its pointers; the port does what the other
// searches do.
func (v *VM) nearcla() {
	w := v.read16()
	v.varD7 = int32(w)
	cla := int8(w)
	v.sviewSearch(func(src, tgt uint32) bool {
		return cla == int8(v.mem[tgt+0x4c]) && v.r16(tgt+scrGroup) == v.r16(src+scrGroup) && src != tgt
	})
}

func cnearent(v *VM) {
	v.fview = 0
	v.readNorme()
	v.nearent()
}

func cneartyp(v *VM) {
	v.fview = 0
	v.readNorme()
	v.neartyp()
}

func cnearmat(v *VM) {
	v.fview = 0
	v.readNorme()
	v.readexecOpername()
	v.setMatmaskByte()
	v.nearmat()
}

func cnearcla(v *VM) {
	v.fview = 0
	v.readNorme()
	v.nearcla()
}

func cviewent(v *VM) {
	v.fview = 1
	v.readNorme()
	v.readChamp()
	v.nearent()
}

func cviewtyp(v *VM) {
	v.fview = 1
	v.readNorme()
	v.readChamp()
	v.neartyp()
}

func cviewmat(v *VM) {
	v.fview = 1
	v.readNorme()
	v.readChamp()
	v.readexecOpername()
	v.setMatmaskByte()
	v.nearmat()
}

func cviewcla(v *VM) {
	v.fview = 1
	v.readNorme()
	v.readChamp()
	v.nearcla()
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
