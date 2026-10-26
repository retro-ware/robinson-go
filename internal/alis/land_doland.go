package alis

// The RRQ terrain column renderer (alis render/rrq3d_dos.c doland_dos,
// barland_dos and tbarland_dos).

func swap16(x uint32) uint32 { return x<<16 | x>>16 }

func carry(a, b uint32) uint32 {
	if a+b < a {
		return 1
	}
	return 0
}

// cellAt returns the address of the terrain cell (x, y) in the strip
// table, wrapping out-of-range cells when the land wraps; ok is false for
// cells off a non-wrapping land.
func (v *VM) cellAt(rc, grid uint32, x, y uint16) (cell uint32, wrapped, ok bool) {
	maxX, maxY := v.r16(rc-0x294), v.r16(rc-0x292)
	wrap := v.mem[rc-0x3fe] == 1
	var strip uint32
	sub := false
	if uint16(maxX) < x {
		if !wrap {
			return 0, false, false
		}
		var wx int16
		if maxX <= int16(x) {
			wx = maxX * 2
		}
		strip = v.ru32(grid + uint32(int32(int16((wx-int16(x))*4))))
		sub = uint16(maxY) < y
	} else {
		if uint16(maxY) < y && !wrap {
			return 0, false, false
		}
		sub = uint16(maxY) < y
		strip = v.ru32(grid + uint32(int32(int16(x<<2))))
	}
	if sub {
		if maxY <= int16(y) {
			strip += uint32(int32(maxY) << 2)
		}
		return strip - 2*uint32(int32(int16(y))), true, true
	}
	return strip + 2*uint32(int32(int16(y))), uint16(maxX) < x, true
}

