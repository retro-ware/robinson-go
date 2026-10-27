package alis

import "strings"

// Expression operands (alis opernames.c, v3 paths).

func (v *VM) popSd6() {
	src := v.acc
	v.strcpy(v.sd6, src)
	l := v.strlen(src) + 1
	if l%2 == 1 {
		src++
	}
	v.acc = src + l
}

func (v *VM) popSd7() {
	src := v.acc
	v.strcpy(v.sd7, src)
	l := v.strlen(src) + 1
	if l%2 == 1 {
		src++
	}
	v.acc = src + l
}

func (v *VM) pushSd7() {
	tgt := v.acc
	l := v.strlen(v.sd7) + 1
	if l%2 == 1 {
		tgt--
		v.mem[tgt] = 0
	}
	tgt -= l
	v.strcpy(tgt, v.sd7)
	v.acc = tgt
}

func (v *VM) vramAt(off int32) uint32 { return uint32(int32(v.script.vramOrg) + off) }

func oimmb(v *VM) { v.varD7 = int32(int8(v.read8())) }
func oimmw(v *VM) { v.varD7 = int32(v.read16()) }
func oimml(v *VM) { v.varD7 = v.read32() }
func oimmp(v *VM) { v.readUntilZero(v.sd7) }

func olocb(v *VM) { v.varD7 = int32(int8(v.r8(v.vramAt(int32(v.read16()))))) }
func olocw(v *VM) { v.varD7 = int32(v.r16(v.vramAt(int32(v.read16())))) }
func olocp(v *VM) { v.strcpy(v.sd7, v.vramAt(int32(v.read16()))) }

func oloctp(v *VM) {
	addr := v.tabstring(v.vramAt(int32(v.read16())))
	v.strcpy(v.sd7, uint32(addr))
}

func olocto(v *VM) {
	v.varD7 = v.tabstring(v.vramAt(int32(v.read16())))
}

func oloctc(v *VM) { v.varD7 = int32(int8(v.r8(uint32(v.tabchar(v.vramAt(int32(v.read16()))))))) }
func olocti(v *VM) { v.varD7 = int32(v.r16(uint32(v.tabint(v.vramAt(int32(v.read16())))))) }

func odirb(v *VM) { v.varD7 = int32(int8(v.r8(v.script.vramOrg + uint32(v.read8())))) }
func odirw(v *VM) { v.varD7 = int32(v.r16(v.script.vramOrg + uint32(v.read8()))) }
func odirp(v *VM) { v.strcpy(v.sd7, v.script.vramOrg+uint32(v.read8())) }

func odirtp(v *VM) {
	addr := v.tabstring(v.script.vramOrg + uint32(v.read8()))
	v.strcpy(v.sd7, uint32(addr))
}

func odirtc(v *VM) {
	v.varD7 = int32(int8(v.r8(uint32(v.tabchar(v.script.vramOrg + uint32(v.read8()))))))
}
func odirti(v *VM) { v.varD7 = int32(v.r16(uint32(v.tabint(v.script.vramOrg + uint32(v.read8()))))) }

func (v *VM) mainAt(off int16) uint32 { return uint32(int32(v.basemain) + int32(off)) }

func omainb(v *VM) { v.varD7 = int32(int8(v.r8(v.mainAt(v.read16())))) }
func omainw(v *VM) { v.varD7 = int32(v.r16(v.mainAt(v.read16()))) }
func omainp(v *VM) { v.strcpy(v.sd7, v.mainAt(v.read16())) }

func omaintp(v *VM) {
	addr := v.tabstring(v.mainAt(v.read16()))
	v.strcpy(v.sd7, uint32(addr))
}

func omaintc(v *VM) { v.varD7 = int32(int8(v.r8(uint32(v.tabchar(v.mainAt(v.read16())))))) }
func omainti(v *VM) { v.varD7 = int32(v.r16(uint32(v.tabint(v.mainAt(v.read16()))))) }

// The him* names reach another entity's variables: a word variable of this
// script holds the entity index (an offset into the entity table), and the
// entity's vram base is read from there (START.EXE 0x9ebf..0x9fc2).

// himEnt reads the entity-variable operand and returns the entity's vram.
// The byte and word forms check the index first: a negative one shows the
// engine's error screen with code err.
func (v *VM) himEnt(off int16, err int) int32 {
	ent := int32(v.r16(v.vramAt(int32(off))))
	if err != 0 && ent < 0 {
		v.alisError(err)
	}
	return v.r32(uint32(int32(v.atent) + ent))
}

