package alis

// RRQ landscape: per-frame setup, sky, billboards and hit testing (alis
// render/rrq3d.c affiland, skytofen, landtopix, zoomtofen, pointpix and
// rrq3d_dos.c calclan0, clrvga, vgatofen, landtofi).

func (v *VM) render3dInit() {}

func (v *VM) calctop(sc uint32, col, row int16) int32 {
	return (int32(row) + int32(col)*int32(v.r16(sc-0x3ca))) * 2
}

func (v *VM) glandtopix(rc uint32, x, y, ox, oy, depth int16) (int16, int16) {
	s, c, f := int32(v.r16(rc-0x28c)), int32(v.r16(rc-0x28a)), int32(v.r16(rc-0x3aa))
	nx := int32(int16((int32(y)*s+int32(x)*c)>>9)) * f
	ny := int32(int16((int32(oy)*s+int32(ox)*c)>>9)) * f
	d := v.r16(rc-0x3ac) + depth
	if d <= 0 {
		d = 1
	}
	return int16(nx / int32(d)), int16(ny / int32(d))
}

func (v *VM) vgatofen() {
	img := &v.img
	img.switchgo = 1
	src := img.wlogicAddr + uint32(int32(img.clipx1)+int32(img.clipy1-img.wlogy1)*int32(img.wloglarg))
	w := int32(img.clipx2-img.clipx1) + 1
	img.vgamodulo = img.wloglarg - int16(w)
	img.bitmodulo = img.loglarg*2 - int16(w)
	t := int32(img.clipx1) + int32(img.clipy1)*int32(img.loglarg)*2
	for y := img.clipy1; y <= img.clipy2; y++ {
		copy(img.logic[t:t+w], v.mem[src:src+uint32(w)])
		t += w + int32(img.bitmodulo)
		src += uint32(int32(w) + int32(img.vgamodulo))
	}
}

func (v *VM) clrvga() {
	img := &v.img
	width := uint16(img.fenx2-img.fenx1) + 1
	if int32(img.fenx1)+int32(width) > int32(img.wloglarg) {
		width = uint16(img.wloglarg - img.fenx1)
	}
	w4 := uint32(width>>2) << 2
	mod := uint32(uint16(img.wloglarg)) - w4
	p := img.wlogicAddr + uint32(int32(img.fenx1)) + uint32(uint16(img.feny1-img.wlogy1))*uint32(uint16(img.wloglarg))
	for y := img.feny2 - img.feny1; y >= 0; y-- {
		clear(v.mem[p : p+w4])
		p += w4 + mod
	}
}

