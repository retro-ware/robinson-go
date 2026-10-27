package alis

// The draw (START.EXE 0x6340..0x7139): each open scene brings its draw
// list up to date and redraws the rectangles that changed into the logic
// screen, which goes to video memory rectangle by rectangle.

// draw is 0x6340.
func (v *VM) draw() {
	if v.OnDraw != nil {
		v.OnDraw()
	}
	v.delayFrame()
	// 0x2875 is a bare ret; 0x2b47 (the screenshot key) is not ported
	v.img.vtiming = 0
	v.wu32(dsWlogic, v.ru32(dsLogic))
	v.wu16(dsWlogx1, v.ru16(dsLogx1))
	v.wu16(dsWlogy1, v.ru16(dsLogy1))
	v.wu16(dsWlogx2, v.ru16(dsLogx2))
	v.wu16(dsWlogy2, v.ru16(dsLogy2))
	v.wu16(dsWloglarg, v.ru16(dsLoglarg))
	for id := v.ru16(dsPtscreen); id != 0; id = v.ru16(v.sca(id) + 4) {
		a := v.sca(id)
		if v.mem[a]&0x40 != 0 {
			continue
		}
		v.wu32(dsScene, a)
		cx := v.ru16(a + 2)
		v.wu16(dsTa0, cx)
		v.affiscr(a, cx)
	}
	v.mem[dsMoveSc] = 0
	// the mouse sprite code at 0x640f is never reached
}

// affiscr is 0x6480: scene a, whose sprite cx heads its draw list.
func (v *VM) affiscr(a uint32, cx uint16) {
	v.mem[dsDrawn] = 0
	if v.mem[a+0x84] != 0 {
		v.scfollow(a)
	}
	if v.mem[dsMoveSc] != 0 || int8(v.mem[a]) < 0 {
		v.scmove(a)
	}
	defer func() { v.mem[a] &^= 0x80 }()
	if v.mem[a+1]&2 != 0 && v.mem[a]&0x80 != 0 {
		v.affiland(a)
		if v.mem[a]&0x80 != 0 {
			v.drawScenes(v.ru16(a + 4))
			v.showRect()
			return
		}
	}
	v.mem[dsWback] = 0
	if v.mem[a+1]&4 != 0 {
		v.mem[dsWback] = 1
	}
	if v.mem[a+1]&0x10 != 0 {
		v.affdirect(a, cx)
		return
	}
	for di := cx; ; {
		bx := di
		if di = v.fsu16(di, spLink); di == 0 {
			return
		}
		v.mem[dsFclip] = 0
		st := v.fs8(di, spState)
		if st == 0 {
			continue
		}
		v.mem[dsDrawn2] = 0
		v.mem[dsDrawn] = 1
		switch st {
		case 2:
			di = v.affmoved(di, bx, cx)
		case 0xff:
			di = v.affnew(di, bx, cx)
		default:
			di = v.affgone(di, bx, cx)
		}
	}
}

// affdirect is 0x655d: a scene drawn whole each time, without
// rectangles.
func (v *VM) affdirect(a uint32, cx uint16) {
	x, y := v.r16(a+0xe), v.r16(a+0x10)
	v.w16(dsFenx1, x)
	v.w16(dsFeny1, y)
	v.w16(dsBlocx1, x)
	v.w16(dsBlocy1, y)
	x2, y2 := x+v.r16(a+0x12), y+v.r16(a+0x14)
	v.w16(dsFenx2, x2)
	v.w16(dsFeny2, y2)
	v.w16(dsBlocx2, x2)
	v.w16(dsBlocy2, y2)
	v.wu16(dsBlocw8, uint16(x2-x+1)>>3)
	if v.mem[a+1]&0x40 == 0 {
		v.clearWindow()
	}
	for di := cx; ; {
		bx := di
		if di = v.fsu16(di, spLink); di == 0 {
			break
		}
		switch v.fs8(di, spState) {
		case 0:
		case 1:
			v.setFsu16(bx, spLink, v.fsu16(di, spLink))
			v.freeSprite(di)
			di = bx
		default:
			v.position(di)
			v.setFsu16(bx, spLink, v.fsu16(di, spLink))
			v.storeNew(di, true)
			v.insert(di, cx)
			di = bx
		}
	}
	v.drawScenes(v.ru16(dsPtscreen))
	v.showRect()
}

