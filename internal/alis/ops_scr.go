package alis

// Fonts, palette and screen parameter opcodes (alis opcodes.c).

func cfont(v *VM) {
	v.readexecOpername()
	v.foasc = uint16(v.varD7)
	v.readexecOpername()
	v.fonum = uint16(v.varD7)
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
	v.flagmain = 0
	palidx := int16(v.varD7)
	duration := int16(v.varD6)
	if palidx < 0 {
		v.dosCtopalet(palidx, duration, 0)
		return
	}
	addr := uint32(v.adresdes(int32(palidx)))
	v.dosCtopalet(palidx, duration, addr+uint32(v.r32(addr)))
}

func cnumput(v *VM) {
	v.readexecOpername()
	v.img.depx = int16(v.varD7)
	v.readexecOpername()
	v.img.depy = int16(v.varD7)
	v.readexecOpername()
	v.img.depz = int16(v.varD7)
	v.readexecOpername()
	v.img.numelem = int8(v.varD7)
}

// scrDirty marks the current screen changed.
func (v *VM) scrDirty(id uint16) { v.mem[v.sca(id)] |= 0x80 }

func cscheart(v *VM) {
	v.readexecOpername()
	v.readexecOpernameSaveD6()
	if id := v.curScreenID(); id != 0 {
		v.w16(v.sca(id)+0x0a, int16(v.varD7))
		v.w16(v.sca(id)+0x0c, int16(v.varD6))
		v.scrDirty(id)
	}
}

func cscpos(v *VM) {
	v.readexecOpername()
	v.readexecOpernameSaveD6()
	if id := v.curScreenID(); id != 0 {
		v.w16(v.sca(id)+scNewx, int16(v.varD7))
		v.w16(v.sca(id)+scNewy, int16(v.varD6))
		v.scrDirty(id)
	}
}

func cscsize(v *VM) {
	v.readexecOpername()
	v.readexecOpernameSaveD6()
	if id := v.curScreenID(); id != 0 {
		v.w16(v.sca(id)+scWidth, int16(v.varD7-1))
		v.w16(v.sca(id)+scHeight, int16(v.varD6-1))
		v.scdosprite(id)
		v.scrDirty(id)
	}
}

// scr3 reads three byte operands into screen bytes at off.
func (v *VM) scr3(off uint32) {
	if id := v.curScreenID(); id != 0 {
		for i := range uint32(3) {
			v.readexecOpername()
			v.mem[v.sca(id)+off+i] = uint8(v.varD7)
		}
		v.scrDirty(id)
	}
}

func cschoriz(v *VM)  { v.scr3(0x20) }
func cscvertic(v *VM) { v.scr3(0x23) }

func cscreduce(v *VM) {
	v.readexecOpername()
	v.readexecOpernameSaveD6()
	if id := v.curScreenID(); id != 0 {
		a := v.sca(id)
		v.mem[a+scCreducing] = uint8(v.varD7)
		v.readexecOpername()
		v.mem[a+scRedmax] = uint8(v.varD6 - 1)
		v.mem[a+scRedshift] = uint8(v.varD7)
		v.scrDirty(id)
	}
}

func cscscale(v *VM) {
	v.readexecOpername()
	if id := v.curScreenID(); id != 0 {
		v.mem[v.sca(id)+scCredonOff] = uint8(v.varD7)
		v.scrDirty(id)
	}
}

func creducing(v *VM) {
	v.readexecOpername()
	v.mem[v.script.vramOrg-0x26] = uint8(v.varD7)
	v.readexecOpername()
	v.mem[v.script.vramOrg-0x27] = uint8(v.varD7)
}
