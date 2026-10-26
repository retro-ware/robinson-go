package alis

// Control flow, expressions and entity life cycle opcodes (alis opcodes.c).

// cnul is a bare ret in START.EXE (0x38ef); alis stops the VM.
func cnul(v *VM) {}

func cesc1(v *VM) { v.readexecCodesc1() }
func cesc2(v *VM) { v.readexecCodesc2() }
func cesc3(v *VM) { v.readexecCodesc3() }

// not implemented in ALIS 3.1 either
func cbreakpt(v *VM)  {}
func cjsrabs(v *VM)   {}
func cjmpabs(v *VM)   {}
func cjsrind16(v *VM) {}
func cjsrind24(v *VM) {}
func cjmpind16(v *VM) {}
func cjmpind24(v *VM) {}
func cmul(v *VM)      {}
func cdiv(v *VM)      {}
func cscdump(v *VM)   {}
func cautobase(v *VM) {}
func cscantab(v *VM)  {}
func cneartab(v *VM)  {}

func cstore(v *VM) {
	v.readexecOpernameSaveD7()
	v.cstoreContinue()
}

func ceval(v *VM) { v.readexecOpernameSaveD7() }

func cadd(v *VM) {
	v.readexecOpernameSaveD7()
	v.readexecAddnameSwap()
}

func csub(v *VM) {
	v.readexecOpernameSaveD7()
	v.varD7 = -v.varD7
	v.readexecAddnameSwap()
}

func cvprint(v *VM) {
	v.charmode = 0
	v.readexecOpernameSaveD7()
	v.putval(int16(v.varD7))
}

func csprinti(v *VM) {
	v.charmode = 0
	for c := v.read8(); c != 0; c = v.read8() {
		v.putChar(int8(c))
	}
}

func csprinta(v *VM) {
	v.charmode = 0
	v.readexecOpernameSaveD7()
	v.putString()
}

func clocate(v *VM) {
	v.charmode = 0
	v.readexecOpernameSaveD7()
	v.readexecOpernameSaveD6()
	v.xlocate = uint8(v.varD7)
	v.ylocate = uint8(v.varD6)
}

func ctab(v *VM) {
	v.charmode = 0
	v.readexecOpernameSaveD7()
	v.xlocate = uint8(v.varD7)
}

func cdim(v *VM) {
	offset := v.read16()
	counter := v.read8()
	byte2 := v.read8()
	vram := v.script.vramOrg
	at := func() uint32 { return uint32(int32(vram) + int32(offset)) }

	offset--
	v.w8(at(), counter)
	offset--
	v.w8(at(), byte2)

	n := int16(counter & 0xf)
	if int8(counter) >= 0 {
		for ; n > 0; n-- {
			offset -= 2
			v.w16(at(), v.read16())
		}
	} else {
		for ; n > 0; n-- {
			offset -= 4
			v.w32(at(), v.read32())
		}
	}
	offset -= 4
	v.w32(at(), v.read32())
}

func crandom(v *VM) {
	v.readexecOpername()
	v.randomNumber = uint16(v.varD7)
	if v.randomNumber == 0 {
		v.randomNumber = v.host.Random()
	}
}

// readOff24 reads a 24-bit branch offset: on the PC only the low word counts.
func (v *VM) readOff24() int32 {
	o := v.read16()
	v.read8()
	return int32(o)
}

func (v *VM) jump(offset int32) { v.script.pc = uint32(int32(v.script.pc) + offset) }

func (v *VM) cloop(offset int32) {
	save := v.script.pc
	v.varD7 = -1
	v.readexecAddnameSwap()
	if !v.srZero {
		v.script.pc = save
		v.jump(offset)
	}
}

func cloop8(v *VM)  { v.cloop(int32(int8(v.read8()))) }
func cloop16(v *VM) { v.cloop(int32(v.read16())) }
func cloop24(v *VM) { v.cloop(v.readOff24()) }

