package alis

// Store and add destinations (alis storenames.c / addnames.c, v3 paths).

func slocb(v *VM) { v.w8(v.vramAt(int32(v.read16())), uint8(v.varD7)) }
func slocw(v *VM) { v.w16(v.vramAt(int32(v.read16())), int16(v.varD7)) }
func slocp(v *VM) { v.strcpy(v.vramAt(int32(v.read16())), v.oldsd7) }

func sloctp(v *VM) {
	addr := v.tabstring(v.vramAt(int32(v.read16())))
	v.strcpy(uint32(addr), v.oldsd7)
}

func sloctc(v *VM) {
	addr := v.tabchar(v.vramAt(int32(v.read16())))
	v.varD7 = v.pop()
	v.w8(uint32(addr), uint8(v.varD7))
}

func slocti(v *VM) {
	addr := v.tabint(v.vramAt(int32(v.read16())))
	v.varD7 = v.pop()
	v.w16(uint32(addr), int16(v.varD7))
}

func sdirb(v *VM) { v.w8(v.script.vramOrg+uint32(v.read8()), uint8(v.varD7)) }
func sdirw(v *VM) { v.w16(v.script.vramOrg+uint32(v.read8()), int16(v.varD7)) }
func sdirp(v *VM) { v.strcpy(v.script.vramOrg+uint32(v.read8()), v.oldsd7) }

func sdirtp(v *VM) {
	addr := v.tabstring(v.script.vramOrg + uint32(v.read8()))
	v.strcpy(uint32(addr), v.oldsd7)
}

func sdirtc(v *VM) {
	addr := v.tabchar(v.script.vramOrg + uint32(v.read8()))
	v.varD7 = v.pop()
	v.w8(uint32(addr), uint8(v.varD7))
}

func sdirti(v *VM) {
	addr := v.tabint(v.script.vramOrg + uint32(v.read8()))
	v.varD7 = v.pop()
	v.w16(uint32(addr), int16(v.varD7))
}

func smainb(v *VM) { v.w8(v.mainAt(v.read16()), uint8(v.varD7)) }
func smainw(v *VM) { v.w16(v.mainAt(v.read16()), int16(v.varD7)) }
func smainp(v *VM) { v.strcpy(v.mainAt(v.read16()), v.oldsd7) }

func smaintp(v *VM) {
	addr := v.tabstring(v.mainAt(v.read16()))
	v.strcpy(uint32(addr), v.oldsd7)
}

func smaintc(v *VM) {
	addr := v.tabchar(v.mainAt(v.read16()))
	v.varD7 = v.pop()
	v.w8(uint32(addr), uint8(v.varD7))
}

func smainti(v *VM) {
	addr := v.tabint(v.mainAt(v.read16()))
	v.varD7 = v.pop()
	v.w16(uint32(addr), int16(v.varD7))
}

func shimb(v *VM) {
	ent := v.r16(v.vramAt(int32(v.read16())))
	vram := v.r32(uint32(int32(v.atent) + int32(ent)))
	off := v.read16()
	v.w8(uint32(vram+int32(off)), uint8(v.varD7))
}

func shimw(v *VM) {
	ent := v.r16(v.vramAt(int32(v.read16())))
	vram := v.r32(uint32(int32(v.atent) + int32(ent)))
	off := v.read16()
	v.w16(uint32(vram+int32(off)), int16(v.varD7))
}

func spile(v *VM) { v.push(v.varD7) }

func seval(v *VM) {
	v.push(v.varD7)
	oeval(v)
	v.readexecStorename()
}

// cstoreContinue stores D7 / the string result to the destination operand.
func (v *VM) cstoreContinue() {
	v.sd7, v.oldsd7 = v.oldsd7, v.sd7
	v.readexecStorename()
}

// add destinations; the x86 add sets the zero flag cloop tests

func (v *VM) addZero8(addr uint32, x uint8) {
	v.mem[addr] += x
	v.srZero = v.mem[addr] == 0
}

func (v *VM) addZero16(addr uint32, x int16) {
	r := v.r16(addr) + x
	v.w16(addr, r)
	v.srZero = r == 0
}

func alocb(v *VM) { v.addZero8(v.vramAt(int32(v.read16())), uint8(v.varD7)) }
func alocw(v *VM) { v.addZero16(v.vramAt(int32(v.read16())), int16(v.varD7)) }
func alocp(v *VM) { v.strcat(v.vramAt(int32(v.read16())), v.oldsd7) }