// drawScenes is 0x6608: the sprites of the open scenes from id on, each
// within its window and the rectangle 0x15fcc.
func (v *VM) drawScenes(id uint16) {
	for ; id != 0; id = v.ru16(v.sca(id) + 4) {
		b := v.sca(id)
		if v.mem[b]&0x40 != 0 {
			continue
		}
		di := v.ru16(b + 2)
		if di == 0 {
			continue
		}
		v.fenetre(di)
		if v.mem[dsFenOn] != 0 {
			v.drawSprites(di)
		}
	}
}

func (v *VM) drawSprites(di uint16) {
	for di = v.fsu16(di, spLink); di != 0; di = v.fsu16(di, spLink) {
		if int8(v.fs8(di, spState)) < 0 || int8(v.fs8(di, spNewf)) < 0 || v.fs16(di, spNewd) < 0 {
			continue
		}
		v.blitSprite(di)
	}
}

// affmoved is 0x682d, state 2: a linked element redraws with the others
// of its link; another redraws the union of its old and new rectangles if
// they overlap, else both. The newad word only is stored (0x6746).
func (v *VM) affmoved(di, bx, cx uint16) uint16 {
	v.linkStart(di)
	if v.r16(dsLinkid) >= 0 {
		v.accumulate(di)
		return v.affgroup(v.relink(di, bx, cx), di, bx, cx)
	}
	v.position(di)
	if v.overlap(di) {
		v.setFsu16(bx, spLink, v.fsu16(di, spLink))
		v.storeNew(di, false)
		v.insert(di, cx)
		v.redraw(di, cx)
		return bx
	}
	v.setFsu16(bx, spLink, v.fsu16(di, spLink))
	v.redraw(di, cx)
	v.storeNew(di, false)
	v.insert(di, cx)
	v.redraw(di, cx)
	return bx
}

// affnew is 0x680c, state 0xff: a new element.
func (v *VM) affnew(di, bx, cx uint16) uint16 {
	v.linkStart(di)
	return v.affgroup(v.relink(di, bx, cx), di, bx, cx)
}

// affgone is 0x6857, state 1 (and any other): an erased element.
func (v *VM) affgone(di, bx, cx uint16) uint16 {
	v.linkStart(di)
	return v.affgroup(v.unlinkFree(di, bx), di, bx, cx)
}

// affgroup is 0x6949 and its end 0x6876: the other elements of the link
// after di0 follow it, then the rectangle they cover is redrawn.
func (v *VM) affgroup(di, di0, bx0, cx uint16) uint16 {
	if v.r16(dsLinkid) >= 0 {
		for {
			bx := di
			if di = v.fsu16(di, spLink); di == 0 {
				break
			}
			if v.ru16(dsLinkid) != v.fsu16(di, spClinking) {
				continue
			}
			switch st := v.fs8(di, spState); {
			case st == 0:
			case int8(st) < 0:
				di = v.relink(di, bx, cx)
			case st == 2:
				v.accumulate(di)
				di = v.relink(di, bx, cx)
			default:
				di = v.unlinkFree(di, bx)
			}
		}
	}
	v.mem[dsFclip] = 1
	v.redraw(di0, cx)
	return bx0
}

// relink is 0x6890: di takes its new place in the draw list.
func (v *VM) relink(di, bx, cx uint16) uint16 {
	v.position(di)
	v.setFsu16(bx, spLink, v.fsu16(di, spLink))
	v.storeNew(di, true)
	v.accumulate(di)
	v.insert(di, cx)
	return bx
}

// unlinkFree is 0x6917: di leaves the draw list and is freed.
func (v *VM) unlinkFree(di, bx uint16) uint16 {
	v.accumulate(di)
	v.setFsu16(bx, spLink, v.fsu16(di, spLink))
	v.freeSprite(di)
	return bx
}

