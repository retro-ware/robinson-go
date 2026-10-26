package alis

// Sprite blitters into the 8-bit logic screen (alis image.c destofen,
// draw_* and draw_requiem_map; PC chunky paths only).

type rect struct{ x1, y1, x2, y2 int32 }

// wdraw is the current blit destination: logic, or the background cache
// while fenetre composites into it.
func (v *VM) wdraw() []byte {
	if v.img.wlogicBack {
		return v.img.backmap
	}
	return v.img.logic
}

func (v *VM) destofen(s sprite) {
	img := &v.img
	if img.sback != 0 && img.backmap != nil && &s[0] == &v.spr(uint16(img.backsprite))[0] {
		bx1, by1 := max(img.clipx1, img.backx1), max(img.clipy1, img.backy1)
		bx2, by2 := min(img.clipx2, img.backx2), min(img.clipy2, img.backy2)
		if bx1 <= bx2 && by1 <= by2 {
			for y := int(by1); y <= int(by2); y++ {
				o := y*gameWidth + int(bx1)
				copy(img.logic[o:o+int(bx2-bx1)+1], img.backmap[o:])
			}
		}
		return
	}

	addr := s.newad()
	if addr == 0 || s.data() == 0 {
		return
	}
	addr += uint32(v.r32(addr))
	bm := addr

	pos := rect{int32(s.newx()), int32(s.newy()), int32(s.newx() + v.r16(bm+2)), int32(s.newy() + v.r16(bm+4))}
	flip := s.newf() != 0
	width := int32(s.width()) + 1

	cx1, cy1, cx2, cy2 := int32(img.clipx1), int32(img.clipy1), int32(img.clipx2), int32(img.clipy2)
	if cx2 < pos.x1 || cy2 < pos.y1 || pos.x2 < cx1 || pos.y2 < cy1 {
		return
	}
	img.blocx1 = int16(max(pos.x1, cx1))
	img.blocy1 = int16(max(pos.y1, cy1))
	img.blocx2 = int16(min(pos.x2, cx2))
	img.blocy2 = int16(min(pos.y2, cy2))
	bx1, by1, bx2, by2 := int32(img.blocx1), int32(img.blocy1), int32(img.blocx2), int32(img.blocy2)

	b := rect{0, 0, width, int32(s.height()) + 1}
	if bx1 < 0 {
		b.x1 -= bx1
		b.x2 += bx1
	}
	if bx1 > pos.x1 {
		b.x1 += bx1 - pos.x1
		b.x2 -= bx1 - pos.x1
	}
	if bx2 <= pos.x2 {
		b.x2 -= pos.x2 - bx2
	}
	if by1 > pos.y1 {
		b.y1 += by1 - pos.y1
		b.y2 -= by1 - pos.y1
	}
	if by2 < pos.y2 {
		b.y2 -= pos.y2 - by2
	}
	b.x2 += b.x1
	b.y2 += b.y1

	at := bm + 6
	switch v.mem[bm] {
	case 0x01:
		v.drawRect(&pos, &b, v.mem[bm+1])
	case 0x00:
		v.draw4bit(at, &pos, &b, width, flip, 0, true)
	case 0x02:
		v.draw4bit(at, &pos, &b, width, flip, 0, false)
	case 0x10:
		v.draw4bit(at+2, &pos, &b, width, flip, v.mem[at], true)
	case 0x12:
		v.draw4bit(at+2, &pos, &b, width, flip, v.mem[at], false)
	case 0x14:
		v.draw8bit(at, &pos, &b, width, flip, true)
	case 0x16:
		v.draw8bit(at, &pos, &b, width, flip, false)
	case 0x40:
		// draw_fli_video: films draw themselves
	case 0x7f:
		v.drawRequiemMap(s, s.newad()+uint32(v.r32(s.newad())))
	}
}

func (v *VM) drawRect(pos, b *rect, c uint8) {
	dst := v.wdraw()
	t := pos.x1 + (b.y1+pos.y1)*gameWidth
	for h := b.y1; h < b.y2; h, t = h+1, t+gameWidth {
		for w := b.x1; w < b.x2; w++ {
			dst[t+w] = c
		}
	}
}