func cswitch1(v *VM) {
	v.readexecOpername()
	val := int16(v.varD7)
	addition := int16(v.read8())
	if v.script.pc&1 != 0 {
		v.script.pc++
	}
	for {
		test := v.read16()
		if val == test {
			v.jump(int32(v.read16()))
			return
		}
		v.script.pc += 2
		if !(val > test) {
			break
		}
		addition--
		if addition <= -1 {
			break
		}
	}
	if addition > 0 {
		v.jump(int32(addition) * 4)
	}
}

func cswitch2(v *VM) {
	v.readexecOpername()
	addition := int16(v.read8())
	if v.script.pc&1 != 0 {
		v.script.pc++
	}
	val := int16(v.varD7 + int32(v.read16()))
	v.varD7 = int32(val)
	if val >= 0 && val <= addition {
		v.varD7 = int32(val) * 2
		v.jump(v.varD7)
		v.jump(int32(v.read16()))
	} else {
		v.jump(int32(addition)*2 + 2)
	}
}

// virtual accumulator of the running script (vram-relative)
func (v *VM) vaccAt() uint32 { return uint32(int32(v.script.vramOrg) + int32(v.script.vaccOff)) }

func (v *VM) xpush32(x int32) {
	v.script.vaccOff -= 4
	v.w32(v.vaccAt(), x)
}

func (v *VM) xpeek32() int32 { return v.r32(v.vaccAt()) }

func (v *VM) xpop32() int32 {
	x := v.xpeek32()
	v.script.vaccOff += 4
	return x
}

func cleave(v *VM) {
	vram := v.script.vramOrg
	if v.fseq == 0 {
		if off := v.r16(vram - 0x0c); off != 0 {
			v.w16(vram-0x0c, 0)
			v.w32(vram-0x08, v.r32(uint32(int32(vram)+int32(off))))
			v.w16(vram-0x0a, off+4)
			return
		}
	} else {
		v.script.vaccOff = v.r16(vram - 0x0c)
		if v.script.vaccOff != 0 {
			v.w16(vram-0x0c, 0)
			v.script.pc = uint32(int32(v.script.pcOrg) + v.xpop32())
			return
		}
	}
	cret(v)
}

func cprotect(v *VM) {
	v.readexecOpername()
	v.vprotect = int8(v.varD7)
}

func casleep(v *VM) {
	v.readexecOpername()
	if v.varD7 > -1 {
		if vram := v.r32(v.atent + uint32(v.varD7)); vram != 0 {
			v.w8(uint32(vram)-0x04, 0)
		}
	}
}

func (v *VM) scrAdd16(idx uint16, off uint32, x int16) {
	v.add16(v.sca(idx)+off, x)
	v.setScrState(idx, v.scrState(idx)|-0x80)
}

func cscmov(v *VM) {
	v.readexecOpernameSaveD7()
	x := int16(v.varD7)
	v.readexecOpernameSaveD7()
	y := int16(v.varD7)
	v.readexecOpernameSaveD7()
	z := int16(v.varD7)
	idx := v.curScreenID()
	if z != 0 {
		v.scrAdd16(idx, scDepz, z)
	}
	if y != 0 {
		v.scrAdd16(idx, scDepy, y)
	}
	if x != 0 {
		v.scrAdd16(idx, scDepx, x)
	}
}

func cscset(v *VM) {
	v.readexecOpername()
	x := int16(v.varD7)
	v.readexecOpername()
	y := int16(v.varD7)
	v.readexecOpername()
	z := int16(v.varD7)
	idx := v.curScreenID()
	a := v.sca(idx)
	if z != v.r16(a+scDepz) {
		v.w16(a+scDepz, z)
		v.setScrState(idx, v.scrState(idx)|-0x80)
	}
	if y != v.r16(a+scDepy) {
		v.w16(a+scDepy, y)
		v.setScrState(idx, v.scrState(idx)|-0x80)
	}
	if x != v.r16(a+scDepx) {
		v.w16(a+scDepx, x)
		v.setScrState(idx, v.scrState(idx)|-0x80)
	}
}