func (v *VM) calclan0(sc, rc uint32) {
	img := &v.img
	r16 := func(o uint32) int16 { return v.r16(rc - o) }
	rd32 := func(o uint32) int32 { return v.r32(rc - o) }
	gs := uint32(v.ru16(rc-0x3c0)) & 0x3f
	fix := func(num, den int16) int32 {
		frac := int16((int32(uint32(uint16(num%den))<<16) >> 1) / int32(den))
		return int32(uint32(uint16(num/den))*0x10000 + uint32(int32(frac)*2))
	}
	abs16 := func(x int16) int16 {
		if x < 0 {
			return -x
		}
		return x
	}
	img.signedx = r16(0x378) - r16(0x37e)
	img.signedy = r16(0x376) - r16(0x37c)
	adx, ady := abs16(img.signedx), abs16(img.signedy)
	// major and minor axis offsets: Y dominant steps along Y
	if adx < ady {
		v.w16(rc-0x2b0, ady)
		v.w16(rc-0x2ac, abs16(r16(0x36a)-r16(0x370)))
		v.w32(rc-0x2c4, fix(r16(0x378)-r16(0x37e), ady))
		sign := int32(0x10000)
		if img.signedy < 0 {
			sign = -0x10000
		}
		v.w32(rc-0x2c0, sign)
		cc := int16(int32(r16(0x37c)-r16(0x370)) * int32(int16(rd32(0x2c4)>>2)) >> 14)
		if rd32(0x2c0) < 0 {
			cc = -cc
		}
		v.w16(rc-0x24e, abs16(r16(0x372)+cc-r16(0x37e))>>gs)
	} else {
		v.w16(rc-0x2b0, adx)
		v.w16(rc-0x2ac, abs16(r16(0x36c)-r16(0x372)))
		v.w32(rc-0x2c0, fix(r16(0x376)-r16(0x37c), adx))
		sign := int32(0x10000)
		if img.signedx < 0 {
			sign = -0x10000
		}
		v.w32(rc-0x2c4, sign)
		cc := int16(int32(r16(0x37e)-r16(0x372)) * int32(int16(rd32(0x2c0)>>2)) >> 14)
		if rd32(0x2c4) < 0 {
			cc = -cc
		}
		v.w16(rc-0x24e, abs16(r16(0x370)+cc-r16(0x37c))>>gs)
	}
	n := r16(0x24e)
	v.w32(rc-0x2bc, fix(r16(0x372)-r16(0x37e), n)>>gs)
	v.w32(rc-0x2b8, fix(r16(0x370)-r16(0x37c), n)>>gs)
	sx, sy := int32(0x10000), int32(0x10000)
	if rd32(700) < 0 {
		sx = -0x10000
	}
	if rd32(0x2b8) < 0 {
		sy = -0x10000
	}
	v.w32(rc-0x2a0, sx)
	v.w32(rc-0x29c, sy)
	v.w32(rc-0x2b4, ((int32(r16(0x2ac)-r16(0x2b0))<<8)/int32(n)<<8)>>gs)

	v.w32(rc-0x252, 0)
	v.wu32(rc-0x256, uint32(v.ru16(sc+0x9e))<<16)
	if v.r16(sc+0x98) != 0 {
		v.w32(rc-0x256, 0x60000)
		fog := v.ru16(sc + 0x9e)
		if int16(fog) < 7 {
			v.w32(rc-0x256, 0)
		}
		if v.r16(sc+0x98) > -1 {
			v.wu32(rc-0x256, uint32(v.ru16(sc+0xa0))*0x10000-1)
		}
		v.w32(rc-0x252, (int32(uint32(fog)*0x10000-uint32(rd32(0x256)))>>8)/int32(r16(0x24e)+1)<<8)
	}
	n = r16(0x24e)
	v.w32(rc-0x2e4, (rd32(0x318)-rd32(0x330))/int32(n))
	v.w32(rc-0x2dc, ((rd32(0x2f0)-rd32(0x2fc))*0x100/int32(n))<<8)
	v.w32(rc-0x2d4, (rd32(0x310)-rd32(0x328))/int32(n))
	v.w32(rc-0x2cc, ((rd32(0x2ec)-rd32(0x2f8))*0x100/int32(n))<<8)
	v.w32(rc-0x2e8, rd32(0x330))
	v.w32(rc-0x2e0, rd32(0x32c)<<16)
	v.w32(rc-0x2d8, rd32(0x328))
	v.w32(rc-0x2d0, rd32(0x2f8))
}

func (v *VM) skyfast(t uint32, fill uint8, stride, height int16) uint32 {
	img := &v.img
	for l := int16(0); l < height; l++ {
		for i := uint32(0); i < uint32(uint16(img.clipl)); i++ {
			v.mem[t+i] = fill
		}
		t += uint32(int32(img.clipl + stride))
	}
	return t
}