func (v *VM) freeSprite(di uint16) {
	v.setFsu16(di, spToNext, v.ru16(dsLibsprit))
	v.wu16(dsLibsprit, di)
}

// storeNew stores where 0x587a placed di; dword is false where the
// original stores the low word of newad only (0x6746).
func (v *VM) storeNew(di uint16, dword bool) {
	if dword {
		v.setFsu32(di, spNewad, v.ru32(dsNewad))
	} else {
		v.setFsu16(di, spNewad, v.ru16(dsNewad))
	}
	v.setFsu16(di, spNewx, v.ru16(dsNewx))
	v.setFsu16(di, spNewy, v.ru16(dsNewy))
	v.setFsu16(di, spNewd, v.ru16(dsNewd))
	v.setFs8(di, spNewf, v.mem[dsNewf])
	v.setFsu16(di, spWidth, v.ru16(dsNewl))
	v.setFsu16(di, spHeight, v.ru16(dsNewh))
	v.setFsu16(di, spZoomy, v.ru16(dsNewzoomy))
	v.setFsu16(di, spZoomx, v.ru16(dsNewzoomx))
}

// linkStart is 0x69a2: the link of di, and an empty rectangle.
func (v *VM) linkStart(di uint16) {
	v.wu16(dsLinkid, v.fsu16(di, spClinking))
	v.w16(dsClipx1, 0x7fff)
	v.w16(dsClipy1, 0x7fff)
	v.wu16(dsClipx2, 0x8000)
	v.wu16(dsClipy2, 0x8000)
}

// accumulate is 0x69d8: the rectangle grows to cover di.
func (v *VM) accumulate(di uint16) {
	if v.fs16(di, spNewd) < 0 {
		return
	}
	x := v.fs16(di, spNewx)
	if x <= v.r16(dsClipx1) {
		v.w16(dsClipx1, x)
	}
	if x += v.fs16(di, spWidth); x >= v.r16(dsClipx2) {
		v.w16(dsClipx2, x)
	}
	y := v.fs16(di, spNewy)
	if y <= v.r16(dsClipy1) {
		v.w16(dsClipy1, y)
	}
	if y += v.fs16(di, spHeight); y >= v.r16(dsClipy2) {
		v.w16(dsClipy2, y)
	}
}

// insert is 0x6ac8: di goes into the draw list of scene sprite cx, which
// runs from far to near: by depth, then cordspr, then element number,
// all descending; a new element goes before its equals.
func (v *VM) insert(di, cx uint16) {
	d := v.fs16(di, spNewd)
	o := int8(v.fs8(di, spCordspr))
	n := int8(v.fs8(di, spNumelem))
	bp, bx := cx, cx
	for {
		bp = bx
		if bx = v.fsu16(bx, spLink); bx == 0 {
			break
		}
		if int8(v.fs8(bx, spState)) < 0 {
			continue
		}
		if bd := v.fs16(bx, spNewd); d != bd {
			if d < bd {
				continue
			}
			break
		}
		if bo := int8(v.fs8(bx, spCordspr)); o != bo {
			if o < bo {
				continue
			}
			break
		}
		if bn := int8(v.fs8(bx, spNumelem)); n != bn {
			if n < bn {
				continue
			}
			break
		}
		if v.fs8(di, spState) != 0xff {
			continue
		}
		break
	}
	v.setFsu16(di, spLink, bx)
	v.setFsu16(bp, spLink, di)
	v.setFs8(di, spState, 0)
}

// overlap is 0x6e82: whether di's rectangle meets where 0x587a placed it;
// if so 0x15fd6 is the union of both.
func (v *VM) overlap(di uint16) bool {
	nx, ny := v.r16(dsNewx), v.r16(dsNewy)
	ox, oy := v.fs16(di, spNewx), v.fs16(di, spNewy)
	ox2, oy2 := ox+v.fs16(di, spWidth), oy+v.fs16(di, spHeight)
	nx2, ny2 := nx+v.r16(dsNewl), ny+v.r16(dsNewh)
	if v.fs16(di, spNewd) < 0 || ox > nx2 || oy > ny2 || ox2 < nx || oy2 < ny {
		v.mem[dsFclip] = 0
		return false
	}
	v.w16(dsClipx1, min(ox, nx))
	v.w16(dsClipy1, min(oy, ny))
	v.w16(dsClipx2, max(ox2, nx2))
	v.w16(dsClipy2, max(oy2, ny2))
	v.mem[dsFclip] = 1
	return true
}