func cclipping(v *VM)  { v.fswitch = 0 }
func cswitching(v *VM) { v.fswitch = 1 }

// formAt is the address of form idx of the running script, unchecked
// (START.EXE 0x7139).
func (v *VM) formAt(idx int16) uint32 {
	addr := v.scriptOrg(v.script.vramOrg)
	addr += uint32(v.r32(addr + 0xe))
	addr += uint32(v.r32(addr + 6))
	return uint32(int32(addr) + int32(v.r16(uint32(int32(addr)+int32(idx*2)))))
}

// getVector reads a form index and returns the centre of its box, mirrored
// with the entity (START.EXE 0x7176).
func (v *VM) getVector(x, y, z *int16) {
	v.readexecOpername()
	frm := v.formAt(int16(v.varD7))
	s8 := func(o uint32) int16 { return int16(int8(v.r8(frm + o))) }
	switch test := int8(v.r8(frm)); {
	case test == 0:
		*x = s8(4) + s8(7)>>1
		*y = s8(5) + s8(8)>>1
		*z = s8(6) + s8(9)>>1
	case test < 0:
		v.alisError(0x14)
		return
	case test == 1:
		*x = v.r16(frm+4) + v.r16(frm+0xa)>>1
		*y = v.r16(frm+6) + v.r16(frm+0xc)>>1
		*z = v.r16(frm+8) + v.r16(frm+0xe)>>1
	default:
		*x = v.r16(frm + 2)
		*y = v.r16(frm + 4)
		*z = v.r16(frm + 6)
	}
	if int8(v.r8(v.script.vramOrg-0x2e)) >= 0x15 && v.xinv(v.script.vramOrg) != 0 {
		*x = -*x
	}
}

func cwlive(v *VM) {
	v.getVector(&v.wcx, &v.wcy, &v.wcz)
	v.varD7 = -1
	v.clivin()
}

func (v *VM) progEnd(index int16) uint32 {
	dataOrg := v.atprogAt(int(index))
	dataEnd := uint32(0xfffffff)
	for i := 0; i < int(v.nbprog); i++ {
		if a := v.atprogAt(i); dataOrg < a && a < dataEnd {
			dataEnd = a
		}
	}
	if dataEnd == 0xfffffff {
		dataEnd = v.finprog
	}
	return dataEnd
}

func cunload(v *VM) {
	id := v.read16()
	if id == -1 || uint16(id) == v.ru16(v.script.vramOrg-0x10) {
		return
	}
	index := v.debprotf(id)
	if index == -1 {
		return
	}
	dataOrg := v.atprogAt(int(index))
	v.shrinkprog(int32(dataOrg), int32(v.progEnd(index)-dataOrg), uint16(id))
}

func cwakeup(v *VM) {
	v.readexecOpername()
	if v.varD7 > -1 {
		if vram := v.r32(v.atent + uint32(v.varD7)); vram != 0 {
			v.w8(uint32(vram)-0x04, 1)
		}
	}
}

func cstop(v *VM) { v.script.running = false }

func csleep(v *VM) {
	v.w8(v.script.vramOrg-0x04, 0)
	if v.fseq != 0 {
		cstop(v)
	}
}

func clive(v *VM) {
	v.wcx, v.wcy, v.wcz = 0, 0, 0
	v.varD7 = -1
	v.clivin()
}