func (v *VM) skytofen(sc, rc uint32) {
	img := &v.img
	if img.ftstpix != 0 {
		return
	}
	res := v.ru32(sc + 0xa4)
	data := v.ru32(res) + res
	var hotY int16
	bm := data
	if v.mem[data] == 3 {
		res = uint32(int32(int16(v.ru16(data+2)*4)) + int32(res))
		bm = v.ru32(res) + res
		hotY = v.r16(data + 6)
	}
	row0 := uint32(int32(img.clipx1)) + v.ru32(img.atlpix+uint32(int32(int16((img.clipy1-img.wlogy1)*4))))
	if v.mem[bm] == 1 {
		c := v.mem[bm+1] & 0xf
		v.skyfast(row0, c|c<<4, img.wloglarg-img.clipl, img.cliph-1)
		return
	}
	bh := v.r16(bm + 4)
	img.skyposy = hotY + v.r16(sc+0xc) + v.r16(rc-0x352) - int16(uint16(bh+1)>>1)
	tw := v.r16(bm+2) + 1
	mod := (int16(uint32(int32(v.r16(sc+0x38))*int32(v.r16(sc+0xa8)))>>8) - img.clipx1) % tw
	pos := uint16(-mod)
	if mod > 0 {
		pos = uint16(tw) + pos
	}
	img.skyposx = int16(pos & 0xfffe)
	img.skyleft = tw - img.skyposx
	if img.clipl < tw-img.skyposx {
		img.skyleft = img.clipl
	}
	dst := row0
	gap := img.wloglarg - img.clipl
	const doff = 8
	if top := img.skyposy - img.clipy1; top != 0 && img.clipy1 <= img.skyposy {
		top = min(top, img.cliph)
		dst = v.skyfast(dst, v.mem[bm+doff], gap, top-1)
	}
	skyTop := max(img.skyposy, img.clipy1)
	skyBot := bh + img.skyposy
	if img.clipy2 < skyBot {
		skyBot = img.clipy2
	}
	if rows := skyBot - skyTop; rows != 0 && skyTop <= skyBot {
		stride := v.ru16(bm+2) + 1
		var srcOff uint32
		if d := uint16(img.clipy1 - img.skyposy); d != 0 && img.skyposy <= img.clipy1 {
			srcOff = uint32(d) * uint32(stride)
		}
		rowSrc := bm + uint32(int32(img.skyposx)) + srcOff + doff
		half := img.skyleft>>1 - 1
		for {
			srcEnd, src, d := rowSrc, rowSrc, dst
			if half > -1 {
				for n := half; n != -1; n-- {
					srcEnd = src + 2
					dst = d + 2
					v.w16(d, v.r16(src))
					src, d = srcEnd, dst
				}
			}
			wrap := img.clipl>>1 - half
			if wrap > 1 {
				srcEnd = uint32(int32(srcEnd) - int32(int16(stride)))
				src = dst
				for n := wrap - 2; n != -1; n-- {
					dst = src + 2
					v.w16(src, v.r16(srcEnd))
					srcEnd += 2
					src = dst
				}
			}
			if img.clipl&1 != 0 {
				dst++
			}
			dst = uint32(int32(gap) + int32(dst))
			rowSrc = uint32(int32(int16(stride)) + int32(rowSrc))
			rows--
			if rows == -1 {
				break
			}
		}
	}
	if bot := img.clipy2 - img.skyposy - bh; bot != 0 && bh <= img.clipy2-img.skyposy {
		bot = min(bot, img.cliph)
		v.skyfast(dst, v.mem[bm+uint32(uint16(v.r16(bm+2)+1))*uint32(bh)+doff], gap, bot-1)
	}
}

func (v *VM) landtopix(sc uint32, id uint16) {
	img := &v.img
	s := v.spr(id)
	img.newad = s.data()
	if int8(s.creducing()) > 1 {
		ang := int16(s.sprite0x28() >> 16)
		norm := uint16(ang - v.r16(sc+0x38) + 900)
		for int16(norm) > -1 {
			norm -= 0x168
		}
		cr := uint32(uint16(int16(int8(s.creducing()))))
		fi := (cr*uint32(uint16(norm+0x168)) + 0xb4) / 0x168
		off := int16(fi)
		if int8(s.creducing()) <= int8(fi) {
			off = 0
		}
		img.newad += uint32(int32(off) * 4)
	}
	hdr := v.ru32(img.newad) + img.newad
	bm, meta := hdr, img.zoombid
	if v.mem[hdr] == 3 {
		img.newad += uint32(int32(v.r16(hdr+2)) * 4)
		bm = v.ru32(img.newad) + img.newad
		meta = hdr
	}
	var zs uint16
	img.newf = int16(v.mem[meta+1] ^ s.flaginvx())
	if s.credonOff() == -0x80 {
		img.newzoomy = int16(s.newzoomy())
		img.newx = v.r16(sc+0xa) + s.depx()
		img.newy = v.r16(sc+0xc) - s.depz()
		img.newd = s.depy()
	} else {
		r := v.clandtopix(sc, s.depx()-v.r16(sc+0x16), s.depy()-v.r16(sc+0x18), s.depz()-v.r16(sc+0x1a))
		img.newzoomy = r.z
		img.newd = r.depth
		img.newx = v.r16(sc+0xa) + r.sx
		img.newy = v.r16(sc+0xc) + r.sy
		zs = uint16(int16(s.credonOff()))
	}
	if int8(zs) >= 8 {
		img.newzoomy = int16(uint16(uint32(int32(img.newzoomy)) * uint32(zs-8) >> 3))
		zs = 3
	}
	if s.credonOff() == -0x80 {
		img.newzoomx = int16(s.newzoomx())
	} else {
		img.newzoomx = img.newzoomy
	}
	if bm > 0xdfffff {
		img.newad = 0
		bm = img.landdata
		img.newd = -1000
		s.setState(1)
	}
	zs = (zs + 1) & 0x3f
	hz, wz := uint16(img.newzoomx), uint16(img.newzoomy)
	sh := max(((int32(v.r16(bm+4)+1)*int32(hz)*2>>zs)+0xff)>>8-1, 0)
	sw := max(((int32(v.r16(bm+2)+1)*int32(wz)*2>>zs)+0xff)>>8-1, 0)
	img.newh, img.newl = int16(sh), int16(sw)
	img.newy += int16(int32(int16(v.ru16(bm+4)>>1-v.ru16(meta+6))) * int32(int16(hz)) * -2 >> (zs + 8))
	img.newx += int16(int32(int16(v.ru16(bm+2)>>1-v.ru16(meta+4))) * int32(int16(wz)) * -2 >> (zs + 8))
}