// redraw is 0x6bce: the rectangle (0x15fd6 if fclip, else di's) within
// the window of scene sprite cx is drawn again, every open scene in it,
// and goes to video memory.
func (v *VM) redraw(di, cx uint16) {
	a := v.ru32(dsScene)
	if v.mem[a]&0x40 != 0 {
		return
	}
	v.mem[dsFenE9] = 0
	if !v.clipRect(di, cx) {
		return
	}
	if v.mem[a+1]&2 != 0 {
		v.mem[dsLandRd] = 1
	}
	for id := v.ru16(dsPtscreen); id != 0; id = v.ru16(v.sca(id) + 4) {
		b := v.sca(id)
		if v.mem[b]&0x40 != 0 {
			continue
		}
		sdi := v.ru16(b + 2)
		if sdi == 0 {
			continue
		}
		v.fenetre(sdi)
		if v.mem[dsFenOn] == 0 {
			continue
		}
		if v.mem[b+1]&2 != 0 {
			v.landRedraw(b, id, sdi)
			continue
		}
		v.drawSprites(sdi)
	}
	// the mouse sprite code at 0x6cca is never reached
	if v.mem[v.ru32(dsScene)+1]&0x20 == 0 {
		v.showRect()
	}
}

// clipRect is 0x6fba: the rectangle to redraw, 16 pixel aligned and
// within the window of scene sprite cx, into 0x15fcc.
func (v *VM) clipRect(di, cx uint16) bool {
	if v.mem[dsFclip] == 0 {
		if v.fs16(di, spNewd) < 0 {
			return false
		}
		// 0x6fd2 pushes cx and returns without popping it
		v.fatal("draw: START.EXE 0x6fd2 would return to a bad address")
		return false
	}
	if v.r16(dsClipx1) > v.fs16(cx, spDepx) || v.r16(dsClipy1) > v.fs16(cx, spDepy) ||
		v.r16(dsClipx2) < v.fs16(cx, spNewx) || v.r16(dsClipy2) < v.fs16(cx, spNewy) {
		return false
	}
	x1 := v.r16(dsClipx1) &^ 0xf
	v.w16(dsClipx1, x1)
	v.w16(dsBlocx1, max(x1, v.fs16(cx, spNewx)))
	v.w16(dsBlocy1, max(v.r16(dsClipy1), v.fs16(cx, spNewy)))
	x2 := v.r16(dsClipx2) | 0xf
	v.w16(dsClipx2, x2)
	v.w16(dsBlocx2, min(x2, v.fs16(cx, spDepx)))
	v.w16(dsBlocy2, min(v.r16(dsClipy2), v.fs16(cx, spDepy)))
	v.wu16(dsBlocw8, uint16(v.r16(dsBlocx2)-v.r16(dsBlocx1)+1)>>3)
	return true
}

// fenetre is 0x6d7f: the window of scene sprite di within the rectangle
// 0x15fcc, x 16 pixel aligned, into 0x15fea; 0x15fe8 tells whether they
// meet.
func (v *VM) fenetre(di uint16) {
	v.mem[dsFenOn] = 0
	x1 := v.fs16(di, spNewx)
	v.w16(dsT9a, x1)
	if x1 > v.r16(dsBlocx2) {
		return
	}
	y1 := v.fs16(di, spNewy)
	v.w16(dsT9c, y1)
	if y1 > v.r16(dsBlocy2) {
		return
	}
	x2 := v.fs16(di, spDepx)
	v.w16(dsT9e, x2)
	if x2 < v.r16(dsBlocx1) {
		return
	}
	y2 := v.fs16(di, spDepy)
	v.w16(dsTa0, y2)
	if y2 < v.r16(dsBlocy1) {
		return
	}
	x1 = max(x1, v.r16(dsBlocx1)) &^ 0xf
	y1 = max(y1, v.r16(dsBlocy1))
	x2 = min(x2, v.r16(dsBlocx2)) | 0xf
	y2 = min(y2, v.r16(dsBlocy2))
	v.w16(dsFenx1, x1)
	v.w16(dsFeny1, y1)
	v.w16(dsFenx2, x2)
	v.w16(dsFeny2, y2)
	v.w16(dsFenl, x2-x1+1)
	v.w16(dsFenh, y2-y1+1)
	v.mem[dsFenOn] = 1
}