func (v *VM) clivin() {
	var prog *scriptData
	id := v.read16()
	if id == -1 {
		v.varD7 = -1
	} else if index := v.debprotf(id); index != -1 {
		prog = v.loadedScripts[index]
	}

	if prog != nil {
		s := v.scriptLive(prog)
		prev := v.script.vramOrg
		next := s.vramOrg
		cp32 := func(o uint32) { v.w32(next+o, v.r32(prev+o)) }
		cp16 := func(o uint32) { v.w16(next+o, v.r16(prev+o)) }
		cp32(0x00)
		cp32(0x08)
		cp32(0x10)
		cp16(0x30)
		cp16(scrWCX2)
		cp16(scrWCY2)
		cp16(scrWCZ2)
		cp32(scrWCAX)
		cp32(scrWCAY)
		cp32(scrWCAZ)
		cp16(0x40)
		cp16(0x44)
		cp16(0x48)

		v.add16(next+scrWCX, v.wcx)
		v.add16(next+scrWCY, v.wcz)
		v.add16(next+scrWCZ, v.wcy)

		v.w16(next-0x28, v.r16(prev-0x0e))
		v.w16(next-0x16, v.r16(prev-0x16))
		v.w16(next-0x22, v.r16(prev-0x22))
		v.w16(next-0x2a, v.r16(next-0x0e))

		if v.maxent < uint32(v.nbent) {
			return
		}
	}
	v.cstoreContinue()
}

func ckill(v *VM) {
	v.readexecOpernameSaveD7()
	if v.varD7 < 0 {
		return
	}
	v.killent(uint16(v.varD7))
}

func cstopret(v *VM) {
	if v.fseq == 0 {
		cstop(v)
	}
	cret(v)
	v.script.running = false
}

func cexit(v *VM) {
	if v.varD5 == 0 {
		v.cstopret = 1
		v.state = stateStopped
		v.script.running = false
		return
	}
	v.killent(v.varD5)
	v.script.running = false
}

func cload(v *VM) {
	id := v.read16()
	if id != 0 {
		name := v.readZeroString()
		if i := lastDot(name); i >= 0 {
			name = name[:i+1] + "io"
		}
		if v.scriptLoad(name) == nil {
			v.cstopret = 1
			v.state = stateStopped
			v.script.running = false
		}
		return
	}
	v.readexecOpernameSwap()
}

func lastDot(s string) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '.' {
			return i
		}
	}
	return -1
}

func cdefsc(v *VM) {
	idx := uint16(v.read16())
	v.setScrState(idx, v.scrState(idx)|0x40)
	a := v.sca(idx)
	v.mem[a+scNumelem] = v.read8()
	v.wu16(a+scScreenID, v.img.libsprit)
	for i := uint32(0); i < 32; i++ {
		v.mem[a+6+i] = v.read8()
	}

	sp := v.spr(v.img.libsprit)
	sp.setLink(0)
	sp.setNumelem(int8(v.mem[a+scNumelem]))
	x := v.r16(a + scNewx)
	y := v.r16(a + scNewy)
	w := v.r16(a + scWidth)
	h := v.r16(a + scHeight)
	sp.setNewx(x)
	sp.setNewy(y)
	sp.setNewd(0x7fff)
	sp.setDepx(x + w)
	sp.setDepy(y + h)

	v.img.libsprit = sp.toNext()
	v.wu16(a+scToNext, 0)
	v.scdosprite(idx)
	v.w16(a+0x2a, 0)
	v.w16(a+0x2c, 0)
	v.w16(a+0x2e, 0)
	v.img.libsprit = sp.toNext()
	v.scadd(idx)
	v.vectoriel(idx)
}

func cscreen(v *VM) {
	id := uint16(v.read16())
	if id != v.curScreenID() {
		v.wu16(v.script.vramOrg-0x16, id)
	}
}

func cput(v *VM) {
	v.flagmain = 0
	v.img.invertX = v.xinv(v.script.vramOrg)
	v.readexecOpername()
	v.img.depx, v.img.depy, v.img.depz = 0, 0, 0
	v.img.numelem = 0
	v.put(uint16(v.varD7))
}

func cputnat(v *VM) {
	v.flagmain = 0
	v.img.invertX = v.xinv(v.script.vramOrg)
	v.readexecOpernameSaveD7()
	v.img.depx = int16(v.varD7)
	v.readexecOpernameSaveD7()
	v.img.depy = int16(v.varD7)
	v.readexecOpernameSaveD7()
	v.img.depz = int16(v.varD7)
	v.readexecOpernameSaveD7()
	v.readexecOpernameSaveD6()
	v.img.numelem = int8(v.varD6)
	v.put(uint16(v.varD7))
}

