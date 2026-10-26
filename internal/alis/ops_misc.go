package alis

// Screen maps, classes, instruments, sprite modes and erasure (alis opcodes.c).

func cscmap(v *VM) {
	v.readexecOpername()
	ent := v.ru16(v.script.vramOrg - 0x0e)
	val := v.read16()
	if val == 0 {
		ent = 0
		val = v.read16()
	}
	if id := v.curScreenID(); id != 0 {
		a := v.sca(id)
		v.w16(a+0x42, val)
		v.wu16(a+0x40, ent)
	}
}

func cfindcla(v *VM) {
	v.readexecOpername()
	idx := 0
	ent := int16(0)
	for {
		tgt := uint32(v.r32(v.atent + uint32(int32(ent))))
		if int8(v.varD7) == int8(v.mem[tgt+0x4c]) && v.script.vramOrg != tgt {
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


func (v *VM) instru() {
	v.readexecOpername()
	tabidx := int16(v.varD7)
	v.readexecOpername()
	scridx := int16(v.varD7)
	v.readexecOpername()
	instidx := int16(v.varD7)
	var addr int32
	if scridx < 0 {
		instidx = 0
	} else {
		addr = v.adresmus(int32(scridx))
		if t := v.mem[addr]; t != 1 && t != 2 && t != 5 && t != 6 {
			return
		}
		addr += 0x10
	}
	v.aud.tabinst[tabidx].address = uint32(addr)
	v.aud.tabinst[tabidx].unknown = uint32(int32(instidx))
}

func cinstru(v *VM) {
	v.flagmain = 0
	v.instru()
}

func cminstru(v *VM) {
	v.flagmain = 1
	v.instru()
}

func cordspr(v *VM) {
	v.readexecOpername()
	v.mem[v.script.vramOrg-0x2b] = uint8(v.varD7)
}

func calign(v *VM) {
	v.readexecOpername()
	var bit uint8
	switch b := uint8(v.varD7); {
	case b&0x10 != 0:
		bit = 4
	case b&0x8 != 0:
		bit = 3
	case b&0x4 != 0:
		bit = 2
	case b&0x2 != 0:
		bit = 1
	}
	v.mem[v.script.vramOrg-0x2c] = bit
	v.readexecOpername()
	v.mem[v.script.vramOrg-0x2d] = uint8(v.varD7)
}

// TODO(START.EXE): alis takes the mode byte from basemain+1, not from the screen.
func cbackstar(v *VM) {
	for i := range 6 {
		v.readexecOpername()
		v.starbuff[i] = int16(v.varD7)
	}
	v.starbuff[6] = 0
	id := v.curScreenID()
	if int16(id) < 0 {
		return
	}
	value := v.mem[v.basemain+1] & 0xe7
	if v.varD7 != 0 {
		value |= 0x18
		at := v.ru16(v.sca(id) + 2)
		v.spr(at).setNumelem(int8(value))
	}
	v.mem[v.sca(id)+1] = value
}

func cstarring(v *VM) {
	for i := range 8 {
		v.readexecOpername()
		v.starbuff[i] = int16(v.varD7)
	}
}

// cengine takes no operands on the PC.
func cengine(v *VM) {}

func cquality(v *VM) {
	v.readexecOpername()
	v.vquality = uint16(v.varD7)
}

func chsprite(v *VM) {
	v.readexecOpername()
	v.mem[v.script.vramOrg-0x2f] = uint8(v.varD7)
	for range 3 {
		v.readexecOpername()
	}
}

func cselpalet(v *VM) {
	v.readexecOpername()
	v.dosCselpalet(int16(v.varD7))
}

func clinepalet(v *VM) {
	v.readexecOpername()
	v.readexecOpernameSaveD6()
}

// cautomode evaluates its operand and does nothing else (START.EXE 0xa393).
func cautomode(v *VM) { v.readexecOpernameSaveD7() }

func cautofile(v *VM) {
	v.readexecOpernameSwap()
	copy(v.autoname[:12], v.mem[v.sd7:v.sd7+12])
	v.autoname[12] = 0
}

// ccancel is cerase with the cancel flag (START.EXE 0x5cb7).
func ccancel(v *VM) {
	v.ferase = 1
	cerase(v)
}

func ccancall(v *VM) {
	v.ferase = 1
	var tmp uint16
	cur := v.elemHead()
	for cur != 0 {
		v.killelem(&cur, &tmp)
	}
	v.ferase = 0
}

func ccancen(v *VM) {
	v.ferase = 1
	cerasen(v)
}

func cblast(v *VM) {
	v.ferase = 1
	ckill(v)
}