func (v *VM) doland(sc, rc uint32) {
	img, ld := &v.img, &v.land
	rd32 := func(o uint32) int32 { return v.r32(rc - o) }
	hi := func(o uint32) int16 { return int16(v.r32(rc-o) >> 16) }
	lo := func(o uint32) int16 { return int16(v.r32(rc - o)) }
	r16 := func(o uint32) int16 { return v.r16(rc - o) }

	img.spritprof = 0x8000
	img.spritnext = int32(v.spr(v.ru16(sc + 2)).link())
	if img.spritnext != 0 {
		img.spritprof = int32(v.spr(uint16(img.spritnext)).newd())
	}
	if v.r16(sc+0xa2) != 0 {
		v.skytofen(sc, rc)
	}

	alt := img.atalti
	var altDelta uint32
	grid := img.atlland
	ld.fprectop, ld.fprectopa, ld.adresa = 0, 0, 0
	ld.prectopa, ld.prectopb, ld.prectopc = 1000, 1000, 1000
	ld.precbota, ld.precbotb, ld.precbotc = 1000, 1000, 1000

	v.w32(rc-0x25a, 0)
	v.w32(rc-0x27c, 0)
	camX, camY := v.ru16(rc-0x3a2), v.ru16(rc-0x3a0)
	v.wu32(rc-0x2a8, uint32(camX)<<16)
	v.wu32(rc-0x2a4, uint32(camY)<<16)
	stepXRaw := uint32(rd32(0x2c4))
	stepX := swap16(stepXRaw)
	stepYRaw := uint32(rd32(0x2c0))
	stepY := swap16(stepYRaw)
	v.wu32(rc-0x270, uint32(uint16(r16(0x26a)+r16(0x352))))

	project := func(gx, gy int16) {
		sh := v.ru16(rc-0x3c0) & 0x3f
		px := gx<<sh - r16(0x360)
		py := gy<<sh - r16(0x35e)
		px, py = v.glandtopix(rc, px, py, r16(0x378)+px-r16(0x37e), r16(0x376)+py-r16(0x37c), hi(0x2e0))
		v.wu32(rc-0x280, uint32(uint16(r16(0x26c)+px))<<16)
		v.w32(rc-0x27c, int32(int16((int32(py-px)<<8)/int32(r16(0x3a4))))<<8)
	}
	project(int16(camX), int16(camY))

	travX, travY := uint32(camX)<<16, uint32(camY)<<16
	img.texHbase = int16((uint16(travX) + uint16(travY)) << 4)

	for {
		v.w32(rc-0x2a8, rd32(0x2bc)+rd32(0x2a8))
		v.w32(rc-0x2a4, rd32(0x2b8)+rd32(0x2a4))
		rowX := travX
		rowY := travY + uint32(int32(hi(0x29c)))<<16

		snap := func(lt func() bool, sub bool) {
			for n := 512; n > 0 && lt(); n-- {
				if sub {
					rowY -= stepYRaw
					rowX -= stepXRaw
				} else {
					rowY += stepYRaw
					rowX += stepXRaw
				}
			}
		}
		behindX := func() bool { return hi(0x2a8) < int16(rowX>>16) }
		aheadX := func() bool { return int16(rowX>>16) < hi(0x2a8) }
		behindY := func() bool { return hi(0x2a4) < int16(rowY>>16) }
		aheadY := func() bool { return int16(rowY>>16) < hi(0x2a4) }
		switch {
		case stepXRaw == 0x10000:
			if hi(0x2bc) < 0 {
				snap(behindX, true)
			} else {
				snap(aheadX, false)
			}
		case stepXRaw == 0xffff0000:
			if hi(0x2bc) < 0 {
				snap(behindX, false)
			} else {
				snap(aheadX, true)
			}
		default:
			rowY = travY
			rowX = travX + uint32(int32(hi(0x2a0)))<<16
			if stepYRaw == 0x10000 {
				if hi(0x2b8) < 0 {
					snap(behindY, true)
				} else {
					snap(aheadY, false)
				}
			} else if hi(0x2b8) < 0 {
				snap(behindY, false)
			} else {
				snap(aheadY, true)
			}
		}

		v.w32(rc-0x256, rd32(0x252)+rd32(0x256))
		img.vdarkw = int16((rd32(0x256) >> 16 & 0xff) << 8)
		depth := rd32(0x2dc) + rd32(0x2e0)
		v.w32(rc-0x2e0, depth)
		if depth < 0 {
			v.w32(rc-0x2e0, rd32(0x2e0)-rd32(0x2dc))
		}
		v.w32(rc-0x2d8, rd32(0x2d4)+rd32(0x2d8))

		colStep := swap16(uint32(rd32(0x27c)))
		colPos := swap16(uint32(rd32(0x280)))
		if div := uint16(uint32(rd32(0x27c))>>8 | uint32(rd32(0x27c))<<24); div > 0 {
			q := 0x200000 / uint32(div)
			if 0x200000%uint32(div) != 0 {
				q++
			}
			img.texPerspStep = uint32(uint16(q))
		} else {
			img.texPerspStep = 0
		}
		v.w32(rc-0x278, rd32(0x270))

		if r16(0x24e) > -1 {
			project(int16(rowX>>16), int16(rowY>>16))
			den := int16(1)
			if s := int32(r16(0x3a8)) + int32(hi(0x2e0)); s > 0 {
				den = int16(s)
			}
			v.wu32(rc-0x270, uint32(uint16(r16(0x26a)+int16(rd32(0x2d8)/int32(den)))))
		}

		if int32(hi(0x2e0)) <= img.spritprof {
			v.spritaff(hi(0x2e0))
		}

		alt += altDelta
		seg := uint16((rd32(0x2e0) - rd32(0x2c8)) >> 8 / int32(r16(0x262)))
		if r16(0x24e) == 1 {
			seg = 0xb3
		}
		if seg > 0xb3 {
			if int16(seg) < 0 {
				seg = 0
			} else {
				seg = 0xb3
			}
		}
		altDelta = img.atalti + uint32(seg)*0x200 - alt
		altAt := func(i int32) int16 { return v.r16(alt + altDelta + uint32(i*2)) }

		img.precx = img.clipx1 - 0x10
		v.w16(rc-0x260, img.clipy2)
		v.w16(rc-0x25c, img.clipy2)
		ld.fprectop, ld.fprectopa = 0, 0

		rows := v.ru16(rc-0x24e) - 1
		v.wu16(rc-0x24e, rows)
		if int16(rows) < 0 && r16(0x24e) < -3 {
			v.spritaff(-1)
			return
		}

		savedColX := rd32(0x280)
		scanX := uint32(uint16(rowX))<<16 | rowX>>16
		scanY := uint32(uint16(rowY))<<16 | rowY>>16

		for {
			maxY := r16(0x260)
			prevMax := max(r16(0x25e), maxY)
			barX := int16(colPos)
			for hi(0x280) < barX {
				v.w16(rc-0x25e, maxY)
				var ph uint16
				nx := uint16(int16(stepX+scanX)) + uint16(carry(stepX, scanX))
				scanX = uint32(uint16((stepX+scanX)>>16))<<16 | uint32(nx)
				ny := uint16(int16(stepY+scanY)) + uint16(carry(stepY, scanY))
				scanY = uint32(uint16((stepY+scanY)>>16))<<16 | uint32(ny)
				v.w32(rc-0x280, rd32(0x27c)+rd32(0x280))
				if c, _, ok := v.cellAt(rc, grid, nx, ny); ok {
					ld.adresa = c
					ph = v.ru16(c) & 0xff
				}
				maxY = lo(0x270) + v.r16(alt+altDelta+uint32(ph)*2)
				v.w16(rc-0x260, maxY)
				prevMax = max(prevMax, maxY)
			}
			v.landColumn(sc, rc, grid, alt, altAt, travX, travY, scanX, stepXRaw, stepY, colPos, barX, prevMax)

			img.texHbase += 0x2000
			img.precx = barX
			if img.landclipx2 < barX {
				break
			}
			travX += stepXRaw
			travY += stepYRaw
			sum := colStep + colPos
			colPos = sum&0xffff0000 | uint32(uint16(int16(sum)+int16(carry(colStep, colPos))))
		}
		v.w32(rc-0x280, savedColX)
		travX, travY = rowX, rowY
	}
}