// position is 0x587a: where di is drawn, into 0x16014..0x16026. The
// scene's matrix (or its slant) projects the element's offset from the
// scene's position, the scene's perspective divides by the depth and
// picks the reduced image, and the image is centred.
func (v *VM) position(di uint16) {
	a := v.sca(v.fsu16(di, spScreen))
	if v.mem[a+1]&2 != 0 {
		v.landPosition(a, di)
		return
	}
	v.mem[dsNewf] = v.fs8(di, spInvx)
	x := v.fs16(di, spDepx) - v.r16(a+0x16)
	y := v.fs16(di, spDepy) - v.r16(a+0x18)
	z := v.fs16(di, spDepz) - v.r16(a+0x1a)
	v.w16(dsTc2, x)
	v.w16(dsTa2, x)
	v.w16(dsTc6, y)
	v.w16(dsTa4, y)
	v.w16(dsTa6, z)
	x, y, z = v.project(a, x, y, z)
	v.w16(dsTa2, x)
	v.w16(dsTa4, y)
	v.w16(dsTa6, z)
	v.w16(dsTa0, 0)
	shift := int8(v.mem[a+0x1d])
	v.w16(dsT9a, int16(shift))
	if shift >= 0 && int8(v.fs8(di, spCredon)) >= 0 {
		px := shl32(x, uint(shift))
		v.wu16(dsTc2, uint16(px))
		v.wu16(dsTc4, uint16(px>>16))
		py := shl32(y, uint(shift))
		v.wu16(dsTc6, uint16(py))
		v.wu16(dsTc8, uint16(py>>16))
		if v.r16(dsTa6) == 0 {
			v.w16(dsTa6, 1)
		}
		v.w16(dsTa4, v.idiv16(py, v.r16(dsTa6)))
		v.w16(dsTa2, v.idiv16(px, v.r16(dsTa6)))
		v.w16(dsTa0, v.r16(dsTa6)>>(v.fs8(di, spCredon)&0x1f))
		r := v.r16(dsTa0) + int16(int8(v.fs8(di, spCreduc)))
		v.w16(dsTa0, r)
		if r < 0 {
			v.w16(dsTa0, 0)
		} else {
			m := int16(int8(v.mem[a+0x1e]))
			v.w16(dsTa8, m)
			if uint16(r) > uint16(m) {
				v.w16(dsTa0, m)
			}
			v.w16(dsTa0, v.r16(dsTa0)<<2)
		}
	}
	cl := v.mem[a+0x1c] & 0x1f
	v.w16(dsTa2, v.r16(dsTa2)>>cl+v.r16(a+0xa))
	v.w16(dsTa4, v.r16(dsTa4)>>cl+v.r16(a+0xc))

	entry := v.fsu32(di, spData) + uint32(int32(v.r16(dsTa0)))
	res := entry + v.ru32(entry)
	if v.mem[res] == 3 {
		if v.fs8(di, spInvx) != 0 {
			v.w16(dsTa2, v.r16(dsTa2)-v.r16(res+4))
		} else {
			v.w16(dsTa2, v.r16(dsTa2)+v.r16(res+4))
		}
		v.w16(dsTa4, v.r16(dsTa4)+v.r16(res+6))
		if v.mem[res+1] != 0 {
			v.mem[dsNewf] ^= 1
		}
		entry += uint32(int32(v.r16(res+2) << 2))
		res = entry + v.ru32(entry)
	}
	v.wu32(dsNewad, entry)
	w := v.ru16(res + 2)
	v.wu16(dsNewl, w)
	v.w16(dsTa2, v.r16(dsTa2)-int16(w>>1))
	h := v.ru16(res + 4)
	v.wu16(dsNewh, h)
	v.w16(dsTa4, v.r16(dsTa4)-int16(h>>1))
	v.wu16(dsNewzoomy, 0)
	v.wu16(dsNewzoomx, 0)
	v.w16(dsNewx, v.r16(dsTa2))
	v.w16(dsNewy, v.r16(dsTa4))
	v.w16(dsNewd, v.r16(dsTa6))
}

