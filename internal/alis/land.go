//go:build ignore

package alis

// RRQ 3D landscape, shared part (alis render/rrq3d.c, little-endian/DOS
// paths). Scenes and their render context live in VM memory; rc is the
// render context address, the land's tables sit below it.

type land struct {
	fprectop, fprectopa, fbottom, notopa uint8
	prectopa, precbota                   uint16
	prectopb, precbotb                   uint16
	prectopc, precbotc                   uint16
	prectopi, precboti                   uint16
	botalt, bothigh, solha               uint16
	adresa                               uint32
	fillColor                            uint8
	savedSI                              int16
	bottomType                           int16
}

type lstl struct{ gridX, gridY, scaledX, scaledY int16 }

type cltp struct {
	camX, camY       int32
	altProj          uint32
	sx, sy, depth, z int16
}

func (v *VM) lenstoland(sc, rc uint32, x, y int16) lstl {
	yaw := int32(v.r16(sc + 0x38))
	c, s := int32(tabcos(yaw)), int32(tabsin(yaw))
	var r lstl
	r.gridX = v.r16(sc+0x16) + int16((int32(x)*c-int32(y)*s)>>9)
	r.gridY = v.r16(sc+0x18) + int16((int32(x)*s+int32(y)*c)>>9)
	sh := v.ru16(rc-0x3c0) & 0x3f
	r.scaledX = r.gridX >> sh
	r.scaledY = r.gridY >> sh
	return r
}

func perspDivisor(d int32) int32 {
	if d == 0 || d > 32767 || d < 0 && d >= -32768 {
		return 1
	}
	return d
}

func sat16(q int64) int32 {
	if q < -32768 {
		return -32768
	}
	if q > 32767 {
		return 32767
	}
	return int32(q)
}

func (v *VM) clandtopix(sc uint32, rx, ry, rz int16) cltp {
	a := int32(v.r16(sc + 0x38))
	c, s := int32(tabcos(a)), int32(tabsin(a))
	camX := (int32(rx)*c + int32(ry)*s) >> 9
	camY := (int32(ry)*c - int32(rx)*s) >> 9
	if int32(v.r16(sc+0x94)) < camY {
		return cltp{camY: -1, depth: -1, z: 0x100}
	}
	pitch := int32(v.r16(sc + 0x34))
	pc := (int32(int16(camY)) * int32(tabsin(-2*pitch))) >> 9
	alt := -(int32(rz) + pc)
	camX *= int32(v.r16(sc + 0x76))
	alt *= int32(v.r16(sc + 0x7a))
	xd := perspDivisor(int32(int16(camY)) + int32(v.r16(sc+0x70)))
	yd := perspDivisor(camY + int32(v.r16(sc+0x74)))
	return cltp{
		camX: camX, camY: camY, altProj: uint32(alt),
		sx:    int16(sat16(int64(camX) / int64(xd))),
		sy:    int16(sat16(int64(alt) / int64(yd))),
		depth: int16(camY),
		z:     int16(sat16(int64(int32(v.r16(sc+0x7a))<<8) / int64(yd))),
	}
}

func (v *VM) putLstl(rc uint32, g, s uint32, r lstl) {
	v.w16(rc-g, r.gridX)
	v.w16(rc-g+2, r.gridY)
	v.w16(rc-s, r.scaledX)
	v.w16(rc-s+2, r.scaledY)
}

func (v *VM) putCltp(rc uint32, cam, scr uint32, r cltp) {
	v.w32(rc-cam, r.camX)
	v.w32(rc-cam+4, r.camY)
	v.wu32(rc-cam+8, r.altProj)
	v.w16(rc-scr, r.sx)
	v.w16(rc-scr+2, r.sy)
	v.w16(rc-scr+4, r.depth)
}