func (v *VM) spritland(sc uint32, head uint16) {
	img := &v.img
	cur := head
	for {
		prev := cur
		ps := v.spr(prev)
		cur = ps.link()
		if cur == 0 {
			return
		}
		s := v.spr(cur)
		switch s.state() {
		case 0:
		case 1:
			ps.setLink(s.link())
			s.setToNext(img.libsprit)
			img.libsprit = cur
			cur = prev
		default:
			v.landtopix(sc, cur)
			s.setNewad(img.newad)
			s.setNewx(img.newx)
			s.setNewy(img.newy)
			s.setNewd(img.newd)
			s.setNewf(int8(img.newf))
			s.setWidth(img.newl)
			s.setHeight(img.newh)
			s.setNewzoomx(uint16(img.newzoomx))
			s.setNewzoomy(uint16(img.newzoomy))
			ps.setLink(s.link())
			cur = v.rangesprite(cur, prev, head)
		}
	}
}

func (v *VM) spritaff(depth int16) {
	img := &v.img
	idx := uint16(img.spritnext)
	for {
		if idx == 0 {
			img.spritnext = 0
			img.spritprof = 0x8000
			return
		}
		s := v.spr(idx)
		if s.newd() <= depth {
			break
		}
		if s.state() > -1 && s.newf() > -1 && s.newd() > -1 {
			v.zoomtofen(s)
		}
		idx = s.link()
	}
	img.spritprof = int32(v.spr(idx).newd())
	img.spritnext = int32(idx)
}

func (v *VM) barsprite(rc uint32, ti, wx, wy int16) {
	img := &v.img
	s := v.spr(uint16(img.atexsprite))
	tix := rc + uint32(int32(ti))
	te := v.ru32(tix + 0x10)
	if v.ru32(te) == 0 {
		return
	}
	gs := v.ru16(rc-0x3c0) & 0x3f
	r := v.clandtopix(img.mapscreen, wx<<gs-v.r16(rc-0x268), wy<<gs-v.r16(rc-0x266), img.solh-v.r16(rc-0x264))
	d := r.depth
	if d < v.r16(img.mapscreen+0x92) {
		d = -1
	}
	s.setNewx(v.r16(rc-0x26c) + r.sx)
	s.setNewy(v.r16(rc-0x26a) + r.sy)
	s.setNewd(d)
	s.setNewzoomx(uint16(r.z))
	s.setNewzoomy(uint16(r.z))
	if d < 0 {
		return
	}
	s.setScriptEnt(v.ru16(tix + 0x1c))
	s.setCredonOff(int8(v.mem[tix+0x15]))
	base := v.ru32(te) + te
	meta, bm := img.zoombid, base
	if int8(v.mem[bm]) == 3 {
		te += uint32(int32(v.r16(bm+2)) * 4)
		bm = v.ru32(te) + te
		meta = base
	}
	s.setNewad(te)
	s.setNewf(int8(v.mem[meta+1]))
	lvl := uint8(s.credonOff())
	if v.mem[bm+1]&0x20 == 0 {
		lvl++
	}
	sh := uint32(lvl) & 0x3f
	zy, zx := int32(int16(s.newzoomy())), int32(int16(s.newzoomx()))
	hgt := int32(uint32(int32(int16(v.r16(bm+4)+1))*zy)>>8*2)>>sh - 1
	wid := int32(uint32(int32(int16(v.r16(bm+2)+1))*zx)>>8*2)>>sh - 1
	s.setWidth(int16(max(wid, 0)))
	s.setHeight(int16(max(hgt, 0)))
	s.setNewy(int16(int32(int16(v.ru16(bm+4)>>1-v.ru16(meta+6)))*zy>>8*-2>>sh) + s.newy())
	s.setNewx(int16(int32(int16(v.ru16(bm+2)>>1-v.ru16(meta+4)))*zx>>8*-2>>sh) + s.newx())
	v.zoomtofen(s)
}