// shl32 is cwd and a shl/rcl loop: x sign extended to 32 bits, shifted.
func shl32(x int16, n uint) uint32 {
	if n >= 32 {
		return 0
	}
	return uint32(int32(x)) << n
}

// idiv16 is idiv word [0x15ea6] of dx:ax (0x5990, 0x59aa), with the
// divide error handler's retries (see xdiv).
func (v *VM) idiv16(n uint32, d int16) int16 {
	eax, _ := v.xdiv(DivSigned|DivWord|DivRM4, n>>16, n, uint32(uint16(d)))
	return int16(eax)
}

// project is 0x5b2b: by the scene's matrix if sc[1] < 0, else x stays,
// y slants by the depth (0x5b0f).
func (v *VM) project(a uint32, x, y, z int16) (int16, int16, int16) {
	m := func(i uint32) int16 { return int16(int8(v.mem[a+0x20+i])) }
	if int8(v.mem[a+1]) >= 0 {
		return x, m(4)*y - z, y
	}
	v.w16(dsT9c, x)
	v.w16(dsT9e, y)
	v.w16(dsTa0, z)
	rx := m(0)*x + m(1)*y + m(2)*z
	ry := m(3)*x + m(4)*y + m(5)*z
	rz := m(6)*x + m(7)*y + m(8)*z
	v.w16(dsTa2, rx)
	v.w16(dsT9a, rz)
	return rx, ry, rz
}

// scfollow is 0x61ed: a scene follows the position (bit 0 of +0x84) and
// angles (bit 1) of entity +0x60, plus offsets.
func (v *VM) scfollow(a uint32) {
	e := v.r16(a + 0x60)
	if e < 0 {
		return
	}
	esi := v.ru32(uint32(int32(e)) + v.ru32(0x15f72))
	if esi == 0 {
		return
	}
	set := func(off uint32, x int16) {
		if x != v.r16(a+off) {
			v.w16(a+off, x)
			v.mem[a] |= 0x80
		}
	}
	if v.mem[a+0x84]&1 != 0 {
		set(0x16, v.r16(esi)+v.r16(a+0x86))
		set(0x18, v.r16(esi+8)+v.r16(a+0x88))
		set(0x1a, v.r16(esi+0x10)+v.r16(a+0x8a))
	}
	if v.mem[a+0x84]&2 != 0 {
		set(0x34, v.r16(esi+0x18)+v.r16(esi+0x40)+v.r16(a+0x8c))
		set(0x36, v.r16(esi+0x20)+v.r16(esi+0x44)+v.r16(a+0x8e))
		set(0x38, v.r16(esi+0x28)+v.r16(esi+0x48)+v.r16(a+0x90))
	}
}

// scmove is 0x62e2: the scene moves by +0x2a..+0x2e, and all its drawn
// elements change.
func (v *VM) scmove(a uint32) {
	for i := uint32(0); i < 6; i += 2 {
		v.w16(a+0x16+i, v.r16(a+0x16+i)+v.r16(a+0x2a+i))
	}
	v.wu16(a+0x2a, 0)
	v.wu16(a+0x2c, 0)
	v.wu16(a+0x2e, 0)
	for di := v.fsu16(v.ru16(dsTa0), spLink); di != 0; di = v.fsu16(di, spLink) {
		if v.fs8(di, spState) == 0 {
			v.setFs8(di, spState, 2)
		}
	}
}