// landColumn draws the ground, overhang or billboard of the cell under
// the current column.
func (v *VM) landColumn(sc, rc, grid, alt uint32, altAt func(int32) int16, travX, travY, scanX, stepXRaw, stepY, colPos uint32, barX, prevMax int16) {
	img, ld := &v.img, &v.land
	cx, cy := uint16(travX>>16), uint16(travY>>16)
	cell, wrapped, ok := v.cellAt(rc, grid, cx, cy)
	if !ok {
		return
	}
	groundY := v.r16(rc - 0x25c)
	data := v.ru16(cell)
	img.solh = int16(data & 0xff)
	img.solpixy = int16(v.r32(rc-0x278)) + v.r16(alt+uint32(int32(img.solh*2)))
	v.w16(rc-0x246, img.solpixy)
	v.w16(rc-0x25c, img.solpixy)
	if img.solpixy < groundY {
		v.w16(rc-0x246, groundY)
		groundY = img.solpixy
	}
	dirX := int16(stepXRaw >> 16)
	ti := int16((data&0x3f00)>>3) - 0xc00
	tix := rc + uint32(int32(ti))
	ld.fillColor = v.mem[tix+8]
	ld.savedSI = 0
	if extra := v.r16(tix + 2); extra != 0 {
		ld.savedSI = v.r16(alt + uint32(int32(extra)*2))
		groundY += ld.savedSI
	}
	packed := uint32(uint16(travX))<<16 | travX>>16
	if prevMax > groundY {
		v.barland(cell, rc, dirX, int16(uint32(v.r32(rc-0x2c0))>>16), groundY, prevMax-groundY, ti, barX, packed)
	}

	feat := int8(v.mem[tix+0x14])
	if feat == 0 {
		ld.fprectop, ld.fprectopa = 0, 0
		return
	}
	if wrapped {
		return
	}
	if feat == 1 {
		v.barsprite(rc, ti, int16(cx), int16(cy))
		ld.fprectop, ld.fprectopa = 0, 0
		return
	}
	if feat > 0 {
		return
	}

	// overhang or bridge
	ld.notopa = 1
	prevGX := uint16(scanX >> 16)
	ohPacked := uint32(prevGX)<<16 | uint32(ld.prectopa)
	ohPrevBot := ld.precbota
	cont := false
	if ld.adresa != 0 {
		ld.solha = v.ru16(ld.adresa) & 0xff
		pti := rc + uint32(int32(int16((v.ru16(ld.adresa)&0x3f00)>>3)-0xc00))
		if int8(v.mem[pti+0x14]) < 0 {
			thick := v.r16(pti + 0x16)
			ceil := v.ru16(uint32(int32(ld.adresa)+v.r32(pti+0x10))) & 0xff
			dh := ceil - ld.solha
			if int16(ceil) > int16(ld.solha) && thick > int16(dh) {
				botH := ceil
				if int16(ld.solha+uint16(thick)-dh) < int16(ceil) {
					botH = uint16(thick) - dh
				}
				top := int16(v.r32(rc-0x270)) + altAt(int32(ceil))
				bot := uint16(int16(v.r32(rc-0x270)) + altAt(-int32(int16(botH-ceil))))
				ld.adresa = 0
				if ld.fprectopa == 0 {
					ld.fprectopa = 1
					ld.prectopc = 5000
					ld.precbotc = 0xec78
					ld.prectopa = uint16(top)
					ld.precbota = bot
				} else {
					ld.prectopb, ld.precbotb = ld.prectopc, ld.precbotc
					if int32(ld.prectopa) < int32(top) {
						ohPacked = uint32(prevGX)<<16 | uint32(uint16(top))
					}
					if int32(int16(ohPacked)) < int32(ld.prectopc) {
						ohPacked = ohPacked&0xffff0000 | uint32(ld.prectopc)
					}
					minBot := ld.precbota
					if int16(bot) < int16(ld.precbota) {
						minBot = bot
					}
					ohPrevBot = minBot
					if int16(ld.precbotc) < int16(minBot) {
						ohPrevBot = ld.precbotc
					}
					ld.prectopa, ld.precbota = uint16(top), bot
					if top > int16(ld.prectopc) {
						ld.prectopc = uint16(top)
					}
					if int16(bot) < int16(ld.precbotc) {
						ld.precbotc = bot
					}
					if int16(ohPacked) < int16(ohPrevBot) {
						ld.notopa = 0
					}
				}
				cont = true
			}
		}
		if !cont {
			ld.fprectopa = 0
		}
	}

	cell = uint32(v.r32(tix+0x10)+2*int32(int16(cy))) + v.ru32(grid+uint32(int32(int16(cx<<2))))
	ld.fbottom = 0
	od := v.ru16(cell)
	img.toph = int16(od & 0xff)
	oti := int16((od&0x3f00)>>3) - 0xc00
	rel := uint16(img.toph - img.solh)
	avail := v.ru16(tix + 0x16)
	if img.toph > img.solh && int16(avail) > int16(rel) {
		rem := avail - rel
		if img.solh+int16(rem) < img.toph {
			ld.fbottom = 1
			rel = rem
		}
		botOff := int16(rel) - img.toph
		topPix := uint16(int16(v.r32(rc-0x278)) + v.r16(alt+uint32(int32(img.toph*2))))
		clipY := v.r16(rc - 0x246)
		img.toppixy = int16(topPix)
		v.wu16(rc-0x246, topPix)
		prevTop := ld.prectopi
		ld.prectopi = topPix
		if int16(topPix) < int16(prevTop) {
			v.wu16(rc-0x246, ld.prectopi)
			prevTop = ld.prectopi
			topPix = ld.prectopi
		}
		ld.prectopi = topPix
		if ld.fbottom != 0 {
			clipY = int16(v.r32(rc-0x278)) + v.r16(alt+uint32(int32(int16(botOff*-2))))
		}
		ld.botalt, ld.bothigh = ld.precboti, uint16(clipY)
		if int32(ld.precboti) < int32(clipY) {
			ld.bothigh, ld.botalt = ld.precboti, uint16(clipY)
		}
		ld.precboti = uint16(clipY)
		if ld.fprectop != 0 && int16(ld.botalt) > int16(prevTop) {
			barH := ld.botalt - prevTop
			height := int32(barH)
			draw := true
			if ld.notopa == 0 {
				chk := int16(ohPacked)
				delta := uint16(chk) - prevTop
				ohPacked = ohPacked&0xffff0000 | uint32(delta)
				if !(chk > int16(prevTop)) {
					newTop := int16(prevTop + barH)
					adj := uint16(newTop) - ohPrevBot
					height = int32(adj)
					prevTop = ohPrevBot
					if !(newTop > int16(ohPrevBot)) {
						draw = false
					}
				} else if int16(barH) <= int16(ohPrevBot-prevTop) {
					ld.fbottom = 0
					height = int32(delta)
				}
			}
			if draw {
				saved := v.r16(rc - 0x25c)
				v.wu16(rc-0x25c, ld.prectopi)
				v.tbarland(cell, rc, dirX, int16(stepY), int16(prevTop), int16(height), oti, int16(colPos), packed)
				v.w16(rc-0x25c, saved)
			}
		}
		ld.fprectop = 1
	}
}

