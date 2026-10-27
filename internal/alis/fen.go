//go:build ignore

package alis

// Projection, depth sort and redraw windows (alis image.c, "calculate what
// to draw and where").

// screen record fields by byte offset
func (v *VM) sc8(scene uint16, off uint32) int8 { return int8(v.mem[v.sca(scene)+off]) }
func (v *VM) sc16(scene uint16, off uint32) int16 {
	return v.r16(v.sca(scene) + off)
}
func (v *VM) setSc8(scene uint16, off uint32, x int8)   { v.mem[v.sca(scene)+off] = uint8(x) }
func (v *VM) setSc16(scene uint16, off uint32, x int16) { v.w16(v.sca(scene)+off, x) }

func (v *VM) inilink(elem uint16) int16 {
	img := &v.img
	img.blocx1, img.blocy1 = 0x7fff, 0x7fff
	img.blocx2, img.blocy2 = -0x8000, -0x8000
	return v.spr(elem).clinking()
}

func (v *VM) calcfen(e1, e3 uint16) bool {
	img := &v.img
	if img.joints == 0 {
		s := v.spr(e1)
		if s.newd() < 0 {
			return false
		}
		img.blocx1 = s.newx()
		img.blocy1 = s.newy()
		img.blocx2 = img.blocx1 + s.width()
		img.blocy2 = img.blocy1 + s.height()
	}
	s3 := v.spr(e3)
	tx, ty, tw, th := s3.newx(), s3.newy(), s3.depx(), s3.depy()
	if img.blocx1 <= tw && img.blocy1 <= th && tx <= img.blocx2 && ty <= img.blocy2 {
		img.fenx1 = max(img.blocx1, tx)
		img.feny1 = max(img.blocy1, ty)
		img.fenx2 = min(img.blocx2, tw)
		img.feny2 = min(img.blocy2, th)
		// TODO(START.EXE): the DOS original aligns the bloc before clamping.
		ax1 := img.fenx1 &^ 15
		ax2 := img.fenx2 | 15
		if ax1 < 0 {
			ax1 = 0
		}
		if ax2 > gameWidth-1 {
			ax2 = gameWidth - 1
		}
		img.fenx1, img.fenx2 = ax1, ax2
		img.fenlargw = int16(uint16(img.fenx2-img.fenx1+1) >> 2)
		return true
	}
	return false
}

func (v *VM) clipfen(s sprite) bool {
	img := &v.img
	img.fclip = 0
	x1, y1, x2, y2 := s.newx(), s.newy(), s.depx(), s.depy()
	if img.fenx2 < x1 || img.feny2 < y1 || x2 < img.fenx1 || y2 < img.feny1 {
		return false
	}
	img.clipx1 = max(x1, img.fenx1)
	img.clipy1 = max(y1, img.feny1)
	img.clipx2 = min(x2, img.fenx2)
	img.clipy2 = min(y2, img.feny2)
	img.clipl = img.clipx2 - img.clipx1 + 1
	img.cliph = img.clipy2 - img.clipy1 + 1
	img.fclip = 1
	return true
}

func (v *VM) rangesprite(e1, e2, e3 uint16) uint16 {
	img := &v.img
	s1 := v.spr(e1)
	d1, ord1, num1 := s1.newd(), int8(s1.cordspr()), s1.numelem()
	if img.wback != 0 && e1 != uint16(img.backsprite) && img.backprof <= d1 {
		img.pback = 1
	}
	s3 := v.spr(e3)
	for s3.link() != 0 {
		l := v.spr(s3.link())
		ld, lo := l.newd(), int8(l.cordspr())
		if l.state() > -1 && ld <= d1 && (ld < d1 || (lo <= ord1 && (lo < ord1 ||
			(l.numelem() <= num1 && (l.numelem() < num1 || s1.state() == -1))))) {
			break
		}
		e3 = s3.link()
		s3 = l
	}
	s1.setState(0)
	s1.setLink(s3.link())
	s3.setLink(e1)
	if e2 == e3 {
		return e1
	}
	return e2
}

func (v *VM) tstjoints(elem uint16) {
	img := &v.img
	s := v.spr(elem)
	tx := s.newx() + s.width()
	ty := s.newy() + s.height()
	if s.newd() > -1 && img.newd > -1 && s.newx() <= img.newx+img.newl && s.newy() <= img.newy+img.newh &&
		img.newx <= tx && img.newy <= ty {
		img.blocx1 = min(s.newx(), img.newx)
		img.blocy1 = min(s.newy(), img.newy)
		tx = max(tx, img.newx+img.newl)
		ty = max(ty, img.newy+img.newh)
		img.blocx2, img.blocy2 = tx, ty
		img.joints = 1
		return
	}
	img.joints = 0
}