func concat13(a uint8, b uint32) uint32 { return uint32(a)<<24 | b&0xffffff }

func (v *VM) zoomtofen(s sprite) {
	img := &v.img
	bm := s.newad() + v.ru32(s.newad())
	f := v.mem[bm]
	if f != 0x14 && f != 0x18 && f != 0x1a && f != 0x1c && f != 0x1e {
		return
	}
	doff := uint32(8)
	if f == 0x18 || f == 0x1a {
		doff = 6
	}
	cw := s.width() + 1
	ch := uint16(s.height() + 1)
	scale := (uint32(v.ru16(bm+4)+1) << 8) / uint32(ch)
	zi := uint32(int32(int16(scale)))
	zf := (zi << 8) >> 16
	ystep := zi<<24 | zf

	sy := s.newy()
	ye := s.newy() - img.clipy1 - img.cliph
	if (int32(img.cliph) <= int32(uint16(s.newy()-img.clipy1)) && ye != 0) || -int16(ch)-ye < 0 {
		if img.clipy2 < s.newy() {
			return
		}
		ye = s.newy() + int16(ch) - 1
		if ye < img.clipy1 {
			return
		}
		if s.newy() < img.clipy1 {
			ch = uint16(s.newy() + int16(ch) - img.clipy1)
			sy = img.clipy1
		}
		if img.clipy2 < ye {
			ch = uint16(img.clipy2 + int16(ch) - ye)
		}
	}
	xo := s.newx()
	xe := xo - img.clipx1 - img.clipl
	sx := xo
	if (int32(img.clipl) <= int32(uint16(xo-img.clipx1)) && xe != 0) || -cw-xe < 0 {
		if img.clipx2 < xo {
			return
		}
		xe = xo + cw - 1
		if xe < img.clipx1 {
			return
		}
		if xo < img.clipx1 {
			cw = xo + cw - img.clipx1
			sx = img.clipx1
		}
		if img.clipx2 < xe {
			cw = img.clipx2 + cw - xe
		}
	}
	bw := v.r16(bm+2) + 1
	xstep := uint32(uint16((zi<<24)>>16))<<16 | uint32(uint16(int32(bw)*int32(int8(scale>>8))))

	if img.ftstpix == 0 {
		v.zoomRender(s, bm, doff, cw, ch, ystep, zi, zf, xstep, sx, sy, xo, bw)
		return
	}
	if sy <= img.ytstpix && img.ytstpix < int16(ch)+sy && sx <= img.xtstpix && img.xtstpix < cw+sx {
		k := zf<<8&0xffff | zi&0xff
		tx := uint32(uint16(img.xtstpix-xo)) * k >> 8
		ty := int32((uint32(uint16(img.ytstpix-s.newy())) * k >> 8 & 0xffff) * uint32(v.ru16(bm+2)+1))
		if s.newf() != 0 {
			ty = int32(int16(v.ru16(bm+2)+1)) + ty - 1
			tx = -tx
		}
		if v.mem[bm+tx+uint32(ty)+8] != 0 && v.r32(uint32(s.scriptEnt())+v.atent) != 0 {
			img.cxtstpix = s.depx()
			img.cytstpix = s.depy()
			img.cztstpix = s.depz()
			img.etstpix = int16(s.scriptEnt())
			img.antstpix = int32(s.newad())
			img.dtstpix = s.newd()
		}
	}
}

