package alis

// Background, page scroll, collision material and shrink.

// cscback (0x51ea) only stores its operands: the original has no
// background cache.
func cscback(v *VM) {
	v.readexecOpername()
	v.mem[0x178b1] = uint8(v.varD7)
	for _, a := range []uint32{0x178bf, 0x178c1, 0x178c3, 0x178c5, 0x178c7} {
		v.readexecOpername()
		v.wu16(a, uint16(v.varD7))
	}
}

// cscrolpage (0x51a0) stores its operands; a page scroll keeps the scene
// of the running entity from going to video memory after redraws.
func cscrolpage(v *VM) {
	v.readexecOpername()
	v.mem[0x178f3] = uint8(v.varD7)
	for _, a := range []uint32{0x178f5, 0x178f7, 0x178f9} {
		v.readexecOpername()
		v.wu16(a, uint16(v.varD7))
	}
	if v.mem[0x178f3] != 0 {
		v.mem[v.sca(v.curScreenID())] |= 0x20
	}
}

func cmatent(v *VM) {
	v.varD7 = int32(v.matent[v.ptrent-1])
	v.cstoreContinue()
}

func cshrink(v *VM) {
	v.readexecOpername()
	v.readexecOpernameSaveD6()
	v.mem[dsFlagmain] = 0
	addr := uint32(v.adresdes(v.varD7))
	addr += uint32(v.r32(addr))
	var width int16
	switch v.mem[addr] {
	case 0, 2, 0x12, 0x10:
		width = (v.r16(addr+2) + 1) >> 1
	case 0x1a, 0x18, 0x16, 0x14, 0x1e, 0x1c:
		width = v.r16(addr+2) + 1
	default:
		return
	}
	height := v.r16(addr + 4)
	if height == 0 {
		return
	}
	height++
	bits := int32(width) * int32(height)
	org := uint32(v.r32(v.script.vramOrg - 0x14))
	l := uint32(v.r32(org + 0xe))
	e := v.r16(org + l + 4)
	a := uint32(v.r32(org+l)) + l
	for i := int32(0); i < int32(e); i, a = i+1, a+4 {
		t := v.r32(org + a)
		if addr < a+org+uint32(t) {
			v.w32(org+a, t-bits)
		}
	}
	v.w32(org+l+6, v.r32(org+l+6)-bits)
	v.w32(org+l+0xc, v.r32(org+l+0xc)-bits)
	v.w16(addr+2, 0xf)
	v.w16(addr+4, 0)
	v.shrinkprog(int32(addr+6), bits, 0)
	v.script.data.sz -= uint32(bits)
}