func (v *VM) scalaire(scene uint16, x, y, z *int16) {
	m := func(o uint32) int32 { return int32(v.sc8(scene, o)) }
	if v.sc8(scene, 1) < 0 {
		X, Y, Z := int32(*x), int32(*y), int32(*z)
		px := m(0x20)*X + m(0x21)*Y + m(0x22)*Z
		py := m(0x23)*X + m(0x24)*Y + m(0x25)*Z
		pz := m(0x26)*X + m(0x27)*Y + m(0x28)*Z
		*x, *y, *z = int16(px), int16(pz), int16(py)
	} else {
		py := *y
		*y = *y*int16(v.sc8(scene, 0x24)) - *z
		*z = py
	}
}

func (v *VM) depscreen(scene, elem uint16) {
	v.setSc16(scene, 0x16, v.sc16(scene, 0x16)+v.sc16(scene, 0x2a))
	v.setSc16(scene, 0x18, v.sc16(scene, 0x18)+v.sc16(scene, 0x2c))
	v.setSc16(scene, 0x1a, v.sc16(scene, 0x1a)+v.sc16(scene, 0x2e))
	for e := elem; e != 0; {
		s := v.spr(e)
		if int8(s.chsprite()) > -1 && s.state() == 0 {
			s.setState(2)
		}
		e = s.link()
	}
}

func (v *VM) deptopix(scene, elem uint16) {
	img := &v.img
	if v.sc8(scene, 1)&2 != 0 {
		v.landtopix(v.sca(scene), elem)
		return
	}
	s := v.spr(elem)
	scene = uint16(s.screenID())
	img.newf = int16(s.flaginvx())

	dx := s.depx() - v.sc16(scene, 0x16)
	dy := s.depy() - v.sc16(scene, 0x18)
	dz := s.depz() - v.sc16(scene, 0x1a)
	v.scalaire(scene, &dx, &dy, &dz)

	var offset int16
	cred := v.sc8(scene, 0x1d)
	if cred > -1 && s.credonOff() > -1 {
		if dz == 0 {
			dz = 1
		}
		dx = int16((int32(dx) << (uint8(cred) & 0x3f)) / int32(dz))
		dy = int16((int32(dy) << (uint8(cred) & 0x3f)) / int32(dz))
		offset = (dz >> (uint8(s.credonOff()) & 0x3f)) + int16(int8(s.creducing()))
		if offset < 0 {
			offset = 0
		} else {
			if clink := int16(v.sc8(scene, 0x1e)); clink < offset {
				offset = clink
			}
			offset <<= 2
		}
	}
	sh := uint8(v.sc8(scene, 0x1c)) & 0x3f
	dx = (dx >> sh) + v.sc16(scene, 0x0a)
	dy = (dy >> sh) + v.sc16(scene, 0x0c)

	img.newad = s.data() + uint32(int32(offset))
	sd := img.newad + uint32(v.r32(img.newad))
	if v.mem[sd] == 3 {
		if s.flaginvx() != 0 {
			dx -= v.r16(sd + 4)
		} else {
			dx += v.r16(sd + 4)
		}
		dy += v.r16(sd + 6)
		if v.mem[sd+1] != 0 {
			img.newf ^= 1
		}
		img.newad += uint32(int32(v.r16(sd+2) << 2))
		sd = img.newad + uint32(v.r32(img.newad))
	}
	img.newl = v.r16(sd + 2)
	img.newh = v.r16(sd + 4)
	img.newzoomx, img.newzoomy = 0, 0
	img.newx = dx - img.newl>>1
	img.newy = dy - img.newh>>1
	img.newd = dz
}

func (v *VM) clrfen() {
	img := &v.img
	fx1, fy1 := max(img.fenx1, 0), max(img.feny1, 0)
	fx2, fy2 := min(img.fenx2, gameWidth-1), min(img.feny2, gameHeight-1)
	w := int(fx2-fx1) + 1
	if w <= 0 || fy2 < fy1 {
		return
	}
	for y := int(fy1); y <= int(fy2); y++ {
		clear(img.logic[y*gameWidth+int(fx1) : y*gameWidth+int(fx1)+w])
	}
}

func (v *VM) clipback() {
	img := &v.img
	if img.clipy1 < img.backy1 {
		if img.clipy2 < img.backy1 {
			return
		}
	} else {
		if img.clipy2 <= img.backy2 {
			img.cback = 1
			return
		}
		if img.backy2 < img.clipy1 {
			return
		}
	}
	img.cback = -1
}