func (v *VM) zoomRender(s sprite, bm, doff uint32, cw int16, ch uint16, ystep, zi, zf, xstep uint32, sx, sy, xo, bw int16) {
	img := &v.img
	fb := uint32(int32(sx)) + uint32(uint16(sy-img.wlogy1))*uint32(uint16(img.wloglarg)) + img.wlogicAddr
	img.ztflowy &= 0xffff
	yoff := uint16(sy - s.newy())
	ty := uint32(yoff)
	k := (zf&0xff)<<8 | zi&0xff
	if yoff != 0 {
		ty = uint32(yoff) * k
		img.ztflowy = concat13(uint8(ty), img.ztflowy)
		ty = (ty >> 8 & 0xffff) * uint32(v.ru16(bm+2)+1)
	}
	skip := uint32(int32(img.wloglarg - cw))
	if s.newf()&1 != 0 {
		row := int32(v.ru16(bm+2)) + int32(ty)
		img.ztflowx = 0xffff0000 | img.ztflowx&0xffff
		if d := uint16(sx - xo); d != 0 {
			zf = uint32(d) * k
			img.ztflowx = concat13(uint8(-1-int8(zf)), img.ztflowx)
			row -= int32(zf >> 8)
		}
		zf = img.ztflowy
		for y := uint16(0); y < ch; y, fb = y+1, fb+skip {
			zy := img.ztflowx&0xffff0000 | zf&0xffff
			for x := int16(0); x < cw; x++ {
				idx := int32(int16(zy)) + row + int32(doff)
				if idx < 0 {
					idx += 0x10000
				}
				if p := v.mem[bm+uint32(idx)]; p != 0 {
					v.mem[fb] = p
				}
				fb++
				d := zy - ystep
				b := uint32(0)
				if zy < ystep {
					b = 1
				}
				zy = d&0xffff0000 | uint32(uint16(d)-uint16(b))
			}
			c := carry(xstep, zf)
			zf += xstep
			if c != 0 {
				zf = zf&0xffff0000 | uint32(uint16(bw)+uint16(zf))
			}
		}
		return
	}
	img.ztflowx &= 0xffff
	if d := uint16(sx - xo); d != 0 {
		zf = uint32(d) * k
		img.ztflowx = concat13(uint8(zf), img.ztflowx)
		ty += zf >> 8
	}
	zf = img.ztflowy
	for y := uint16(0); y < ch; y, fb = y+1, fb+skip {
		zy := img.ztflowx&0xffff0000 | zf&0xffff
		for x := int16(0); x < cw; x++ {
			if p := v.mem[bm+uint32(uint16(zy))+ty+doff]; p != 0 {
				v.mem[fb] = p
			}
			fb++
			c := carry(ystep, zy)
			sum := ystep + zy
			zy = sum&0xffff0000 | uint32(uint16(sum)+uint16(c))
		}
		c := carry(xstep, zf)
		zf += xstep
		if c != 0 {
			zf = zf&0xffff0000 | uint32(uint16(bw)+uint16(zf))
		}
	}
}

func (v *VM) affiland(scene uint16) {
	img := &v.img
	sc := v.basemain + uint32(int32(int16(scene)))
	img.mapscreen = sc
	img.wloglarg = 0x140
	img.wlogicAddr = v.ru32(v.ru32(sc + 0x30))
	img.wlogx1 = v.r16(sc + 0xe)
	img.wlogy1 = v.r16(sc + 0x10)
	rc := v.landRC(sc)

	img.fenx1 = v.r16(sc + 0xe)
	img.landclipy1 = v.r16(sc + 0x10)
	img.landclipx2 = v.r16(sc+0x12) + img.fenx1
	img.feny2 = v.r16(sc+0x14) + img.landclipy1
	img.clipl = img.landclipx2 - img.fenx1 + 1
	img.landcliph = img.feny2 - img.landclipy1 + 1
	img.vbarclipx2 = img.landclipx2 + 1
	img.feny1 = img.landclipy1
	img.fenx2 = img.landclipx2
	img.clipx1, img.clipy1 = img.fenx1, img.landclipy1
	img.clipx2, img.clipy2 = img.landclipx2, img.feny2
	img.cliph = img.landcliph

	if img.ftstpix == 0 {
		if v.mem[sc]&0x80 == 0 {
			if v.fswitch != 0 && img.switchland != 0 {
				img.fenlargw = img.wloglarg >> 2
				img.wloglarg >>= 1
				v.tvtofen()
				img.switchland = 0
			}
		} else {
			v.calctoy(sc, rc)
			v.spritland(sc, v.ru16(sc+2))
			v.calclan0(sc, rc)
			if v.mem[sc+1]&0x40 == 0 && v.r16(sc+0xa2) == 0 {
				v.clrvga()
			}
			v.doland(sc, rc)
			v.vgatofen()
			img.landone = 1
			img.switchland = 1
		}
	} else {
		v.calclan0(sc, rc)
		v.doland(sc, rc)
	}
	v.setWlogic(false)
	img.fenlargw = img.loglarg >> 2
}