func cerase(v *VM) {
	var cur, prev uint16
	if v.searchtete(&cur, &prev) {
		screen := int16(v.curScreenID())
		for {
			v.killelem(&cur, &prev)
			if cur == 0 {
				v.ferase = 0
				return
			}
			if screen != v.spr(cur).screenID() {
				break
			}
		}
	}
	v.ferase = 0
}

func cerasen(v *VM) {
	v.readexecOpernameSaveD7()
	v.img.numelem = int8(v.varD7)
	var cur, prev uint16
	for v.searchelem(&cur, &prev) {
		v.killelem(&cur, &prev)
	}
	v.ferase = 0
}

func cset(v *VM) {
	vram := v.script.vramOrg
	v.readexecOpername()
	v.w16(vram+scrWCX, int16(v.varD7))
	v.readexecOpername()
	v.w16(vram+scrWCY, int16(v.varD7))
	v.readexecOpername()
	v.w16(vram+scrWCZ, int16(v.varD7))
}

func cmov(v *VM) {
	vram := v.script.vramOrg
	v.readexecOpername()
	v.add16(vram+scrWCX, int16(v.varD7))
	v.readexecOpername()
	v.add16(vram+scrWCY, int16(v.varD7))
	v.readexecOpername()
	v.add16(vram+scrWCZ, int16(v.varD7))
}

func copensc(v *VM) {
	id := uint16(v.read16())
	a := v.sca(id)
	v.mem[a] &= 0xbf
	v.mem[a] |= 0x80
	v.scbreak(id)
	v.scadd(id)
	if v.mem[a+1]&2 != 0 {
		v.openland(id)
	}
}

func cclosesc(v *VM) {
	id := uint16(v.read16())
	v.mem[v.sca(id)] |= 0x40
}

func cerasall(v *VM) {
	var tmp uint16
	cur := v.elemHead()
	for cur != 0 {
		v.killelem(&cur, &tmp)
	}
	v.ferase = 0
}

func cforme(v *VM) {
	v.readexecOpername()
	v.w16(v.script.vramOrg-0x1a, int16(v.varD7))
}

func cdelforme(v *VM) { v.w16(v.script.vramOrg-0x1a, -1) }

func cclock(v *VM) {
	v.readexecOpername()
	vram := v.script.vramOrg
	v.wu16(vram-0x3a, uint16(v.varD7))
	v.wu32(vram-0x3e, uint32(v.varD7)+v.timeclock)
}

func cret(v *VM) {
	if v.fseq == 0 && v.saversp <= v.script.vaccOff {
		cstop(v)
		return
	}
	v.script.pc = uint32(int32(v.script.pcOrg) + v.xpeek32())
	if v.script.vaccOff == v.r16(v.script.vramOrg-0x0c) {
		v.w16(v.script.vramOrg-0x0c, 0)
	}
	v.script.vaccOff += 4
}

func (v *VM) cjsr(offset int32) {
	v.xpush32(int32(v.script.pc - v.script.pcOrg))
	v.jump(offset)
}

func cjsr8(v *VM)  { v.cjsr(int32(int8(v.read8()))) }
func cjsr16(v *VM) { v.cjsr(int32(v.read16())) }
func cjsr24(v *VM) { v.cjsr(v.readOff24()) }

func cjmp8(v *VM)  { v.jump(int32(int8(v.read8()))) }
func cjmp16(v *VM) { v.jump(int32(v.read16())) }
func cjmp24(v *VM) { v.jump(v.readOff24()) }

func (v *VM) bztest() bool  { return int16(v.varD7) == 0 }
func (v *VM) beqtest() bool { return int16(v.varD7) == int16(v.varD6) }

func cbz8(v *VM) {
	if v.bztest() {
		v.jump(int32(int8(v.read8())))
	} else {
		v.script.pc++
	}
}