// trsfen copies the fen window from src to dst.
func (v *VM) trsfen(src, dst []byte) {
	img := &v.img
	fx1, fy1 := max(img.fenx1, 0), max(img.feny1, 0)
	fx2, fy2 := min(img.fenx2, gameWidth-1), min(img.feny2, gameHeight-1)
	if fx2 < fx1 || fy2 < fy1 {
		return
	}
	for y := int(fy1); y <= int(fy2); y++ {
		o := y*gameWidth + int(fx1)
		copy(dst[o:o+int(fx2-fx1)+1], src[o:])
	}
}

func (v *VM) phytolog() {
	img := &v.img
	img.fenx1, img.feny1, img.fenx2, img.feny2 = 0, 0, gameWidth-1, gameHeight-1
	v.trsfen(img.physic, img.logic)
}

func (v *VM) tvtofen() { v.trsfen(v.img.physic, v.img.logic) }

func (v *VM) fentotv() { v.trsfen(v.img.logic, v.img.physic) }

func (v *VM) memfen() {
	img := &v.img
	if img.ptabfen+4 > len(img.tabfen) {
		return // TODO(START.EXE): overflow is unchecked in the original
	}
	img.tabfen[img.ptabfen] = img.fenx1
	img.tabfen[img.ptabfen+1] = img.fenx2
	img.tabfen[img.ptabfen+2] = img.feny1
	img.tabfen[img.ptabfen+3] = img.feny2
	img.ptabfen += 4
}

func (v *VM) oldfen() {
	img := &v.img
	for i := 0; i < img.ptabfen; i += 4 {
		img.fenx1, img.fenx2 = img.tabfen[i], img.tabfen[i+1]
		img.feny1, img.feny2 = img.tabfen[i+2], img.tabfen[i+3]
		v.tvtofen()
	}
}

// folscreen follows the camera entity: a moved or turned camera marks the
// scene for a full redraw.
func (v *VM) folscreen(scene uint16) {
	sc := v.sca(scene)
	if v.r16(sc+0x60) < 0 {
		return
	}
	addr := uint32(v.r32(v.atent + uint32(v.ru16(sc+0x60))))
	if addr == 0 {
		return
	}
	upd := func(off uint32, val int16) {
		if val != v.r16(sc+off) {
			v.w16(sc+off, val)
			v.mem[sc] |= 0x80
		}
	}
	if v.mem[sc+0x84]&1 != 0 {
		upd(0x16, v.r16(sc+0x86)+v.r16(addr+scrWCX))
		upd(0x18, v.r16(sc+0x88)+v.r16(addr+scrWCY))
		upd(0x1a, v.r16(sc+0x8a)+v.r16(addr+scrWCZ))
	}
	if v.mem[sc+0x84]&2 != 0 {
		upd(0x34, v.r16(sc+0x8c)+v.r16(addr+0x40)+v.r16(addr+scrWCAX))
		upd(0x36, v.r16(sc+0x8e)+v.r16(addr+0x44)+v.r16(addr+scrWCAY))
		upd(0x38, v.r16(sc+0x90)+v.r16(addr+0x48)+v.r16(addr+scrWCAZ))
	}
}

func (v *VM) addlink(elem uint16) {
	img := &v.img
	s := v.spr(elem)
	if s.newd() > -1 {
		t := s.newx()
		if t <= img.blocx1 {
			img.blocx1 = t
		}
		t += s.width()
		if img.blocx2 <= t {
			img.blocx2 = t
		}
		t = s.newy()
		if t <= img.blocy1 {
			img.blocy1 = t
		}
		t += s.height()
		if img.blocy2 <= t {
			img.blocy2 = t
		}
	}
}

// storenew copies the projected image fields into the sprite.
func (v *VM) storenew(s sprite) {
	img := &v.img
	s.setNewad(img.newad)
	s.setNewx(img.newx)
	s.setNewy(img.newy)
	s.setNewd(img.newd)
	s.setNewf(int8(img.newf))
	s.setWidth(img.newl)
	s.setHeight(img.newh)
	s.setNewzoomx(uint16(img.newzoomx))
	s.setNewzoomy(uint16(img.newzoomy))
}

func (v *VM) inouvlink(scene, e1, e2, e3 uint16) uint16 {
	v.deptopix(scene, e1)
	s1 := v.spr(e1)
	v.storenew(s1)
	v.spr(e2).setLink(s1.link())
	v.addlink(e1)
	return v.rangesprite(e1, e2, e3)
}