func (v *VM) landtofi(_ uint16, scene uint16) {
	img := &v.img
	sc := v.basemain + uint32(int32(int16(scene)))
	saveH, saveY1 := img.cliph, img.clipy1
	img.wloglarg = 0x140
	img.wlogicAddr = v.ru32(v.ru32(sc + 0x30))
	img.wlogx1 = v.r16(sc + 0xe)
	img.landclipx2 = img.wlogx1 + v.r16(sc+0x12)
	img.clipy1 = v.r16(sc + 0x10)
	img.cliph = img.clipy2 - img.clipy1 + 1
	img.wlogy1 = img.clipy1
	img.mapscreen = sc
	img.landclipy1 = img.clipy1
	img.landcliph = img.cliph
	img.fdoland = 0
	if v.r16(sc+0x12)+1 == img.clipl && v.ru16(sc+0x14)+1 == uint16(img.cliph) {
		img.fdoland = 1
	}
	rc := v.landRC(sc)
	v.calclan0(sc, rc)
	if v.mem[sc+1]&0x40 == 0 && v.r16(sc+0xa2) == 0 {
		v.clrvga()
	}
	v.doland(sc, rc)
	img.fdoland = 0
	img.clipy1, img.cliph = saveY1, saveH
	v.vgatofen()
	img.wlogicBack = false
	img.wlogx1, img.wlogy1 = img.logx1, img.logy1
	img.wloglarg = img.loglarg
}

func (v *VM) pixtoland(rc uint32) {
	img := &v.img
	r16 := func(o uint32) int16 { return v.r16(rc - o) }
	cosY, sinY, sinP := int32(r16(0x28a)), int32(r16(0x28c)), int32(r16(0x290))
	d := int32(img.dtstpix)
	dx := img.xtstpix - r16(0x26c)
	dy := r16(0x26a) - img.ytstpix
	ax := int32(r16(0x3ac) + int16(d))
	ux := int16(int32(int16(int32(dx)*cosY>>9)) * ax / int32(r16(0x3aa)))
	img.cxtstpix = r16(0x268) + (ux - int16(d*sinY>>9))
	uy := int16(int32(int16(int32(dx)*sinY>>9)) * ax / int32(r16(0x3aa)))
	img.cytstpix = r16(0x266) + int16(d*cosY>>9) + uy
	ay := int32(r16(0x3a8) + int16(d))
	uz := int16(int32(dy) * ay / int32(r16(0x3a6)))
	img.cztstpix = r16(0x264) + int16(d*sinP>>9) + uz
}

func (v *VM) pointpix(x, y int16) {
	img := &v.img
	id := uint32(v.ru16(v.script.vramOrg - 0x16))
	sc := v.basemain + id
	img.dtstpix = v.r16(sc + 0x94)
	img.etstpix, img.ntstpix = -1, -1
	img.ftstpix = 1
	img.xtstpix, img.ytstpix = x, y
	if v.mem[sc]&0x40 == 0 {
		v.affiland(uint16(id))
		if img.etstpix > -1 {
			e := v.ru32(v.atent + uint32(int32(img.etstpix)))
			hdr := v.ru32(e - 0x14)
			ed := v.ru32(hdr+0xe) + hdr
			img.ntstpix = int16((uint32(img.antstpix)&0xffffff - (v.ru32(ed) + ed)) >> 2)
			if v.r16(ed+4) <= img.ntstpix {
				b := v.basemain + v.ru32(v.basemain+0xe)
				img.ntstpix = int16((uint32(img.antstpix)&0xffffff - (v.ru32(b) + b)) >> 2)
			}
		}
	}
	v.pixtoland(v.landRC(sc))
	img.ftstpix = 0
}