func (v *VM) draw8bit(at uint32, pos, b *rect, width int32, flip, transp bool) {
	dst := v.wdraw()
	wadd := gameWidth - (b.x2 - b.x1)
	src := int32(at) + 2 + b.y1*width
	if flip {
		src += width - 1
	}
	t := b.x1 + pos.x1 + (pos.y1+b.y1)*gameWidth
	for h := b.y1; h < b.y2; h, src, t = h+1, src+width, t+wadd {
		for w := b.x1; w < b.x2; w, t = w+1, t+1 {
			var c uint8
			if flip {
				c = v.mem[src-w]
			} else {
				c = v.mem[src+w]
			}
			if c != 0 || !transp {
				dst[t] = c
			}
		}
	}
}

// draw4bit draws a 4-bit chunky sprite (two pixels per byte, high nibble
// first) adding off to each colour; transp skips colour 0.
func (v *VM) draw4bit(at uint32, pos, b *rect, width int32, flip bool, off uint8, transp bool) {
	dst := v.wdraw()
	swadd := int32(uint16(width) >> 1)
	src := int32(at) + b.y1*swadd
	tgt := (pos.y1 + b.y1) * gameWidth
	plot := func(t int32, c uint8) {
		if c != 0 || !transp {
			dst[t] = off + c
		}
	}
	if flip {
		if b.x1%2 == 1 {
			sp, tp := swadd-1-(b.x1>>1)+src, b.x1+pos.x1+tgt
			for h := b.y1; h < b.y2; h, sp, tp = h+1, sp+swadd, tp+gameWidth {
				plot(tp, v.mem[sp]>>4)
			}
			b.x1++
		}
		if (b.x2-1)%2 == 0 {
			sp, tp := swadd-1-(b.x2>>1)+src, b.x2+pos.x1-1+tgt
			for h := b.y1; h < b.y2; h, sp, tp = h+1, sp+swadd, tp+gameWidth {
				plot(tp, v.mem[sp]&0xf)
			}
			b.x2--
		}
		if b.x1 != b.x2 {
			sp, tp := swadd-1+src, b.x1+pos.x1+tgt
			twadd := gameWidth - (b.x2 - b.x1)
			b.x1 >>= 1
			b.x2 >>= 1
			for h := b.y1; h < b.y2; h, sp, tp = h+1, sp+swadd, tp+twadd {
				for w := b.x1; w < b.x2; w, tp = w+1, tp+2 {
					c := v.mem[sp-w]
					plot(tp, c&0xf)
					plot(tp+1, c>>4)
				}
			}
		}
		return
	}
	if b.x1%2 == 1 {
		sp, tp := (b.x1>>1)+src, b.x1+pos.x1+tgt
		for h := b.y1; h < b.y2; h, sp, tp = h+1, sp+swadd, tp+gameWidth {
			plot(tp, v.mem[sp]&0xf)
		}
		b.x1++
	}
	if (b.x2-1)%2 == 0 {
		b.x2--
		sp, tp := (b.x2>>1)+src, b.x2+pos.x1+tgt
		for h := b.y1; h < b.y2; h, sp, tp = h+1, sp+swadd, tp+gameWidth {
			plot(tp, v.mem[sp]>>4)
		}
	}
	if b.x1 != b.x2 {
		sp, tp := src, b.x1+pos.x1+tgt
		twadd := gameWidth - (b.x2 - b.x1)
		b.x1 >>= 1
		b.x2 >>= 1
		for h := b.y1; h < b.y2; h, sp, tp = h+1, sp+swadd, tp+twadd {
			for w := b.x1; w < b.x2; w, tp = w+1, tp+2 {
				c := v.mem[sp+w]
				plot(tp, c>>4)
				plot(tp+1, c&0xf)
			}
		}
	}
}