func (v *VM) iremplink(scene, e1, e2, e3 uint16) uint16 {
	img := &v.img
	v.addlink(e1)
	s1 := v.spr(e1)
	if img.wback != 0 && img.backprof <= s1.newd() {
		img.pback = 1
		return v.inouvlink(scene, e1, e2, e3)
	}
	v.deptopix(scene, e1)
	v.storenew(s1)
	v.spr(e2).setLink(s1.link())
	v.addlink(e1)
	return v.rangesprite(e1, e2, e3)
}

func (v *VM) iefflink(e1, e2 uint16) uint16 {
	img := &v.img
	v.addlink(e1)
	s1 := v.spr(e1)
	v.spr(e2).setLink(s1.link())
	s1.setToNext(img.libsprit)
	img.libsprit = e1
	if img.wback != 0 && img.backprof <= s1.newd() {
		img.pback = 1
	}
	return e2
}

func (v *VM) setWlogic(back bool) {
	img := &v.img
	img.wlogicBack = back
	if back {
		img.wlogx1, img.wlogx2, img.wlogy1, img.wlogy2 = img.backx1, img.backx2, img.backy1, img.backy2
		img.wloglarg = img.backlarg
	} else {
		img.wlogx1, img.wlogx2, img.wlogy1, img.wlogy2 = img.logx1, img.logx2, img.logy1, img.logy2
		img.wloglarg = img.loglarg
	}
}

func (v *VM) drawable(s sprite) bool { return s.state() > -1 && s.newf() > -1 && s.newd() > -1 }

func (v *VM) fenetre(scene, e1, e3, prevsp uint16) {
	img := &v.img
	if v.sc8(scene, 0)&0x40 != 0 {
		return
	}
	img.scdirect = 0
	if !v.calcfen(e1, e3) {
		return
	}
	if v.sc8(scene, 1)&2 != 0 {
		img.fdoland = 1
	}
	for sc := v.scr.ptscreen; sc != 0; sc = v.scrToNext(sc) {
		if v.sc8(sc, 0)&0x40 != 0 {
			continue
		}
		tmp := v.scrScreenID(sc)
		if tmp == 0 {
			continue
		}
		if !v.clipfen(v.spr(tmp)) {
			continue
		}
		flags := v.sc8(sc, 1)
		if flags&2 != 0 {
			v.landtofi(prevsp, sc)
			continue
		}
		img.cback = 0
		if flags&4 != 0 {
			v.clipback()
		}
		if flags&0x40 == 0 {
			v.clrfen()
		}
		skipToBack := false
		if img.cback != 0 {
			if img.cback < 0 {
				if img.wback != 0 && img.pback != 0 {
					c1, c2 := img.clipy1, img.clipy2
					v.setWlogic(true)
					if img.clipy1 <= img.backy1 {
						img.clipy1 = img.backy1
					}
					if img.backy2 <= img.clipy2 {
						img.clipy2 = img.backy2
					}
					for t := v.spr(tmp).link(); t != 0 && t != uint16(img.backsprite); t = v.spr(t).link() {
						if s := v.spr(t); v.drawable(s) {
							v.destofen(s)
						}
					}
					v.setWlogic(false)
					img.clipy1, img.clipy2 = c1, c2
				}
			} else {
				if img.pback == 0 {
					tmp = uint16(img.backsprite)
					skipToBack = true
				} else {
					v.setWlogic(true)
				}
			}
		}
		if skipToBack {
			if s := v.spr(tmp); v.drawable(s) {
				v.destofen(s)
				img.switchgo = 1
			}
		}
		for t := v.spr(tmp).link(); t != 0; t = v.spr(t).link() {
			if img.sback != 0 && t == uint16(img.backsprite) {
				v.setWlogic(false)
			}
			if s := v.spr(t); v.drawable(s) {
				v.destofen(s)
				img.switchgo = 1
			}
		}
	}
	if v.sc8(scene, 1)&0x20 == 0 {
		if v.fswitch != 0 {
			if img.wpag == 0 {
				v.memfen()
			}
			return
		}
		if img.wpag == 0 {
			v.fentotv()
		}
	}
}

func (v *VM) suitlin1(a2, d1, d2, d3, d4 uint16) uint16 {
	for {
		d2 = d1
		d1 = v.spr(d2).link()
		if d1 == 0 {
			break
		}
		s := v.spr(d1)
		if d4 == uint16(s.clinking()) {
			switch st := s.state(); {
			case st == 0:
			case st < 0:
				d1 = v.inouvlink(a2, d1, d2, d3)
			case st == 2:
				d1 = v.iremplink(a2, d1, d2, d3)
			default:
				d1 = v.iefflink(d1, d2)
			}
		}
	}
	v.img.joints = 1
	return d1
}

