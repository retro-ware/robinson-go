//go:build ignore

package alis

// Sprite pool and element lists (alis image.c, "Sprite management").

func (v *VM) inisprit() {
	img := &v.img
	img.debsprit = 0
	img.finsprit = 0xffff
	img.tvsprite = 0x8000
	img.basesprite = img.debsprit + 0x8000
	m := img.spritemem
	putle32(m[0x0c:], uint32(int32(img.logx1)))
	putle16(m[0x16:], uint16(img.logx2))
	putle16(m[0x18:], uint16(img.logy2))
	m[0x29] = uint8(img.logy1)
	img.backsprite = 0x000e
	img.texsprite = 0x0042
	putle32(m[0x5c:], 0)
	m[0x50] = 0xfe
	v.mousflag = 0
	img.atexsprite = 0x78
	img.libsprit = 0xa8
	cur := int32(0xa8)
	for ; cur+sprSz < img.finsprit; cur += sprSz {
		v.spr(uint16(cur)).setToNext(uint16(cur + sprSz))
	}
	v.spr(uint16(cur)).setToNext(0)
}

func putle16(b []byte, x uint16) { b[0], b[1] = byte(x), byte(x>>8) }
func putle32(b []byte, x uint32) {
	b[0], b[1], b[2], b[3] = byte(x), byte(x>>8), byte(x>>16), byte(x>>24)
}

func (v *VM) searchelem(cur, prev *uint16) bool {
	*cur = v.elemHead()
	sid := int16(v.curScreenID())
	for *cur != 0 {
		s := v.spr(*cur)
		if s.screenID() <= sid {
			if sid != s.screenID() {
				break
			}
			if s.numelem() <= v.img.numelem {
				return v.img.numelem == s.numelem()
			}
		}
		*prev = *cur
		*cur = s.toNext()
	}
	return false
}

func (v *VM) searchtete(cur, prev *uint16) bool {
	*prev = 0
	*cur = v.elemHead()
	sid := int16(v.curScreenID())
	for *cur != 0 {
		s := v.spr(*cur)
		if s.screenID() <= sid {
			return s.screenID() == sid
		}
		*prev = *cur
		*cur = s.toNext()
	}
	return false
}

func (v *VM) testnum(cur uint16) bool {
	if cur == 0 {
		return false
	}
	s := v.spr(cur)
	return s.screenID() == int16(v.curScreenID()) && s.numelem() == v.img.numelem
}

func (v *VM) nextnum(cur, prev *uint16) bool {
	*prev = *cur
	*cur = v.spr(*cur).toNext()
	return v.testnum(*cur)
}

func (v *VM) createlem(cur, prev *uint16) {
	img := &v.img
	if img.libsprit == 0 {
		return
	}
	sprit := img.libsprit
	s := v.spr(sprit)
	next := s.toNext()
	if *prev == 0 {
		v.setElemHead(sprit)
	} else {
		v.spr(*prev).setToNext(sprit)
	}
	img.libsprit = next

	scrn := v.scrScreenID(v.curScreenID())
	ss := v.spr(scrn)
	s.setState(-1)
	s.setNumelem(img.numelem)
	s.setScreenID(int16(v.curScreenID()))
	s.setUnknown0x30(0x8000)
	s.setUnknown0x32(0x8000)
	s.setToNext(*cur)
	s.setLink(ss.link())
	s.setPrev(scrn)
	if s.link() != 0 {
		v.spr(s.link()).setPrev(sprit)
	}
	ss.setLink(sprit)
	*cur = sprit
}

func (v *VM) delprec(elem uint16) {
	s := v.spr(elem)
	scr := uint16(s.screenID())
	if elem == v.scrScreenID(scr) {
		v.wu16(v.sca(scr)+scScreenID, s.link())
		return
	}
	idx := v.scrScreenID(scr)
	for idx != 0 && elem != v.spr(idx).link() {
		idx = v.spr(idx).link()
	}
	if idx != 0 {
		v.spr(idx).setLink(s.link())
		if s.link() != 0 {
			v.spr(s.link()).setPrev(idx)
		}
	}
}