func aloctp(v *VM) {
	addr := v.tabstring(v.vramAt(int32(v.read16())))
	v.strcat(uint32(addr), v.oldsd7)
}

// TODO: alis leaves the zero flag alone for these adds; check START.EXE.
func aloctc(v *VM) {
	addr := v.tabchar(v.vramAt(int32(v.read16())))
	v.varD7 = v.pop()
	v.add8(uint32(addr), int8(v.varD7))
}

func alocti(v *VM) {
	addr := v.tabint(v.vramAt(int32(v.read16())))
	v.varD7 = v.pop()
	v.add16(uint32(addr), int16(v.varD7))
}

func adirb(v *VM) { v.addZero8(v.script.vramOrg+uint32(v.read8()), uint8(v.varD7)) }
func adirw(v *VM) { v.addZero16(v.script.vramOrg+uint32(v.read8()), int16(v.varD7)) }
func adirp(v *VM) { v.strcat(v.script.vramOrg+uint32(v.read8()), v.oldsd7) }

func adirtp(v *VM) {
	addr := v.tabstring(v.script.vramOrg + uint32(v.read8()))
	v.strcat(uint32(addr), v.oldsd7)
}

func adirtc(v *VM) {
	addr := v.tabchar(v.script.vramOrg + uint32(v.read8()))
	v.varD7 = v.pop()
	v.add8(uint32(addr), int8(v.varD7))
}

func adirti(v *VM) {
	addr := v.tabint(v.script.vramOrg + uint32(v.read8()))
	v.varD7 = v.pop()
	v.add16(uint32(addr), int16(v.varD7))
}

func amainb(v *VM) { v.add8(v.mainAt(v.read16()), int8(v.varD7)) }
func amainw(v *VM) { v.add16(v.mainAt(v.read16()), int16(v.varD7)) }
func amainp(v *VM) { v.strcat(v.mainAt(v.read16()), v.oldsd7) }

func amaintp(v *VM) {
	addr := v.tabstring(v.mainAt(v.read16()))
	v.strcat(uint32(addr), v.oldsd7)
}

func amaintc(v *VM) {
	addr := v.tabchar(v.mainAt(v.read16()))
	v.varD7 = v.pop()
	v.add8(uint32(addr), int8(v.varD7))
}

func amainti(v *VM) {
	addr := v.tabint(v.mainAt(v.read16()))
	v.varD7 = v.pop()
	v.add16(uint32(addr), int16(v.varD7))
}

func ahimb(v *VM) {}

func ahimw(v *VM) {
	ent := v.r16(v.vramAt(int32(v.read16())))
	addr := v.r32(uint32(int32(v.atent)+int32(ent))) + int32(v.read16())
	v.add16(uint32(addr), int16(v.varD7))
}

func ahimp(v *VM) {}

func ahimtp(v *VM) {
	off1 := v.read16()
	off2 := v.read16()
	addr := v.r32(uint32(int32(v.atent) + int32(v.r16(v.vramAt(int32(off2))))))
	a := v.tabstring(uint32(addr + int32(off1)))
	v.strcat(uint32(a), v.oldsd7)
}

func ahimtc(v *VM) {}
func ahimti(v *VM) {}

func aeval(v *VM) {
	v.push(v.varD7)
	oeval(v)
	v.readexecAddname()
}

func init() {
	for i, f := range []opfn{
		pnul, pnul, pnul, slocb, slocw, slocp, sloctp, sloctc, slocti,
		sdirb, sdirw, sdirp, sdirtp, sdirtc, sdirti,
		smainb, smainw, smainp, smaintp, smaintc, smainti,
		shimb, shimw, pnul, pnul, pnul, pnul, spile, seval, ofin,
	} {
		storenames[i*2] = f
	}
	for i, f := range []opfn{
		pnul, pnul, pnul, alocb, alocw, alocp, aloctp, aloctc, alocti,
		adirb, adirw, adirp, adirtp, adirtc, adirti,
		amainb, amainw, amainp, amaintp, amaintc, amainti,
		ahimb, ahimw, ahimp, ahimtp, ahimtc, ahimti, spile, aeval, ofin,
	} {
		addnames[i*2] = f
	}
}