// clipBarY clips a bar to the land window; false means nothing is left.
func (v *VM) clipBarY(drawy, barheight *int16) bool {
	img := &v.img
	d := *drawy - img.landclipy1 - img.landcliph
	if (int32(img.landcliph) <= int32(uint16(*drawy-img.landclipy1)) && d != 0) || -*barheight-d < 0 {
		if img.clipy2 < *drawy {
			return false
		}
		bot := *barheight + *drawy - 1
		if bot < img.landclipy1 {
			return false
		}
		if *drawy < img.landclipy1 {
			*barheight = *barheight - img.landclipy1 + *drawy
			*drawy = img.landclipy1
		}
		if img.clipy2 < bot {
			*barheight = *barheight - bot + img.clipy2
		}
	}
	return true
}

// barWidth clips a bar horizontally; 0 means nothing to draw.
func (v *VM) barWidth(barx int16) uint16 {
	img := &v.img
	if img.clipx1 > barx {
		return 0
	}
	w := uint16(barx - img.precx)
	if w == 0 || barx < img.precx || w >= 0x80 {
		return 0
	}
	img.vbarx = img.precx
	img.vbarlarg = int16(w)
	return w
}

// hitBar records the cell under the test pixel when pointpix probes the
// land.
func (v *VM) hitBar(cell, rc uint32, drawy, barheight int16, w uint16, stepX, stepY int16, packed uint32) {
	img := &v.img
	clip := v.r16(rc - 0x246)
	if drawy < clip {
		if drawy+barheight <= clip {
			return
		}
		barheight += clip - drawy
		drawy = clip
	}
	if drawy <= img.ytstpix && img.ytstpix < drawy+barheight && img.precx <= img.xtstpix && img.xtstpix < img.precx+int16(w) {
		c := v.ru16(cell)
		img.ntstpix = int16(c >> 8)
		img.cztstpix = int16(c & 0xff)
		sh := uint8(v.ru16(rc-0x3c0)&0x3f) & 0x1f
		img.cxtstpix = int16(uint16((int16(packed>>16)*2-stepX)<<sh) >> 1)
		img.cytstpix = int16(uint16((int16(packed)*2-stepY)<<sh) >> 1)
		img.etstpix = -2
		img.dtstpix = int16(v.r32(rc-0x2e0) >> 16)
	}
}

