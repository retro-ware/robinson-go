package alis

// Movement tests, puts, time, text input/output and messages (alis opcodes.c).

func cwmov(v *VM) {
	var x, y, z int16
	v.getVector(&x, &y, &z)
	vram := v.script.vramOrg
	v.add16(vram+scrWCX, x)
	v.add16(vram+scrWCY, y)
	v.add16(vram+scrWCZ, z)
}

func cwtstmov(v *VM)  {}
func cwftstmov(v *VM) {}

func ctstform(v *VM) {
	vram := v.script.vramOrg
	v.wcx = v.r16(vram + scrWCX)
	v.wcy = v.r16(vram + scrWCY)
	v.wcz = v.r16(vram + scrWCZ)
	v.readexecOpername()
	v.matmask = int16(v.varD7)
	v.readexecOpername()
	v.wforme = int16(v.varD7)
	v.clipform()
	crstent(v)
}

// putAt implements the cxput/cmput family.
func (v *VM) putAt(main bool, invert uint8, at bool) {
	v.flagmain = 0
	if main {
		v.flagmain = 1
	}
	v.img.invertX = invert
	if !at {
		v.readexecOpername()
		v.img.depx, v.img.depy, v.img.depz = 0, 0, 0
		v.img.numelem = 0
		v.put(int16(v.varD7))
		return
	}
	v.readexecOpernameSaveD7()
	v.img.depx = int16(v.varD7)
	v.readexecOpernameSaveD7()
	v.img.depy = int16(v.varD7)
	v.readexecOpernameSaveD7()
	v.img.depz = int16(v.varD7)
	v.readexecOpernameSaveD7()
	v.readexecOpernameSaveD6()
	v.img.numelem = int8(v.varD6)
	v.put(int16(v.varD7))
}

func cxput(v *VM)    { v.putAt(false, v.xinv(v.script.vramOrg)^1, false) }
func cxputat(v *VM)  { v.putAt(false, v.xinv(v.script.vramOrg)^1, true) }
func cmput(v *VM)    { v.putAt(true, v.xinv(v.script.vramOrg), false) }
func cmputat(v *VM)  { v.putAt(true, v.xinv(v.script.vramOrg), true) }
func cmxput(v *VM)   { v.putAt(true, v.xinv(v.script.vramOrg)^1, false) }
func cmxputat(v *VM) { v.putAt(true, v.xinv(v.script.vramOrg)^1, true) }

func cmforme(v *VM) {
	v.readexecOpername()
	v.w16(v.script.vramOrg-0x1a, int16(v.varD7))
}

// The game clock is the host time of day shifted by csettime.
// TODO(START.EXE): check what DOS calls the original makes here.
func csettime(v *VM) {
	v.readexecOpername()
	h := int(uint16(v.varD7))
	v.readexecOpername()
	m := int(uint16(v.varD7))
	v.readexecOpername()
	s := int(uint16(v.varD7))
	hh, mm, ss := v.host.Time()
	v.timeOffset = (h*3600 + m*60 + s) - (hh*3600 + mm*60 + ss)
}

// TODO(START.EXE): alis returns 0,0,0 (the stub sys_get_time is 0).
func cgettime(v *VM) {
	hh, mm, ss := v.host.Time()
	t := ((hh*3600+mm*60+ss+v.timeOffset)%86400 + 86400) % 86400
	v.varD7 = int32(t / 3600)
	v.cstoreContinue()
	v.varD7 = int32(t / 60 % 60)
	v.cstoreContinue()
	v.varD7 = int32(t % 60)
	v.cstoreContinue()
}

// readLine is getval/getstring: it reads keys until return and echoes
// them; digits limits the accepted keys to '-' and '0'..'9'.
func (v *VM) readLine(digits bool) {
	p := v.sd7
	for v.state != stateStopped {
		c := v.ioGetkey()
		if c == '\r' {
			break
		}
		if c == '\b' {
			if p != v.sd7 {
				p--
				v.mem[p] = 0
				v.putChar('\b')
				v.putChar(' ')
				v.putChar('\b')
			}
			continue
		}
		if !digits || c == 0x2d || (int8(c-0x30) > -1 && c < 0x3a) {
			v.mem[p] = c
			p++
			v.putChar(int8(c))
		}
	}
	v.mem[p] = 0
}

func cvinput(v *VM) {
	v.charmode = 0
	v.readLine(true)
	oval(v)
	v.cstoreContinue()
}

func csinput(v *VM) {
	v.charmode = 0
	v.readLine(false)
	v.cstoreContinue()
}

// sendmess queues a one-word message to the entity at vram.
func (v *VM) sendmess(vram uint32, idx uint16) {
	if v.mem[vram-0x24]&1 != 0 {
		return
	}
	val := v.r16(vram-0x1c) + 2
	if -0x3f < val {
		val -= v.r16(uint32(v.r32(vram-0x14)) + 0x16)
	}
	if val == v.r16(vram-0x1e) {
		v.w16(vram-0x1c, val)
		return
	}
	v.wu16(uint32(int32(vram)+int32(v.r16(vram-0x1c))), idx)
	v.w16(vram-0x1c, val)
	v.mem[vram-0x24] |= 0x80
}

func casleepfar(v *VM) {
	v.readexecOpername()
	norme := uint32(v.varD7&0xffff) * uint32(v.varD7&0xffff)
	v.valnorme = int32(norme)
	me := v.script.vramOrg
	ent := int16(0)
	for {
		vram := uint32(v.r32(v.atent + uint32(int32(ent))))
		if vram != 0 && v.mem[vram-0x24]&4 == 0 {
			xd := int32(v.r16(vram)) - int32(v.r16(me))
			yd := int32(v.r16(vram+8)) - int32(v.r16(me+8))
			zd := int32(v.r16(vram+0x10)) - int32(v.r16(me+0x10))
			if uint32(zd*zd+yd*yd+xd*xd) < norme {
				if v.mem[vram-4] == 0 {
					v.mem[vram-4] = 1
					v.sendmess(vram, 0x8001)
					v.sendmess(vram, v.ru16(me-0x0e))
				}
			} else if v.mem[vram-4] != 0 {
				v.mem[vram-4] = 0
				v.sendmess(vram, 0x8000)
				v.sendmess(vram, v.ru16(me-0x0e))
			}
		}
		ent = v.r16(v.atent + 4 + uint32(int32(ent)))
		if ent == 0 {
			break
		}
	}
}

func casleepon(v *VM)  { v.mem[v.script.vramOrg-0x24] &= 0xfb }
func casleepoff(v *VM) { v.mem[v.script.vramOrg-0x24] |= 4 }

func crunfilm(v *VM) {
	v.readexecOpername()
	v.film.batchframes = int16(v.varD7)
	v.runfilm()
	v.varD7 = int32(v.film.frame)
	v.cstoreContinue()
	v.varD7 = int32(v.film.result)
	v.cstoreContinue()
}

func (v *VM) putval(d int16) {
	v.valtostr(v.sd7, d)
	for p := v.sd7; v.mem[p] != 0; p++ {
		v.putChar(int8(v.mem[p]))
	}
}

func cvpicprint(v *VM) {
	v.charmode = 1
	v.readexecOpernameSaveD7()
	v.putval(int16(v.varD7))
}

func cspicprint(v *VM) {
	v.charmode = 1
	v.readexecOpernameSwap()
	v.putString()
}

func cvputprint(v *VM) {
	v.charmode = 2
	v.readexecOpernameSaveD7()
	v.putval(int16(v.varD7))
}

func csputprint(v *VM) {
	v.charmode = 2
	v.readexecOpernameSwap()
	v.putString()
}