// drawScreens draws every visible scene's sprites clipped to the scene's
// own window, starting at scene list entry first.
func (v *VM) drawScreens(first uint16) {
	img := &v.img
	for sc := first; sc != 0; sc = v.scrToNext(sc) {
		p := v.scrScreenID(sc)
		if v.sc8(sc, 0)&0x40 != 0 || p == 0 {
			continue
		}
		if !v.clipfen(v.spr(p)) {
			continue
		}
		for p = v.spr(p).link(); p != 0; p = v.spr(p).link() {
			if s := v.spr(p); v.drawable(s) {
				v.destofen(s)
				img.switchgo = 1
			}
		}
	}
}

func (v *VM) affiscr(scene, screenidx uint16) {
	img := &v.img
	sp := screenidx
	var prev uint16
	if v.mem[v.sca(scene)+0x84] != 0 {
		v.folscreen(scene)
	}
	if img.fremap != 0 || v.sc8(scene, 0) < 0 {
		v.depscreen(scene, screenidx)
	}
	flags := v.sc8(scene, 1)
	draw := true
	if flags&2 != 0 {
		v.affiland(scene)
		draw = v.sc8(scene, 0)&-0x80 == 0
	}
	if draw {
		img.wback = 0
		if flags&4 != 0 {
			img.wback = 1
		}
		img.wpag = 0
		if v.sc8(scene, 0)&0x20 != 0 {
			img.wpag = 1
			img.spag--
			if img.spag == 0 {
				img.wpag = -1
			}
		}
		if flags&0x10 == 0 {
			for {
				prev = sp
				sp = v.spr(sp).link()
				if sp == 0 {
					break
				}
				st := v.spr(sp).state()
				if st == 0 {
					continue
				}
				img.joints = 0
				img.pback = 0
				if st == 2 {
					// TODO(START.EXE): alis keeps clinking in a u16, so the
					// sign test is always true; the original tests it.
					link := uint16(v.inilink(sp))
					if true {
						sp = v.iremplink(scene, sp, prev, screenidx)
						v.suitlin1(scene, sp, prev, screenidx, link)
					}
				} else {
					link := uint16(v.inilink(sp))
					if st == -1 {
						sp = v.inouvlink(scene, sp, prev, screenidx)
					} else {
						sp = v.iefflink(sp, prev)
					}
					v.suitlin1(scene, sp, prev, screenidx, link)
				}
				v.fenetre(scene, sp, screenidx, prev)
				sp = prev
			}
		} else {
			img.fenx1 = v.sc16(scene, 0x0e)
			img.feny1 = v.sc16(scene, 0x10)
			img.fenx2 = img.fenx1 + v.sc16(scene, 0x12)
			img.feny2 = img.feny1 + v.sc16(scene, 0x14)
			img.clipl = img.fenx2 - img.fenx1 + 1
			img.cliph = img.feny2 - img.feny1 + 1
			img.clipx1, img.clipy1, img.clipx2, img.clipy2 = img.fenx1, img.feny1, img.fenx2, img.feny2
			if v.sc8(scene, 1)&0x40 == 0 {
				v.clrfen()
			}
			for {
				prev = sp
				sp = v.spr(sp).link()
				if sp == 0 {
					break
				}
				ps, cs := v.spr(prev), v.spr(sp)
				if cs.state() == 0 {
					continue
				}
				if cs.state() == 1 {
					ps.setLink(cs.link())
					cs.setToNext(img.libsprit)
					img.libsprit = sp
					sp = prev
				} else {
					v.deptopix(scene, sp)
					ps.setLink(cs.link())
					v.storenew(cs)
					sp = v.rangesprite(sp, prev, screenidx)
				}
			}
			v.drawScreens(v.scr.ptscreen)
			if v.fswitch == 0 && img.wpag == 0 {
				v.fentotv()
			}
		}
	} else {
		v.drawScreens(v.scrToNext(scene))
		if v.fswitch == 0 && img.wpag == 0 {
			v.fentotv()
		}
	}
	if img.wpag < 0 {
		v.setSc8(scene, 0, v.sc8(scene, 0)&^0x20)
		img.fenx1 = v.sc16(scene, 0x0e)
		img.feny1 = v.sc16(scene, 0x10)
		img.fenx2 = img.fenx1 + v.sc16(scene, 0x12)
		img.feny2 = img.feny1 + v.sc16(scene, 0x14)
	}
	v.setSc8(scene, 0, v.sc8(scene, 0)&0x7f)
}
