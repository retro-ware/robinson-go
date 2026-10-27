package alis

// Fonts, palette and screen parameter opcodes (alis opcodes.c).

// cfont is START.EXE 0x3c64: font -16 sets the font colour instead
// (four operands, the last three unused); another negative font reads
// one operand more and clears the font sprites.
func cfont(v *VM) {
	v.wu16(0x15f36, 1)
	v.readexecOpername()
	switch d := uint16(v.varD7); {
	case int16(d) >= 0:
		v.foasc = d
		v.readexecOpername()
		v.fonum = uint16(v.varD7)
	case d == 0xfff0:
		v.readexecOpername()
		v.mem[dsMonoCol] = uint8(v.varD7)
		v.readexecOpername()
		v.readexecOpername()
		v.readexecOpername()
		return
	default:
		v.wu16(0x15f36, d)
		v.foasc = 0xffff
		v.fonum = 0
		v.readexecOpername()
	}
	v.readexecOpername()
	v.folarg = uint8(v.varD7)
	v.readexecOpername()
	v.fohaut = uint8(v.varD7)
	v.readexecOpername()
	v.fomax = uint16(v.varD7)
}

func cpaper(v *VM) { v.readexecOpername() }

func ctoblack(v *VM) {
	v.readexecOpernameSaveD6()
	v.dosCtoblack(int16(v.varD6))
}

// cmovcolor only changes 4 bpp palettes.
func cmovcolor(v *VM) {
	v.readexecOpername()
	v.readexecOpernameSaveD6()
}

func ctopalet(v *VM) {
	v.readexecOpername()
	v.readexecOpernameSaveD6()
	v.mem[dsFlagmain] = 0
	palidx := int16(v.varD7)
	duration := int16(v.varD6)
	if palidx < 0 {
		v.dosCtopalet(palidx, duration, 0)
		return
	}
	addr := uint32(v.adresdes(int32(palidx)))
	v.dosCtopalet(palidx, duration, addr+uint32(v.r32(addr)))
}

// cnumput sets the put offset and element number for text (0x3cef).
func cnumput(v *VM) {
	v.readexecOpername()
	v.w16(dsDepx, int16(v.varD7))
	v.readexecOpername()
	v.w16(dsDepy, int16(v.varD7))
	v.readexecOpername()
	v.w16(dsDepz, int16(v.varD7))
	v.readexecOpername()
	v.mem[dsNumelem] = uint8(v.varD7)
}

func creducing(v *VM) {
	v.readexecOpername()
	v.mem[v.script.vramOrg-0x26] = uint8(v.varD7)
	v.readexecOpername()
	v.mem[v.script.vramOrg-0x27] = uint8(v.varD7)
}