func (v *VM) barland(cell, rc uint32, stepX, stepY, bary, barheight, index, barx int16, packed uint32) {
	img := &v.img
	drawy := bary
	if !v.clipBarY(&drawy, &barheight) {
		return
	}
	w := v.barWidth(barx)
	if w == 0 {
		return
	}
	img.vbarbot = 0
	if v.r16(rc-0x24e) < 0 {
		return
	}
	if img.ftstpix == 0 {
		if v.mem[rc+8+uint32(int32(index))] != 0 {
			v.bartra(cell, rc, uint16(drawy), index, int16(w), barheight, bary)
		} else {
			v.barlands(uint16(drawy), barheight, int16(w))
		}
		return
	}
	v.hitBar(cell, rc, drawy, barheight, w, stepX, stepY, packed)
}

func (v *VM) tbarland(cell, rc uint32, stepX, stepY, bary, barheight, index, barx int16, packed uint32) {
	img, ld := &v.img, &v.land
	ld.savedSI = 0
	drawy := bary
	if !v.clipBarY(&drawy, &barheight) {
		return
	}
	w := v.barWidth(barx)
	if w == 0 || v.r16(rc-0x24e) < 0 {
		return
	}
	clip := v.r16(rc - 0x246)
	if int16(ld.bothigh) < clip {
		ld.bothigh = uint16(clip)
	}
	img.vbarbot = 0
	if ld.fbottom != 0 {
		ld.bottomType = index
		if int16(ld.bothigh) <= drawy {
			v.tbarBottom(cell, rc, drawy, barheight, index, w)
			return
		}
		if c := barheight + drawy - int16(ld.bothigh); c > 0 {
			img.vbarbot = c
		}
	}
	if img.ftstpix == 0 {
		if v.mem[rc+8+uint32(int32(index))] != 0 {
			v.bartra(cell, rc, uint16(drawy), index, int16(w), barheight, bary)
		} else {
			v.barlands(uint16(drawy), barheight, int16(w))
		}
		return
	}
	v.hitBar(cell, rc, drawy, barheight, w, stepX, stepY, packed)
}