func (v *VM) calctoy(sc, rc uint32) {
	r16 := func(a uint32) int32 { return int32(v.r16(a)) }
	v.w16(sc+0x94, int16(r16(rc-0x3f2)*2+r16(sc+0x6c)))
	v.w16(rc-0x26c, v.r16(sc+0xa))
	v.w16(rc-0x26a, v.r16(sc+0xc))
	v.w16(rc-0x268, v.r16(sc+0x16))
	v.w16(rc-0x266, v.r16(sc+0x18))
	v.w16(rc-0x264, v.r16(sc+0x1a))
	pitch, yaw := r16(sc+0x34), r16(sc+0x38)
	cp, sp := tabcos(pitch), tabsin(pitch)
	cy, sy := tabcos(yaw), tabsin(yaw)
	v.w16(rc-0x28e, cp)
	v.w16(rc-0x28a, cy)
	v.w16(rc-0x290, sp)
	v.w16(rc-0x28c, sy)
	v.w16(sc+0xaa, cp)
	v.w16(sc+0xac, sp)
	v.w16(sc+0xb2, cy)
	v.w16(sc+0xb4, sy)
	v.w16(rc-0x360, v.r16(sc+0x16))
	v.w16(rc-0x35e, v.r16(sc+0x18))
	v.w16(rc-0x35c, v.r16(sc+0x1a))

	f4 := r16(rc - 0x3f4)
	v.putLstl(rc, 0x37e, 0x3a2, v.lenstoland(sc, rc, int16((-f4-r16(sc+0x6a)>>1)-f4), v.r16(sc+0x6c)))
	v.putLstl(rc, 0x378, 0x39c, v.lenstoland(sc, rc, int16((r16(sc+0x6a)>>1-f4)-f4), v.r16(sc+0x6c)))
	v.putLstl(rc, 0x372, 0x396, v.lenstoland(sc, rc, int16((-f4-r16(sc+0x64)>>1)-f4), v.r16(sc+0x66)))
	v.putLstl(rc, 0x36c, 0x390, v.lenstoland(sc, rc, int16((r16(sc+0x64)>>1-f4)-f4), v.r16(sc+0x66)))
	v.putLstl(rc, 0x366, 0x38a, v.lenstoland(sc, rc, int16(-f4), v.r16(sc+0x6c)))
	v.putLstl(rc, 0x35a, 0x384, v.lenstoland(sc, rc, int16(-f4), v.r16(sc+0x66)))

	gs := v.ru16(rc-0x3c0) & 0x3f
	cx, cyy, cz := r16(sc+0x16), r16(sc+0x18), r16(sc+0x1a)
	corner := func(g uint32) cltp {
		return v.clandtopix(sc, int16(r16(rc-g)<<gs-cx), int16(r16(rc-g+2)<<gs-cyy), int16(-cz))
	}
	v.putCltp(rc, 0x330, 0x354, corner(0x3a2))
	v.putCltp(rc, 0x324, 0x34e, corner(0x39c))
	v.putCltp(rc, 0x318, 0x348, corner(0x396))
	v.putCltp(rc, 0x30c, 0x342, corner(0x390))
	v.putCltp(rc, 0x300, 0x33c, v.clandtopix(sc, int16(r16(rc-0x366)-cx), int16(r16(rc-0x364)-cyy), -0x100))
	v.putCltp(rc, 0x2f4, 0x336, v.clandtopix(sc, int16(r16(rc-0x35a)-cx), int16(r16(rc-0x358)-cyy), -0x100))
	v.w16(sc+0x94, int16(r16(sc+0x6c)-r16(rc-0x3f2)))
}

func (v *VM) altiland(sc, rc uint32) {
	adj := v.r16(rc - 0x3f4)
	v.w16(sc+0x6c, adj+v.r16(sc+0x6c))
	v.w16(sc+0x66, v.r16(sc+0x66)-adj)
	v.calctoy(sc, rc)
	adj = v.r16(rc - 0x3f4)
	v.w16(sc+0x6c, v.r16(sc+0x6c)-adj)
	v.w16(sc+0x66, adj+v.r16(sc+0x66))
	v.w32(rc-0x2c8, v.r32(rc-0x32c)<<16)

	const rows = 0xb3
	end, start := v.r32(rc-0x2ec), v.r32(rc-0x2f8)
	rowScale := start
	yfp := v.r32(rc-0x32c) << 16
	ystep := int32(int16((v.r32(rc-0x314)-v.r32(rc-0x32c))*0x100/rows)) - 1
	v.w16(rc-0x262, int16(ystep))
	p := v.img.atalti
	for range rows {
		div := int16(int32(v.r16(rc-0x3a8)) + int32(int16(uint32(yfp)>>16)))
		q := int16(rowScale / int32(div))
		rf := uint32(int32(q) * 0x100)
		v.w16(p, 20000)
		acc := rf
		p += 2
		for range 0xff {
			acc = acc<<16 | acc>>16
			ip := -int16(acc)
			v.w16(p, ip)
			acc = acc&0xffff0000 | uint32(uint16(-ip))
			acc = acc<<16 | acc>>16
			acc += rf
			p += 2
		}
		yfp += ystep * 0x100
		rowScale += int32(int16((end - start) / rows))
	}
	for range 0x100 {
		v.w16(p, 20000)
		p += 2
	}
	t := int16(int32(int16(int32(v.r16(sc+0x6c))*4)) * 0x168 / 0x140)
	v.img.fhorizon = int16(int32(int16(uint16(int32(t)+0x80)&0xff00)) * 0x140 / 0x168)
}

