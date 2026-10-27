package alis

// Music and sound opcodes (START.EXE 0x9464..0x9677).

// sampleAddr is 0x9484: resource idx of the running script, of the main
// script with dsFlagmain, without a range check.
func (v *VM) sampleAddr(idx int16) uint32 {
	vram := v.script.vramOrg
	if v.mem[dsFlagmain] != 0 {
		vram = v.main.vramOrg
	}
	b := v.scriptOrg(vram)
	b += v.ru32(b + 0xe)
	b += v.ru32(b + 0xc)
	b += uint32(int32(idx * 4))
	return b + v.ru32(b)
}

// cmusic (0x9511) plays a module (types 0, 3, 4): its volume, an unused
// byte, the fade in, hold and fade out ticks.
func (v *VM) music() {
	a := v.sampleAddr(v.getval())
	if t := v.mem[a]; t != 0 && t != 3 && t != 4 {
		for range 5 {
			v.getval()
		}
		return
	}
	v.wu32(dsSong, a+6)
	v.mem[dsMusVol] = uint8(v.getval())
	v.mem[0x160a1] = uint8(v.getval())
	v.wu16(dsMusFade, uint16(v.getval()))
	v.wu16(dsMusHold, uint16(v.getval()))
	v.wu16(dsMusOut, uint16(v.getval()))
	v.musicStart()
}

func cmusic(v *VM) {
	v.mem[dsFlagmain] = 0
	v.music()
}

func cmmusic(v *VM) {
	v.mem[dsFlagmain] = 1
	v.music()
}

// cdelmusic (0x9597) fades the song out over its operand's ticks.
func cdelmusic(v *VM) {
	v.wu16(dsMusOut, uint16(v.getval()))
	v.wu16(dsMusFade, 0)
	v.wu16(dsMusHold, 1)
	v.musicFade(v.ru16(dsMusOut))
}

// ccadence (0x9476) calls an empty routine.
func ccadence(v *VM) { v.getval() }

// csetvolum (0x9464) sets the music volume for the next song.
func csetvolum(v *VM) {
	d := uint8(v.getval())
	v.mem[0x161c6] = d
	v.mem[dsMusVol] = d
}

// instru (0x94be) puts sample resource (types 1, 2, 6) into slot n of the
// music's sample table.
func (v *VM) instru() {
	v.readexecOpername()
	slot := uint32(uint16(v.varD7)) & 0x1f
	v.readexecOpername()
	a := v.sampleAddr(int16(v.varD7))
	v.readexecOpername()
	if t := v.mem[a]; t != 1 && t != 2 && t != 6 {
		return
	}
	v.wu32(dsSamples+slot*8, a+0x10)
	v.wu32(dsSamples+slot*8+4, uint32(int32(int16(v.varD7))))
}

func cinstru(v *VM) {
	v.mem[dsFlagmain] = 0
	v.instru()
}

func cminstru(v *VM) {
	v.mem[dsFlagmain] = 1
	v.instru()
}

// sound (0x95e8) plays a sample resource on an effect channel: its index,
// priority, volume (0..0xbf), plays (< 0 loops) and kHz.
func (v *VM) sound() {
	idx := v.getval()
	prio := uint16(v.getval())
	ch := v.sbAlloc(v.ru16(v.script.vramOrg-0xe), prio)
	if ch == 0 {
		for range 3 {
			v.getval()
		}
		return
	}
	v.mem[ch] = 0
	a := v.sampleAddr(idx)
	v.wu32(ch+chStart, a+0xe)
	v.wu32(ch+chLoop, a+0xe)
	end := v.ru32(a+2) - 0xe + a + 6
	v.wu32(ch+chEnd, end)
	v.wu32(ch+chLoopEnd, end)
	v.wu32(ch+0x1c, end)
	v.readexecOpername()
	v.mem[ch+chVol] = uint8(v.varD7) >> 1
	v.readexecOpername()
	v.wu16(ch+chCount, uint16(v.varD7))
	v.readexecOpername()
	v.wu16(ch+chFreq, uint16(v.varD7)*1000)
	v.sbStart(ch)
}

func csound(v *VM) {
	v.mem[dsFlagmain] = 0
	v.sound()
}

func cmsound(v *VM) {
	v.mem[dsFlagmain] = 1
	v.sound()
}

// cdelsound (0x966e) stops the effect channels.
func cdelsound(v *VM) { v.sbStop(-1) }
