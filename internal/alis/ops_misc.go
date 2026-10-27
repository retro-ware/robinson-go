package alis

// Screen maps, classes, instruments, sprite modes and erasure (alis opcodes.c).

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

// dsStars is the star table: 8 words (cbackstar, cstarring).
const dsStars = 0x19676

// cbackstar (START.EXE 0x8f05) sets the first six words, clears the
// seventh and marks the running entity's scene (bits 3-4 of byte 1).
func cbackstar(v *VM) {
	for i := range uint32(6) {
		v.w16(dsStars+2*i, v.getval())
	}
	v.wu16(dsStars+12, 0)
	if id := v.curScreenID(); int16(id) >= 0 {
		a := v.sca(id) + 1
		v.mem[a] &= 0xe7
		if v.r16(dsStars) != 0 {
			v.mem[a] |= 0x18
		}
	}
}

// cstarring (START.EXE 0x8f7d) sets all eight.
func cstarring(v *VM) {
	for i := range uint32(8) {
		v.w16(dsStars+2*i, v.getval())
	}
}

// cengine takes no operands on the PC.
func cengine(v *VM) {}

// cquality (0x95bb) only reads its operand.
func cquality(v *VM) { v.getval() }

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

// cblast (0x4197) kills an entity and frees its elements at once.
func cblast(v *VM) {
	v.mem[dsFerase] = 1
	ckill(v)
}

func cxinv(v *VM)    { v.mem[v.script.vramOrg-3] ^= 1 }
func cxinvon(v *VM)  { v.mem[v.script.vramOrg-3] = 1 }
func cxinvoff(v *VM) { v.mem[v.script.vramOrg-3] = 0 }

func clistent(v *VM) {
	ent := int16(0)
	idx := 0
	for {
		ent = v.r16(v.atent + 4 + uint32(int32(ent)))
		if ent == 0 || tabentFull(idx) {
			break
		}
		v.matent[idx] = 0
		v.tablent[idx] = ent
		idx++
	}
	v.tablent[idx] = -1
	v.fallent = 0
	crstent(v)
}

func credon(v *VM)  { v.mem[v.script.vramOrg-0x25] = 0 }
func credoff(v *VM) { v.mem[v.script.vramOrg-0x25] = 0x80 }
