package alis

// Tile maps (alis opcodes.c, Robinson's Requiem paths). A map header lives
// in the 0x400 bytes below the map data.

func cdefmap(v *VM) {
	m := v.readVaddr()
	v.readexecOpername()
	v.mem[m-0x400] = uint8(v.varD7)
	v.readexecOpername()
	v.w16(m-0x3fe, int16(v.varD7))
	v.wu16(m-0x3fc, v.ru16(v.script.vramOrg-0x0e))
	v.readexecOpername()
	v.w16(m-0x3fa, int16(v.varD7))
	v.w16(m-0x3f8, int16(v.varD7))
	v.readexecOpername()
	v.w16(m-0x3f8, int16(v.varD7-1)+v.r16(m-0x3f8))
	for _, o := range []uint32{0x3f4, 0x3f2, 0x3f0} {
		v.readexecOpername()
		if v.varD7 == 0 {
			v.varD7 = 1
		}
		v.w16(m-o, int16(v.varD7))
	}
	v.mem[m-0x3ff] = 0
	v.w32(m-0x3dc, 4)
	v.mem[m-0x3d8] = 0x7f
	var vala, valb uint32
	if int8(v.mem[m-1]) < 0 {
		valb = v.ru32(m - 6)
		vala = v.ru32(m - 10)
	} else {
		valb = uint32(v.ru16(m - 4))
		vala = uint32(v.ru16(m - 6))
	}
	ratio := uint16(vala / uint32(uint16(valb)))
	v.wu16(m-0x3c6, ratio)
	v.wu16(m-0x3cc, ratio)
	v.wu16(m-0x3c4, uint16(valb))
	v.wu16(m-0x3ca, uint16(valb)>>1)
	v.w16(m-0x3b6, 0)
	for i := uint32(0); i < 3; i++ {
		result := int32(-1)
		count := v.ru16(m - (0x3f4 - i*2))
		for {
			result++
			count >>= 1
			if count == 0 {
				break
			}
		}
		v.w16(m-(0x3c0-i*2), int16(result))
	}
	if v.mem[m-0x3d8] == '\n' {
		if id := v.curScreenID(); id != 0 {
			v.wu16(v.sca(id)+0x40, v.ru16(v.script.vramOrg-0x0e))
			v.wu16(v.sca(id)+0x42, uint16(m-v.script.vramOrg))
		}
	}
}

func csetmap(v *VM) {
	m := v.readVaddr()
	v.readexecOpername()
	v.w16(m-0x3ee, int16(v.varD7))
	v.readexecOpername()
	v.w16(m-0x3ec, int16(v.varD7))
	v.readexecOpername()
	v.w16(m-0x3ea, int16(v.varD7))
	v.readexecOpername()
	v.w16(m-0x3e8, int16(v.varD7))
	v.wu16(m-0x3d6, uint16(v.varD7)-1)
	v.wu16(m-0x3ee, uint16(v.varD7-1)>>1+v.ru16(m-0x3ee))
	v.readexecOpername()
	v.w16(m-0x3e6, int16(v.varD7))
	v.readexecOpername()
	v.w16(m-0x3e4, int16(v.varD7))
	v.wu16(m-0x3d4, uint16(v.varD7)-1)
	v.wu16(m-0x3ea, uint16(v.varD7-1)>>1+v.ru16(m-0x3ea))
}

func cputmap(v *VM) {
	m := v.readVaddr()
	v.readexecOpername()
	v.w16(m-0x3e2, int16(v.varD7))
	v.readexecOpername()
	v.w16(m-0x3e0, int16(v.varD7))
	v.readexecOpername()
	v.w16(m-0x3de, int16(v.varD7))
	v.readexecOpername()
	v.img.numelem = int8(v.varD7)
	pos := uint32(v.ru16(m - 0x3de))
	width := v.ru16(m - 0x3f0)
	v.wu16(m-0x3d2, uint16(pos%uint32(width)))
	v.fmuldes = 0
	var newidx, oldidx uint16
	if !v.searchelem(&newidx, &oldidx) {
		v.createlem(&newidx, &oldidx)
	}
	if sp := v.spr(newidx); sp.state() > -1 {
		sp.setState(2)
	}
	v.putmapin(newidx, m-0x3dc)
	if v.searchelem(&newidx, &oldidx) {
		sp := v.spr(newidx)
		sp.setCredonOff(-1)
		sp.setFlaginvx(0)
		sp.setChsprite(0)
		sp.setDepx(v.r16(m - 0x3ee))
		sp.setDepy(v.r16(m - 0x3ec))
		sp.setDepz(v.r16(m - 0x3ea))
	}
}

func csavepal(v *VM) {
	v.readexecOpername()
	v.dosCsavepal(int16(v.varD7))
}

// csczoom takes no operands on the PC.
func csczoom(v *VM) {}