// chartpixel draws one textured terrain pixel of the RRQ map; missing
// textures leave the pixel alone, as the original.
func (v *VM) chartpixel(t int32, src, types uint32, xdc, ydc uint16) {
	cell := v.ru16(src)
	te := types + uint32(cell>>8&0x3f)*32
	if int8(v.mem[te+0x14]) < 0 {
		src += v.ru32(te + 0x10)
		cell = v.ru16(src)
		te = types + uint32(cell>>8&0x3f)*32
	}
	if v.mem[te+8] == 0 || v.ru32(te+4) == 0 {
		return
	}
	tp := v.ru32(te + 4)
	tex := tp + v.ru32(tp)
	base := v.ru16(te + 2)
	yw := (ydc & (v.ru16(tex+4) - base)) + base
	ys := yw * (v.ru16(tex+2) + 1)
	xw := xdc & v.ru16(tex+2)
	px := v.mem[tex+8+uint32(ys)+uint32(xw)]
	nx2 := int8((cell >> 14 & 3) << 1)
	shade := int16((-int32(nx2)+int32(int8(v.basedark)))>>1) + int16(uint8(cell)>>4)
	shade = min(max(shade, 0), 15)
	v.img.logic[t] = v.mem[v.ptrdark+uint32(uint16(shade)<<8)+uint32(px)]
}

func (v *VM) drawRequiemMap(s sprite, bitmap uint32) {
	img := &v.img
	md := bitmap + 0x3d8
	if v.mem[bitmap-0x28] != 9 {
		// tile maps: alis' mapchar is empty
		return
	}
	baseCol := int32(v.ru16(md-0x3e2) / v.ru16(md-0x3f4))
	pixX := int32(img.blocx1) - int32(s.newx())
	baseRow := int32(v.ru16(md-0x3e0) / v.ru16(md-0x3f2))
	pixY := int32(v.r16(md-0x3d4)) + int32(s.newy()) - int32(img.blocy2)
	ent := v.ru32(md - 0x3ba)
	if ent == 0 {
		return
	}
	cgBm := baseCol + pixX>>4
	rowBm := baseRow + pixY
	bmStride := uint32(v.ru16(md - 0x3c4))
	bmBase := md
	if int8(v.mem[md-1]) < 0 {
		bmBase = v.ru32(v.ru32(md))
	}
	bmSrc := bmBase + uint32(cgBm)*bmStride + uint32(rowBm*2)

	xld := v.r16(md-0x3c0) - v.r16(ent-0x3c0)
	yld := v.r16(md-0x3be) - v.r16(ent-0x3be)
	cgEnt, rowEnt := cgBm, rowBm
	if xld > 0 {
		cgEnt <<= xld
	}
	if yld > 0 {
		rowEnt <<= yld
	}
	entStride := uint32(v.ru16(ent - 0x3c4))
	entBase := ent
	if int8(v.mem[ent-1]) < 0 {
		entBase = v.ru32(v.ru32(ent))
	}
	entSrc := entBase + uint32(cgEnt)*entStride + uint32(rowEnt*2)
	xsd := v.r16(md-0x3c0) - 4 - v.r16(ent-0x3c0)
	pixStride := entStride
	if xsd > 0 {
		pixStride <<= xsd
	}
	grpStride := pixStride << 4
	rowAdv := uint32(2)
	if yld > 0 {
		rowAdv = 2 << yld
	}
	types := ent - 0xc00

	sub := int16(pixX & 0xf)
	lastCg := int16(baseCol) + (img.blocx2-s.newx())>>4
	cgCount := lastCg - int16(cgBm) + 1
	height := img.blocy2 - img.blocy1 + 1
	xd := uint16(img.blocx1 - img.wlogx1 - sub)
	yd := uint16(img.blocy2 - img.wlogy1)
	t := int32(int16(yd))*gameWidth + int32(int16(xd))
	for ; height > 0; height-- {
		t0, e0, b0, x0 := t, entSrc, bmSrc, xd
		for range max(cgCount, 0) {
			mask := ^v.ru16(bmSrc)
			bmSrc += bmStride
			if mask == 0 {
				t += 16
				entSrc += grpStride
				continue
			}
			for bit := 15; bit >= 0; bit-- {
				if mask&(1<<bit) != 0 {
					v.chartpixel(t, entSrc, types, xd, yd)
				}
				t++
				xd++
				entSrc += pixStride
			}
		}
		t, entSrc, bmSrc, xd = t0, e0, b0, x0
		yd--
		bmSrc += 2
		entSrc += rowAdv
		t -= gameWidth
	}
}
