package alis

// Music and sound opcodes (alis opcodes.c).

func (v *VM) music() {
	if v.mem[dsFlagmain] != 0 {
		v.aud.musicID = v.main.data.header.id
	} else {
		v.aud.musicID = v.script.data.header.id
	}
	a := &v.aud
	v.readexecOpername()
	addr := uint32(v.adresmus(v.varD7))
	switch v.mem[addr] {
	case 0, 3:
		a.muvolume = 0
		a.mupnote = addr + 6
		v.readexecOpername()
		a.maxvolume = uint16(v.varD7 << 8)
		v.readexecOpername()
		a.mutempo = uint8(v.varD7)
		v.readexecOpername()
		a.muattac = uint16(v.varD7 + 1)
		v.readexecOpername()
		a.muduree = int16(v.varD7 + 1)
		v.readexecOpername()
		a.muchute = uint16(v.varD7 + 2)
		if a.muattac != 0 {
			a.dattac = a.maxvolume / a.muattac
		}
		if a.muchute != 0 {
			a.dchute = a.maxvolume / a.muchute
		}
		if v.mem[a.mupnote-6] == 0 {
			v.mv1Gomusic()
		}
		a.mustate = 1
	case 4:
		a.muvolume = 0
		a.mupnote = addr + 6
		v.readexecOpername()
		a.maxvolume = uint16(v.varD7 << 8)
		v.readexecOpername()
		a.mutempo = uint8(v.varD7)
		v.readexecOpername()
		a.muattac = uint16(v.varD7 + 1)
		v.readexecOpername()
		a.muduree = int16(v.varD7)
		v.readexecOpername()
		a.muchute = uint16(v.varD7 + 1)
		a.dattac = 0
		a.dchute = 0
		if a.muattac != 0 {
			a.dattac = a.maxvolume / a.muattac
		}
		if a.muchute != 0 {
			a.dchute = a.maxvolume / a.muchute
		}
		a.muvolume = uint8(a.maxvolume >> 8)
		if a.fmusic != 0 {
			v.mv2Gomusic()
		}
		a.mustate = 1
	default:
		for range 5 {
			v.readexecOpername()
		}
	}
}

func cmusic(v *VM) {
	v.mem[dsFlagmain] = 0
	v.music()
}

func cmmusic(v *VM) {
	v.mem[dsFlagmain] = 1
	v.music()
}

func cdelmusic(v *VM) {
	v.readexecOpername()
	if v.aud.rout == routMV1 {
		v.mv1Offmusic(uint32(v.varD7))
	} else {
		v.mv2Offmusic(uint32(v.varD7))
	}
}

func ccadence(v *VM) {
	v.readexecOpername()
	if v.varD7 == 0 {
		v.varD7 = 1
	}
	v.aud.mutempo = uint8(v.varD7)
	v.aud.mutemp = uint8(int32(v.aud.mutempo) * 6 / 0x20)
}

func csetvolum(v *VM) {
	v.readexecOpername()
	a := &v.aud
	a.muvolume = uint8(v.varD7)
	a.maxvolume = a.maxvolume&0xff00 | uint16(uint8(v.varD7))
	a.muvol = a.muvolume>>1 + 1
	if a.muattac != 0 {
		a.dattac = a.maxvolume / a.muattac
	}
	if a.muchute != 0 {
		a.dchute = a.maxvolume / a.muchute
	}
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

func (v *VM) sound() {
	v.readexecOpername()
	index := uint8(v.varD7)
	v.readexecOpername()
	priorson := int8(v.varD7)
	v.readexecOpername()
	volson := uint8(v.varD7)
	v.readexecOpername()
	loopsam := uint16(v.varD7)
	v.readexecOpername()
	speedsam := int8(v.varD7)
	addr := uint32(v.adresmus(int32(index)))
	if t := v.mem[addr]; t == 1 || t == 2 {
		if speedsam == 0 {
			speedsam = int8(v.mem[addr+1])
		}
		longsam := uint32(v.r32(addr+2)) - 0x10
		v.playsample(chanSample, addr+0x10, speedsam, volson, longsam, loopsam, priorson)
	}
}

func csound(v *VM) {
	v.mem[dsFlagmain] = 0
	v.sound()
}

func cmsound(v *VM) {
	v.mem[dsFlagmain] = 1
	v.sound()
}

func credon(v *VM)    { v.mem[v.script.vramOrg-0x25] = 0 }
func credoff(v *VM)   { v.mem[v.script.vramOrg-0x25] = 0x80 }
func cdelsound(v *VM) { v.offsound() }
