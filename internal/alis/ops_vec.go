package alis

// Movement vectors, worlds and entity type searches (alis opcodes.c).

func cdefvect(v *VM) {
	offset := int16(v.read8())
	offset += offset * 2
	offset += 3
	v.jump(int32(offset))
}

func csetvect(v *VM) { v.w16(v.script.vramOrg-0x20, v.read16()) }

// debApproach returns the vector table (count byte first) and the offset
// to entity ent.
func (v *VM) debApproach(ent int32) (addr uint32, wcx, wcy, wcz int16) {
	vram := v.script.vramOrg
	o := uint32(v.r32(uint32(int32(v.atent) + ent)))
	wcx = v.r16(o+scrWCX) - v.r16(vram+scrWCX)
	wcy = v.r16(o+scrWCY) - v.r16(vram+scrWCY)
	wcz = v.r16(o+scrWCZ) - v.r16(vram+scrWCZ)
	if sv := v.ru16(vram - 0x20); sv == 0 {
		addr = v.vstandard
	} else {
		addr = uint32(sv) + uint32(v.r32(vram-0x14))
	}
	v.varD7 = int32(v.mem[addr])
	v.w8(vram+scrAddr, v.mem[addr])
	return addr + 1, wcx, wcy, wcz
}

func (v *VM) calscal(a uint32, wcx, wcy, wcz int16) int32 {
	var r int32
	if x := int8(v.mem[a]); x != 0 {
		r += int32(x) * int32(wcx)
	}
	if x := int8(v.mem[a+1]); x != 0 {
		r += int32(x) * int32(wcy)
	}
	if x := int8(v.mem[a+2]); x != 0 {
		r += int32(x) * int32(wcz)
	}
	return r
}

func (v *VM) setApproach(index int8, best uint32) {
	vram := v.script.vramOrg
	v.w8(vram+scrAddr, v.r8(vram+scrAddr)-uint8(index))
	v.w8(vram+scrWCX2, v.mem[best])
	v.w8(vram+scrWCY2, v.mem[best+1])
	v.w8(vram+scrWCZ2, v.mem[best+2])
}

func capproach(v *VM) {
	v.readexecOpername()
	if v.varD7 == -1 {
		return
	}
	a, wcx, wcy, wcz := v.debApproach(v.varD7)
	var best uint32
	limit := int32(-0x80000000)
	index := int8(v.varD6)
	for {
		if test := v.calscal(a, wcx, wcy, wcz); limit < test {
			index = int8(v.varD7)
			limit = test
			best = a
		}
		a += 3
		v.varD7--
		if v.varD7 == -1 {
			break
		}
	}
	v.setApproach(index, best)
}

// TODO(START.EXE): alis truncates the dot product to s16 here only.
func cescape(v *VM) {
	v.readexecOpername()
	if v.varD7 == -1 {
		return
	}
	a, wcx, wcy, wcz := v.debApproach(v.varD7)
	var best uint32
	limit := int32(0x7fffffff)
	index := int8(v.varD6)
	for {
		if test := int32(int16(v.calscal(a, wcx, wcy, wcz))); test < limit {
			index = int8(v.varD7)
			limit = test
			best = a
		}
		a += 3
		v.varD7--
		if v.varD7 == -1 {
			break
		}
	}
	v.setApproach(index, best)
}

func (v *VM) vectorTarget() {
	vram := v.script.vramOrg
	v.wcx = int16(int8(v.r8(vram+scrWCX2))) + v.r16(vram+scrWCX)
	v.wcy = int16(int8(v.r8(vram+scrWCY2))) + v.r16(vram+scrWCY)
	v.wcz = int16(int8(v.r8(vram+scrWCZ2))) + v.r16(vram+scrWCZ)
}

func cvtstmov(v *VM) {
	v.tstinv = v.xinv(v.script.vramOrg)
	v.vectorTarget()
	v.readexecOpername()
	v.matmask = int16(v.varD7)
	v.wforme = v.cforme(v.script.vramOrg)
	v.clipform()
	crstent(v)
}

func cvftstmov(v *VM) {
	v.tstinv = v.xinv(v.script.vramOrg)
	v.vectorTarget()
	v.readexecOpername()
	v.matmask = int16(v.varD7)
	v.readexecOpername()
	v.wforme = int16(v.varD7)
	v.clipform()
	crstent(v)
}

func cvmov(v *VM) {
	vram := v.script.vramOrg
	v.add16(vram+scrWCX, int16(int8(v.r8(vram+scrWCX2))))
	v.add16(vram+scrWCY, int16(int8(v.r8(vram+scrWCY2))))
	v.add16(vram+scrWCZ, int16(int8(v.r8(vram+scrWCZ2))))
}

// TODO(START.EXE): alis writes all six bytes to the same address.
func cdefworld(v *VM) {
	offset := v.read16()
	at := uint32(int32(v.script.vramOrg) + int32(offset))
	for range 6 {
		v.w8(at, v.read8())
	}
}

// alis writes to vram+0xffde/0xffdf; the world bytes are at -0x22/-0x21.
func cworld(v *VM) {
	v.w8(v.script.vramOrg-0x22, v.read8())
	v.w8(v.script.vramOrg-0x21, v.read8())
}

// cfindmat lists the entities whose form carries a material of the mask
// (START.EXE 0x7b89). The original loads goodmat from matent instead of
// storing it there.
func cfindmat(v *VM) {
	v.readexecOpername()
	v.setMatmaskByte()
	src := v.script.vramOrg
	idx := 0
	for ent := int16(0); ; {
		tgt := uint32(v.r32(v.atent + uint32(int32(ent))))
		if v.matcher(src, tgt) && !tabentFull(idx) {
			v.goodmat = uint16(v.matent[idx])
			v.tablent[idx] = ent
			idx++
			if v.fallent == 0 {
				break
			}
		}
		ent = v.r16(v.atent + 4 + uint32(int32(ent)))
		if ent == 0 {
			break
		}
	}
	v.tablent[idx] = -1
	v.fallent = 0
	crstent(v)
}

// cfindtyp compares the raw type word (START.EXE 0x7bef).
func cfindtyp(v *VM) {
	id := v.read16()
	v.varD7 = int32(id)
	ent := int16(0)
	idx := 0
	for {
		tgt := uint32(v.r32(v.atent + uint32(int32(ent))))
		if id == v.r16(tgt-0x10) && v.script.vramOrg != tgt {
			if tabentFull(idx) {
				break
			}
			v.matent[idx] = 0
			v.tablent[idx] = ent
			idx++
			if v.fallent == 0 {
				break
			}
		}
		ent = v.r16(v.atent + uint32(int32(ent)) + 4)
		if ent == 0 {
			break
		}
	}
	v.tablent[idx] = -1
	v.fallent = 0
	crstent(v)
}