func (v *VM) iniland(sc, rc uint32) {
	img := &v.img
	img.purey2 = v.r16(sc+0x6c) - v.r16(sc+0x66)
	img.purey = img.purey2 >> 2
	img.purey2 >>= 4
	v.w16(rc-0x298, 0)
	v.w16(rc-0x296, 0)
	v.w16(rc-0x294, v.r16(rc-0x3e8))
	v.w16(rc-0x292, v.r16(rc-0x3e6))
	stride := uint32(v.ru16(rc-0x3ca)) << 1
	n := int32(v.r16(rc-0x3cc)-1) << 2
	p := rc
	if int8(v.mem[rc-1]) < 0 {
		p = v.ru32(v.ru32(rc))
	}
	for i := int32(0); i <= n; i, p = i+4, p+stride {
		v.wu32(img.atlland+uint32(i), p)
	}
	p = v.ru32(v.ru32(sc + 0x30))
	for i := uint32(0); i < 800; i, p = i+4, p+0x140 {
		v.wu32(img.atlpix+i, p)
	}
	ap := img.atalias
	for d := int32(1); d <= 0x40; d++ {
		for i := int32(0); i < 0x40; i, ap = i+1, ap+2 {
			v.w16(ap, int16(i*0x100/d))
		}
	}
}

func (v *VM) inilens(sc, rc uint32) {
	r16 := func(a uint32) int32 { return int32(v.r16(a)) }
	u16 := func(a uint32) uint32 { return uint32(v.ru16(a)) }
	v.w16(sc+0x94, int16(r16(sc+0x6c)-r16(rc-0x3f2)-r16(rc-0x3f2)))
	v.w16(sc+0x92, v.r16(sc+0x66)>>2)
	v.w16(sc+0x70, int16(int32(u16(sc+0x6c)*u16(sc+0x64)-u16(sc+0x66)*u16(sc+0x6a))/(r16(sc+0x6a)-r16(sc+0x64))))
	v.w16(sc+0x76, int16((r16(sc+0x70)+r16(sc+0x6c))*(r16(sc+0x12)+1)/r16(sc+0x6a)))
	v.w16(sc+0x74, int16(int32(u16(sc+0x6c)*u16(sc+0x68)-u16(sc+0x66)*u16(sc+0x6e))/(r16(sc+0x6e)-r16(sc+0x68))))
	v.w16(sc+0x7a, int16((r16(sc+0x74)+r16(sc+0x6c))*(r16(sc+0x14)+1)/r16(sc+0x6e)))
	v.w16(rc-0x3ac, v.r16(sc+0x70))
	v.w16(rc-0x3aa, v.r16(sc+0x76))
	v.w16(rc-0x3a8, v.r16(sc+0x74))
	v.w16(rc-0x3a6, v.r16(sc+0x7a))
	v.w16(rc-0x3a4, v.r16(sc+0x6a)/v.r16(rc-0x3f4))
}

// landRC is the render context of a scene: an entity's variables.
func (v *VM) landRC(sc uint32) uint32 {
	return uint32(int32(v.r16(sc+0x42)) + v.r32(v.atent+uint32(v.ru16(sc+0x40))))
}

func (v *VM) openland(id uint16) {
	img := &v.img
	sc := v.basemain + uint32(int32(int16(id)))
	rc := v.landRC(sc)
	if v.r32(sc+0x30) == 0 {
		buf := uint32(v.ioMalloc(int32(uint32(v.ru16(sc+0x14)+9) * 0x140)))
		v.wu32(sc+0x30, buf)
		v.wu32(buf, v.ru32(buf)+0x500)
	}
	if img.tlpix == 0 {
		n := uint32(v.ru16(rc - 0x3cc))
		img.tlpix = uint32(v.ioMalloc(int32(n*4 + 0x18b20)))
		base := v.ru32(img.tlpix)
		img.atalti = base
		img.atlpix = base + 0x16800
		img.atalias = img.atlpix + 0x320
		img.atlland = img.atalias + 0x2000
		img.tlland = img.tlpix
	}
	if img.tlland == 0 {
		img.tlland = uint32(v.ioMalloc(int32(v.r16(rc-0x3cc)) * 4))
		img.atlland = v.ru32(img.tlland)
	}
	v.w16(sc+0x1a, 0x14)
	v.w16(sc+0x34, 0)
	v.w16(sc+0x36, 0)
	v.w16(sc+0x38, 0)
	v.inilens(sc, rc)
	v.iniland(sc, rc)
	v.altiland(sc, rc)
	img.landone = 0
}

// landImage holds the image globals of the land renderer.
type landImage struct {
	wlogicAddr                        uint32
	tlpix, tlland                     uint32
	atalti, atlpix, atalias           uint32
	purey, purey2, fhorizon           int16
	landone                           uint8
	switchland                        int16
	mapscreen                         uint32
	landclipy1, landclipx2, landcliph int16
	vbarclipx2                        int16
	vbarbot, vbarlarg, vbarx, precx   int16
	skyposx, skyposy, skyleft         int16
	ftstpix                           uint8
	xtstpix, ytstpix                  int16
	antstpix                          int32
	cxtstpix, cytstpix, cztstpix      int16
	dtstpix, ntstpix, etstpix         int16
	solpixy, solh, toppixy, toph      int16
	vdarkw                            int16
	texPerspStep                      uint32
	texHbase                          int16
	signedx, signedy                  int16
	vgamodulo, bitmodulo              int16
	spritprof, spritnext              int32
	ztflowx, ztflowy                  uint32
}