// himTab returns the vram of the entity for the table forms, whose operands
// are the table offset first and the entity variable second.
func (v *VM) himTab() uint32 {
	off := v.read16()
	return uint32(v.himEnt(v.read16(), 0) + int32(off))
}

// The byte and word forms leave D7 alone when the entity is dead (vram <= 0).
func ohimb(v *VM) {
	vram := v.himEnt(v.read16(), 0x1b)
	off := v.read16()
	if vram > 0 {
		v.varD7 = int32(int8(v.r8(uint32(vram + int32(off)))))
	}
}

func ohimw(v *VM) {
	vram := v.himEnt(v.read16(), 0x1d)
	off := v.read16()
	if vram > 0 {
		v.varD7 = int32(v.r16(uint32(vram + int32(off))))
	}
}

func ohimp(v *VM) {
	vram := v.himEnt(v.read16(), 0)
	v.strcpy(v.sd7, uint32(vram+int32(v.read16())))
}

func ohimtp(v *VM) { v.strcpy(v.sd7, uint32(v.tabstring(v.himTab()))) }
func ohimtc(v *VM) { v.varD7 = int32(int8(v.r8(uint32(v.tabchar(v.himTab()))))) }
func ohimti(v *VM) { v.varD7 = int32(v.r16(uint32(v.tabint(v.himTab())))) }

func opile(v *VM) {
	v.varD7 = v.varD6
	v.varD6 = v.pop()
}

func oeval(v *VM) {
	v.oevalLoop = true
	for v.oevalLoop && v.state != stateStopped {
		v.readexecOpername()
	}
}

func ofin(v *VM) { v.oevalLoop = false }

func opushacc(v *VM) { v.push(v.varD7) }

func oand(v *VM) {
	v.varD6 = v.varD7
	v.readexecOpername()
	v.varD7 &= v.varD6
}

func oor(v *VM) {
	v.varD6 = v.varD7
	v.readexecOpername()
	v.varD7 |= v.varD6
}

func oxor(v *VM) {
	v.readexecOpernameSaveD7()
	v.varD7 ^= v.varD6
}

func oeqv(v *VM) {
	v.readexecOpernameSaveD7()
	v.varD7 = ^(v.varD7 ^ v.varD6)
}

func boolD7(b bool) int32 {
	if b {
		return -1
	}
	return 0
}

func oegal(v *VM)  { v.readexecOpernameSaveD7(); v.varD7 = boolD7(v.varD6 == v.varD7) }
func odiff(v *VM)  { v.readexecOpernameSaveD7(); v.varD7 = boolD7(v.varD6 != v.varD7) }
func oinfeg(v *VM) { v.readexecOpernameSaveD7(); v.varD7 = boolD7(v.varD6 <= v.varD7) }
func osupeg(v *VM) { v.readexecOpernameSaveD7(); v.varD7 = boolD7(v.varD6 >= v.varD7) }
func oinf(v *VM)   { v.readexecOpernameSaveD7(); v.varD7 = boolD7(v.varD6 < v.varD7) }
func osup(v *VM)   { v.readexecOpernameSaveD7(); v.varD7 = boolD7(v.varD6 > v.varD7) }
func oadd(v *VM)   { v.readexecOpernameSaveD7(); v.varD7 += v.varD6 }

func osub(v *VM) {
	v.readexecOpernameSaveD7()
	v.varD6 -= v.varD7
	v.varD7 = v.varD6
}

func omod(v *VM) {
	v.readexecOpernameSaveD7()
	if v.varD7 == 0 {
		v.fatal("division by zero (omod)") // TODO: x86 idiv traps; check START.EXE
		return
	}
	v.varD6 %= v.varD7
	v.varD7 = v.varD6
}

func odiv(v *VM) {
	v.readexecOpernameSaveD7()
	if v.varD7 == 0 {
		v.fatal("division by zero (odiv)") // TODO: x86 idiv traps; check START.EXE
		return
	}
	v.varD6 /= v.varD7
	v.varD7 = v.varD6
}

func omul(v *VM) {
	v.readexecOpernameSaveD7()
	v.varD7 = int32(uint32(v.varD7) * uint32(v.varD6))
}

func oneg(v *VM) { v.varD7 = -v.varD7 }

func oabs(v *VM) {
	if v.varD7 < 0 {
		v.varD7 = -v.varD7
	}
}

func ornd(v *VM) {
	result := uint32(v.randomNumber)
	v.randomNumber = uint16(result*0x7ab7 - 0x77f)
	result = uint32(v.randomNumber)
	result = uint32(v.varD7&0xffff) * result
	v.varD7 = int32(result*0x10000 | result>>16)
}