func cbz16(v *VM) {
	if v.bztest() {
		v.jump(int32(v.read16()))
	} else {
		v.script.pc += 2
	}
}

func cbz24(v *VM) {
	if v.bztest() {
		v.jump(v.readOff24())
	} else {
		v.script.pc += 3
	}
}

func cbnz8(v *VM) {
	if !v.bztest() {
		v.jump(int32(int8(v.read8())))
	} else {
		v.script.pc++
	}
}

func cbnz16(v *VM) {
	if !v.bztest() {
		v.jump(int32(v.read16()))
	} else {
		v.script.pc += 2
	}
}

func cbnz24(v *VM) {
	if !v.bztest() {
		v.jump(v.readOff24())
	} else {
		v.script.pc += 3
	}
}

func (v *VM) cbcond(jump bool, bits int) {
	var offset int32
	switch bits {
	case 8:
		offset = int32(int8(v.read8()))
	case 16:
		offset = int32(v.read16())
	default:
		offset = v.readOff24()
	}
	if jump {
		v.jump(offset)
	}
}

func cbeq8(v *VM)  { v.cbcond(v.beqtest(), 8) }
func cbeq16(v *VM) { v.cbcond(v.beqtest(), 16) }
func cbeq24(v *VM) { v.cbcond(v.beqtest(), 24) }
func cbne8(v *VM)  { v.cbcond(!v.beqtest(), 8) }
func cbne16(v *VM) { v.cbcond(!v.beqtest(), 16) }
func cbne24(v *VM) { v.cbcond(!v.beqtest(), 24) }

func (v *VM) cstart(offset int32) {
	vram := v.script.vramOrg
	if v.fseq == 0 {
		target := uint32(offset + int32(v.script.pc))
		if v.r16(vram-0x0c) == 0 {
			off := v.r16(vram-0x0a) - 4
			v.w16(vram-0x0c, off)
			v.w16(vram-0x0a, off)
			v.w32(uint32(int32(vram)+int32(off)), int32(v.ru32(vram-0x08)-v.script.pcOrg))
			v.wu32(vram-0x08, target)
		} else {
			v.w16(vram-0x0a, v.r16(vram-0x0c))
			v.wu32(vram-0x08, target)
		}
		v.w8(vram-0x04, 1)
		v.w8(vram-0x01, 1)
		v.wu32(vram-0x3e, 0)
		return
	}
	if v.r16(vram-0x0c) == 0 {
		v.xpush32(int32(v.script.pc - v.script.pcOrg))
		v.w16(vram-0x0c, v.script.vaccOff)
	} else {
		v.script.vaccOff = v.r16(vram - 0x0c)
	}
	v.jump(offset)
}

func cstart8(v *VM)  { v.cstart(int32(int8(v.read8()))) }
func cstart16(v *VM) { v.cstart(int32(v.read16())) }
func cstart24(v *VM) { v.cstart(v.readOff24()) }

// killent removes an entity and compacts the entity memory behind it.
func (v *VM) killent(ent uint16) {
	vram := uint32(v.r32(v.atent + uint32(ent)))
	if vram == 0 {
		v.ferase = 0
		return
	}
	s := v.entscr(ent)
	if s.data != nil && s.data.header.id == v.aud.musicID {
		v.sleepUntilMusicStops()
	}

	v.script = v.entscr(ent)
	cerasall(v)
	v.script = v.entscr(v.varD5)

	org := uint32(v.r32(vram - 0x14))
	datasize := uint32(int32(v.r16(org+0x16)) + int32(v.r16(org+0x12)) + contextSz)
	target := vram - datasize
	shrinkby := uint32(int32(v.r16(org+0x14)) + int32(datasize))
	source := target + shrinkby
	for {
		v.mem[target] = v.mem[source]
		v.mem[target+1] = v.mem[source+1]
		target += 2
		source += 2
		if source >= v.finent {
			break
		}
	}
	v.finent -= shrinkby

	var cur, prev uint16
	for {
		next := v.ru16(v.atent + 4 + uint32(prev))
		if ent == next {
			cur = prev
		}
		if next == 0 {
			break
		}
		prev = next
		if int32(vram) <= v.r32(v.atent+uint32(next)) {
			v.entscr(next).vramOrg -= shrinkby
			v.w32(v.atent+uint32(next), v.r32(v.atent+uint32(next))-int32(shrinkby))
		}
	}

	v.wu16(v.atent+uint32(cur)+4, v.ru16(v.atent+4+uint32(ent)))
	v.w32(v.atent+uint32(ent), 0)
	v.w16(v.atent+uint32(ent)+4, v.dernent)
	v.dernent = int16(ent)
	v.nbent--

	if v.varD5 == ent {
		v.varD5 = cur
		v.restartLoop = true
		cstop(v)
	}
}