// tbarBottom draws the narrowing underside of an overhang.
func (v *VM) tbarBottom(cell, rc uint32, drawy, barheight, index int16, w uint16) {
	img, ld := &v.img, &v.land
	ix := rc + uint32(int32(index))
	precx := uint32(int32(img.precx))
	stride := uint32(uint16(img.wloglarg))
	shift := v.mem[ix]
	hstep := uint16(img.texPerspStep)
	vstep := hstep>>((8-uint16(shift))&0x1f) + 1
	if barheight < 1 {
		img.vbarbot = 0
		return
	}
	dist := ld.botalt - ld.bothigh
	if dist == 0 || int32(ld.botalt) < int32(int16(ld.bothigh)) {
		img.vbarbot = 0
		return
	}
	sw := uint16(img.vbarlarg)
	if img.vbarlarg <= 0 {
		sw = w
	}
	step := v.aliasStep(dist, sw)
	var vc uint16
	colW := uint8(w)
	if int16(ld.bothigh) < drawy {
		pre := uint16(drawy - int16(ld.bothigh))
		vc = pre * vstep
		colW -= uint8(pre * uint16(step) >> 8)
	}
	tp := v.ru32(ix + 4)
	if tp == 0 {
		return
	}
	to := v.ru32(tp)
	texW := uint16(v.mem[tp+to+2])
	texH := v.ru16(tp + to + 4)
	tex := tp + to + 8
	dp := int16(uint8(uint16(img.vdarkw)>>8)) - int16(v.mem[cell+1]>>6*2)
	if dp < 0 {
		dp = 0
	}
	frac := uint16(colW)<<8 | 0xff
	scr := v.ru32(img.atlpix+uint32(int32(int16((drawy-img.wlogy1)*4)))) + precx
	rtl := ld.botalt == ld.precboti
	for rows := uint8(barheight); ; {
		if cols := uint8(frac >> 8); cols > 0 {
			if rtl {
				v.bartraLine(scr, int16(uint8(w))-int16(cols), int16(uint8(w))-1, vc, shift, texW, texH, tex, dp, -1)
			} else {
				v.bartraLine(scr, 0, int16(cols)-1, vc, shift, texW, texH, tex, dp, 1)
			}
		}
		vc += vstep
		frac -= uint16(step)
		scr += stride
		rows--
		if rows == 0 {
			break
		}
	}
}

func (v *VM) bartraLine(dst uint32, start, end int16, vc uint16, shift uint8, texW, texH uint16, tex uint32, dark int16, dir int) {
	img := &v.img
	vm := vc & (texH << shift)
	pt := v.ptrdark + uint32(uint16(dark))<<8
	step := uint16(img.texPerspStep)
	if dir < 0 {
		h := uint16(img.texHbase) + 0x2000
		for c := end; c >= start; c-- {
			t := v.mem[tex+uint32(vm|(h>>8)&texW)]
			v.mem[dst+uint32(int32(c))] = v.mem[pt+uint32(t)]
			h -= step
		}
		return
	}
	h := uint16(img.texHbase)
	for c := start; c <= end; c++ {
		t := v.mem[tex+uint32(vm|(h>>8)&texW)]
		v.mem[dst+uint32(int32(c))] = v.mem[pt+uint32(t)]
		h += step
	}
}