func osgn(v *VM) {
	if v.varD7 > 0 {
		v.varD7 = 1
	} else if v.varD7 < 0 {
		v.varD7 = -1
	}
}

func onot(v *VM) { v.varD7 = ^v.varD7 }

// oinkey is the code of the key down, 0 when none (START.EXE 0xa39f).
func oinkey(v *VM) { v.varD7 = int32(v.kbd.last) }

func okeyon(v *VM) {}

// ojoy reads joystick 0 (START.EXE 0x253f); there is none.
func ojoy(v *VM) { v.varD7 = 0 }

func oprnd(v *VM) {
	result := uint32(v.pop())
	result = uint32(uint16(result<<10 | result>>6))
	result = uint32(uint16(result*0x7ab7 - 0x77f))
	result = uint32(v.varD7&0xffff) * result
	v.varD7 = int32(result*0x10000 | result>>16)
}

func oscan(v *VM) {
	vram := v.script.vramOrg
	newVal := v.r16(vram - 0x1e)
	if newVal == v.r16(vram-0x1c) {
		v.varD7 = -1
		return
	}
	result := v.r16(uint32(int32(vram) + int32(newVal)))
	newVal += 2
	if -0x3f < newVal {
		newVal -= v.r16(v.scriptOrg(vram) + 0x16)
	}
	v.w16(vram-0x1e, newVal)
	if newVal == v.r16(vram-0x1c) {
		v.w8(vram-0x24, v.r8(vram-0x24)&0x7f)
	}
	v.varD7 = int32(result)
}

// oshiftkey is the shift bits (START.EXE 0xa3ba).
func oshiftkey(v *VM) { v.varD7 = int32(v.kbd.mods) }

func (v *VM) ioDfree() int16 { return 32000 } // KB free for saves (capped as the original)

func ofree(v *VM) {
	switch d := v.varD7; {
	case d == 0: // 0x29fb
		v.varD7 = int32(int16((v.finmem() - v.finprog()) / 1000))
	case d == 1: // 0x2a15
		v.varD7 = int32(int16((v.ru32(dsDebsprit) - v.finent) / 1000))
	case d == 2:
		v.varD7 = int32(v.maxprog) - int32(v.nbprog)
	case d == 3:
		v.varD7 = int32(v.maxent) - int32(v.nbent)
	case d == 4: // 0x2a4d: the free sprites
		v.varD7 = 0
		for s := v.ru16(dsLibsprit); s != 0; s = v.fsu16(s, spToNext) {
			v.varD7++
		}
	case 0x40 < d:
		if d < 0x49 {
			v.varD7 = int32(v.ioDfree())
		}
		if 0x60 < v.varD7 && v.varD7 < 0x69 {
			v.varD7 = int32(v.ioDfree())
		}
	default:
		v.varD7 = -1
	}
}

func omodel(v *VM) { v.varD7 = 2000 }

func ogetkey(v *VM) { v.varD7 = int32(v.getkey()) }

func oleft(v *VM) {
	v.popSd7()
	v.mem[v.sd7+uint32(v.varD7&0xff)] = 0
}

// oright keeps the last D7 characters of the popped string (START.EXE 0xa421).
func oright(v *VM) {
	v.popSd7()
	src := v.sd7 + uint32(v.strlen(v.sd7)) - uint32(int32(int16(v.varD7)))
	if src <= v.sd7 {
		src = v.sd7
	}
	for dst := v.sd7; ; src, dst = src+1, dst+1 {
		v.mem[dst] = v.mem[src]
		if v.mem[dst] == 0 {
			break
		}
	}
}

func omid(v *VM) {
	src := uint32(int32(v.sd7) + v.pop())
	v.popSd7()
	tgt := v.sd7
	v.varD7--
	for ; v.varD7 > -1; v.varD7, src, tgt = v.varD7-1, src+1, tgt+1 {
		v.mem[tgt] = v.mem[src]
		if v.mem[tgt] == 0 {
			break
		}
	}
	if v.varD7 < 0 {
		v.mem[tgt] = 0
	}
}

func olen(v *VM) { v.varD7 = int32(v.strlen(v.sd7)) }
func oasc(v *VM) { v.varD7 = int32(int8(v.mem[v.sd7])) }
func ostr(v *VM) { v.valtostr(v.sd7, int16(v.varD7)) }