func (v *VM) killelem(cur, prev *uint16) {
	s := v.spr(*cur)
	unlink := func() {
		if *prev == 0 {
			v.setElemHead(s.toNext())
		} else {
			v.spr(*prev).setToNext(s.toNext())
		}
	}
	if v.ferase == 0 && s.state() > -1 {
		s.setState(1)
		unlink()
	} else {
		unlink()
		s.setToNext(v.img.libsprit)
		v.img.libsprit = *cur
		v.delprec(*cur)
	}
	if *prev == 0 {
		*cur = v.elemHead()
		return
	}
	*cur = v.spr(*prev).toNext()
}

func (v *VM) getelem(cur, prev *uint16) {
	if v.searchelem(cur, prev) {
		for {
			if s := v.spr(*cur); s.state() == 0 {
				s.setState(2)
				return
			}
			if !v.nextnum(cur, prev) {
				break
			}
		}
	}
	v.createlem(cur, prev)
}

func (v *VM) put(idx uint16) {
	v.fmuldes = 0
	v.putin(idx)
}

func (v *VM) putfin() {
	if v.fmuldes == 0 {
		var cur, prev uint16
		if v.searchelem(&cur, &prev) {
			for {
				for v.spr(cur).state() == 0 {
					v.killelem(&cur, &prev)
					if cur == 0 || !v.testnum(cur) {
						v.fadddes = 0
						return
					}
				}
				if !v.nextnum(&cur, &prev) {
					break
				}
			}
		}
	}
	v.fadddes = 0
}

func (v *VM) putin(idx uint16) {
	img := &v.img
	if v.scrScreenID(v.curScreenID()) == 0 {
		return
	}
	x, y, z := img.depx, img.depy, img.depz

	addr := v.adresdes(int32(idx))
	addr += v.r32(uint32(addr))
	res := uint32(addr)
	if v.mem[res] <= 0x80 {
		var cur, prev uint16
		if v.fadddes == 0 && v.fmuldes == 0 {
			if !v.searchelem(&cur, &prev) {
				v.createlem(&cur, &prev)
			}
			if s := v.spr(cur); s.state() > -1 {
				s.setState(2)
			}
		} else {
			v.getelem(&cur, &prev)
		}
		v.putmapin(cur, v.adresdes(int32(idx)))
		return
	}
	if v.mem[res] == 0xfe {
		v.dosPutinPalette(res)
		return
	}

	count := int(v.mem[res+1])
	p := res + 2
	for range count {
		invx, muldes := img.invertX, v.fmuldes
		elem := v.r16(p)
		dx := v.r16(p + 2)
		if img.invertX != 0 {
			dx = -dx
		}
		img.depx += dx
		img.depy = v.r16(p+4) + y
		img.depz = v.r16(p+6) + z
		if elem < 0 {
			elem &= 0x7fff
			img.invertX ^= 1
		}
		v.fmuldes = 1
		v.putin(uint16(elem))
		img.depx, img.depy, img.depz = x, y, z
		img.invertX = invx
		v.fmuldes = muldes
		p += 8
	}
	v.fmuldes = 0
	v.putfin()
}

func (v *VM) putmapin(spridx uint16, bitmap int32) {
	img := &v.img
	vram := v.script.vramOrg
	s := v.spr(spridx)
	s.setData(uint32(bitmap))
	s.setNewad(0)
	s.setFlaginvx(img.invertX)

	s.setSprite0x28(uint32(v.mem[vram-0x28]))
	s.setScriptEnt(v.ru16(vram - 0x0e))
	s.setClinking(v.r16(vram - 0x2a))
	s.setCordspr(v.mem[vram-0x2b])
	s.setCreducing(v.mem[vram-0x27])
	s.setCredonOff(int8(v.mem[vram-0x25]))

	s.setChsprite(v.mem[vram-0x2f])
	if s.credonOff() == -0x80 {
		s.setNewzoomx(v.ru16(vram - 0x38))
		s.setNewzoomy(v.ru16(vram - 0x36))
		s.setCreducing(0)
	}
	if s.credonOff() >= 0 {
		s.setCreducing(v.mem[vram-0x27])
		s.setCredonOff(int8(v.mem[vram-0x26]))
		if s.credonOff() < 0 {
			s.setCreducing(0)
			s.setCredonOff(int8(v.mem[v.basemain+uint32(v.curScreenID())+0x1f]))
		}
	}
	s.setDepx(img.oldcx + img.depx)
	s.setDepy(img.oldcy + img.depy)
	s.setDepz(img.oldcz + img.depz)
	v.putfin()
}