// shrinkprog removes length bytes of program memory at start and relocates
// everything that points behind it.
func (v *VM) shrinkprog(start, length int32, id uint16) {
	if int32(v.finprog)-length > start {
		copy(v.mem[start:], v.mem[start+length:int32(v.finprog)])
	}
	v.finprog -= uint32(length)

	v.audioRelocate(uint32(start), uint32(start+length), -length)

	if id != 0 {
		idx := -1
		for i := 0; i < int(v.nbprog); i++ {
			if v.loadedScripts[i].header.id == id {
				idx = i
			}
			if idx >= 0 {
				if i < int(v.maxprog) {
					v.loadedScripts[i] = v.loadedScripts[i+1]
				} else {
					v.loadedScripts[i] = nil
				}
			}
		}
		found := false
		for i := 0; i < int(v.nbprog); i++ {
			if uint32(start) == v.atprogAt(i) {
				found = true
			}
			if found {
				if i < int(v.maxprog) {
					v.atprogSet(i, v.atprogAt(i+1))
				} else {
					v.atprogSet(i, 0)
				}
			}
		}
		v.dernprog -= 4
		v.nbprog--
	}

	for i := 0; i < int(v.nbprog); i++ {
		s := v.loadedScripts[i]
		if uint32(start) <= s.dataOrg {
			s.dataOrg -= uint32(length)
			v.atprogSet(i, v.atprogAt(i)-uint32(length))
		}
	}

	if id != 0 {
		var ent, prev uint32
		for {
			ent = uint32(v.ru16(v.atent + 4 + ent))
			if ent == 0 {
				break
			}
			s := v.entscr(uint16(ent))
			if id == v.ru16(s.vramOrg-0x10) {
				v.killent(uint16(ent))
				ent = prev
			} else if uint32(start) <= v.ru32(s.vramOrg-0x14) {
				s.pc -= uint32(length)
				s.pcOrg -= uint32(length)
				v.wu32(s.vramOrg-0x14, v.ru32(s.vramOrg-0x14)-uint32(length))
				v.wu32(s.vramOrg-0x08, v.ru32(s.vramOrg-0x08)-uint32(length))
				org := v.ru32(s.vramOrg - 0x14)
				s.vaccOff = int16(-contextSz - int32(v.r16(org+0x16)))
			}
			prev = ent
		}
	}

	for sc := int16(v.scr.ptscreen); sc != 0; sc = int16(v.scrToNext(uint16(sc))) {
		idx := v.scrScreenID(uint16(sc))
		if idx == 0 {
			continue
		}
		for {
			idx = v.spr(idx).link()
			if idx == 0 {
				break
			}
			sp := v.spr(idx)
			end := uint32(start + length)
			if sp.state() == 1 || (sp.newad() >= uint32(start) && sp.newad() < end) {
				sp.setNewad(0)
			} else if uint32(start) < sp.newad() {
				sp.setNewad(sp.newad() - uint32(length))
			}
			if sp.data() >= uint32(start) && sp.data() < end {
				sp.setData(0)
			} else if uint32(start) < sp.data() {
				sp.setData(sp.data() - uint32(length))
			}
		}
	}
}