func osadd(v *VM) {
	v.readexecOpernameSwap()
	v.strcat(v.sd6, v.sd7)
	v.sd6, v.sd7 = v.sd7, v.sd6
}

func osegal(v *VM)  { v.readexecOpernameSwap(); v.varD7 = boolD7(v.strncmpA(v.sd6, v.sd7) == 0) }
func osdiff(v *VM)  { v.readexecOpernameSwap(); v.varD7 = boolD7(v.strncmpA(v.sd6, v.sd7) != 0) }
func osinfeg(v *VM) { v.readexecOpernameSwap(); v.varD7 = boolD7(v.strncmpA(v.sd6, v.sd7) <= 0) }
func ossupeg(v *VM) { v.readexecOpernameSwap(); v.varD7 = boolD7(v.strncmpA(v.sd6, v.sd7) >= 0) }
func osinf(v *VM)   { v.readexecOpernameSwap(); v.varD7 = boolD7(v.strncmpA(v.sd6, v.sd7) < 0) }
func ossup(v *VM)   { v.readexecOpernameSwap(); v.varD7 = boolD7(v.strncmpA(v.sd6, v.sd7) > 0) }

func ospushacc(v *VM) { v.pushSd7() }

func ospile(v *VM) {
	v.sd6, v.sd7 = v.sd7, v.sd6
	v.popSd6()
}

func oval(v *VM) {
	var result int16
	flip := false
	s := v.sd7
	if v.mem[s] != 0 {
		flip = v.mem[s] == 0x2d
		if flip {
			s++
		}
		if v.mem[s] == 0x24 {
			for {
				s++
				c2 := int8(v.mem[s])
				if c2 == 0 || c2-0x30 < 0 {
					break
				}
				var c1 int8
				if c2 < 0x3a {
					c1 = c2 - 0x30
				} else {
					c2 &= -0x21 // 0xdf
					// TODO: alis tests c2+0xbf < 0 as an int (never true); check START.EXE
					if c2+(-0x41) < 0 || 0x46 < c2 {
						break
					}
					c1 = c2 - 0x37
				}
				result = result*0x10 + int16(c1)
			}
		} else {
			for {
				c2 := int8(v.mem[s])
				if c2 == 0 || c2-0x30 < 0 || 0x39 < c2 {
					break
				}
				result = result*10 + int16(c2-0x30)
				s++
			}
		}
	}
	if flip {
		result = -result
	}
	v.varD7 = int32(result)
}

func oexistf(v *VM) {
	name := v.cstr(v.sd7)
	if dot := strings.LastIndexByte(name, '.'); dot >= 0 {
		ext := strings.ToLower(name[dot+1:])
		if ext != "fic" && ext != "cst" && ext != "dat" && !v.fs.Exists(name) {
			name = name[:dot+1] + "io"
		}
	}
	v.varD7 = boolD7(name != "" && v.fs.Exists(name))
}

func ochr(v *VM) {
	v.mem[v.sd7] = uint8(v.varD7)
	v.mem[v.sd7+1] = 0
}

func ochange(v *VM)  { v.varD7 = 0 }
func ocountry(v *VM) { v.varD7 = int32(v.pays) }
func omip(v *VM)     { v.varD7 = 0x64 }

// ojoykey is 0 (START.EXE 0xa3dd).
func ojoykey(v *VM) { v.varD7 = 0 }

func oconfig(v *VM) { v.varD7 = int32(v.theconfig) }

func pnul(v *VM) { v.fatal(v.script.name + ": pnul operand") }

func init() {
	for i, f := range []opfn{
		oimmb, oimmw, oimmp, olocb, olocw, olocp, oloctp, oloctc, olocti,
		odirb, odirw, odirp, odirtp, odirtc, odirti,
		omainb, omainw, omainp, omaintp, omaintc, omainti,
		ohimb, ohimw, ohimp, ohimtp, ohimtc, ohimti,
		opile, oeval, ofin, oimml, olocto, opushacc,
		oand, oor, oxor, oeqv, oegal, odiff, oinfeg, osupeg, oinf, osup,
		oadd, osub, omod, odiv, omul, oneg, oabs, ornd, osgn, onot,
		oinkey, okeyon, ojoy, oprnd, oscan, oshiftkey, ofree, omodel, ogetkey,
		oleft, oright, omid, olen, oasc, ostr, osadd, osegal, osdiff, osinfeg,
		ossupeg, osinf, ossup, ospushacc, ospile, oval, oexistf, ochr, ochange,
		ocountry, omip, ojoykey, oconfig,
	} {
		opernames[i*2] = f
	}
}
