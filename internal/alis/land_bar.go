package alis

// Terrain bar drawing of the RRQ landscape (alis render/rrq3d_dos.c,
// bartra_dos and barlands).

// aliasStep reads the slope step table for a bar of dist rows and width
// columns: near slopes index it directly, far ones at quarter resolution.
func (v *VM) aliasStep(dist, width uint16) int16 {
	if dist < 0x41 {
		return v.r16(v.img.atalias + uint32(uint16((dist-1)*0x40+width-1))*2)
	}
	dist >>= 2
	if dist > 0x40 {
		dist = 0x40
	}
	return v.r16(v.img.atalias+uint32(uint16((dist-1)*0x40+width>>1-1))*2) >> 1
}

func (v *VM) lpix(y int16) uint32 {
	return v.ru32(v.img.atlpix + uint32(int32(y-v.img.wlogy1)*4))
}

// texRow draws count texels of one scanline; dir is 1 (left to right,
// dst is the first pixel) or -1 (right to left, dst is one past the last).
func (v *VM) texRow(dst uint32, vc, vmask uint16, h, step uint16, wmask uint8, tex, dark uint32, count uint8, dir int, transp bool) {
	vm := vc & vmask
	for ; count > 0; count-- {
		hm := uint8(h>>8) & wmask
		idx := vm&0xff00 | uint16(uint8(vm)|hm)
		t := v.mem[tex+uint32(idx)]
		if dir < 0 {
			dst--
		}
		if t != 0 || !transp {
			v.mem[dst] = v.mem[v.ptrdark+dark+uint32(t)]
		}
		if dir > 0 {
			dst++
			h += step
		} else {
			h -= step
		}
	}
}

func (v *VM) bartra(cellAddr, rc uint32, drawy uint16, index, barwidth, barheight, bary int16) {
	if barheight <= 0 || barwidth <= 0 {
		return
	}
	img, ld := &v.img, &v.land
	precx := uint32(int32(img.precx))
	stride := uint32(uint16(img.wloglarg))
	widthB := uint8(barwidth)
	hstep := uint16(img.texPerspStep)
	ix := rc + uint32(int32(index))
	shift := v.mem[ix] & 0x1f
	vstep := hstep>>((8-uint16(v.mem[ix]))&0x1f) + 1
	hbase0 := uint16(img.texHbase)

	slopeClip := v.r16(rc - 0x246)
	sy := int16(drawy)
	slopeH := slopeClip - sy
	slopeHeight := uint16(slopeH)
	slopeRows := uint8(slopeH)

	texPtr := v.ru32(ix + 4)
	if texPtr == 0 {
		return
	}
	texOff := v.ru32(texPtr)
	wmask := v.mem[texPtr+texOff+2]
	texH := v.r16(texPtr + texOff + 4)
	subtile := v.ru16(ix + 2)
	texBase := texPtr + texOff

	cell := v.ru16(cellAddr)
	dp := int16(uint8(img.vdarkw>>8)) - int16(uint8(cell>>8)>>6)*2
	if int16(uint16(dp)<<8) < 0 {
		dp = 0
	}
	dark := uint32(uint8(dp)) << 8

	var vc, vmask uint16
	var tex uint32
	remaining := barheight

	if slopeH == 0 || slopeClip < sy {
		vmask = uint16(texH-int16(subtile)) << shift
		tex = texBase + uint32(subtile)<<shift + 8
	} else if subtile == 0 {
		remaining = barheight - int16(slopeHeight)
		if barheight < int16(slopeHeight) {
			slopeRows = uint8(int8(slopeRows) + int8(remaining))
		}
		step := v.aliasStep(uint16(slopeClip-bary), uint16(barwidth))
		colW := uint8(1)
		if bary < sy {
			vc = uint16(sy-bary) * vstep
			colW = uint8(uint16(sy-bary)*uint16(step)>>8) + 1
		}
		vmask = uint16(texH) << shift
		tex = texBase + 8
		scr := v.lpix(sy) + precx
		frac := uint16(colW) << 8
		dir, h := 1, hbase0
		if slopeClip != v.r16(rc-0x25c) {
			dir, h = -1, hbase0+0x2000
			scr += uint32(widthB)
		}
		for rows := slopeRows; ; {
			v.texRow(scr, vc, vmask, h, hstep, wmask, tex, dark, uint8(frac>>8), dir, false)
			vc += vstep
			frac += uint16(step)
			scr += stride
			rows--
			if rows == 0 {
				break
			}
		}
		if remaining < 1 {
			return
		}
	} else {
		remaining = barheight - int16(slopeHeight)
		if barheight < int16(slopeHeight) {
			slopeHeight += uint16(remaining)
		}
		vmask = uint16(texH-int16(subtile)) << shift
		subOff := uint32(subtile) << shift
		tex = texBase + 8
		if slopeClip == bary-ld.savedSI {
			if bary < sy {
				vc = uint16(sy-bary) * vstep
			}
			scr := v.lpix(sy) + precx
			for rows := uint8(slopeHeight); ; {
				v.texRow(scr, vc, vmask, hbase0, hstep, wmask, tex, dark, widthB, 1, true)
				vc += vstep
				scr += stride
				rows--
				if rows == 0 {
					break
				}
			}
			if remaining < 1 {
				return
			}
			tex += subOff
			vc = 0
		} else {
			step := v.aliasStep2(uint16(slopeClip-bary+ld.savedSI), uint16(barwidth))
			delta := uint32(int32(uint8(uint16(step)>>8)) * int32(img.wloglarg))
			var vstart uint16
			if bary < sy {
				vstart = uint16(sy-bary) * vstep
			}
			di := int32(sy - img.wlogy1)
			end := v.ru32(img.atlpix+uint32((int32(int16(uint8(slopeHeight)))+di)*4)) + precx
			start := v.ru32(img.atlpix+uint32(di*4)) + precx
			rtl := slopeClip != v.r16(rc-0x25c)
			hacc := uint32(hbase0)
			if rtl {
				start += uint32(widthB)
				hacc = uint32(hbase0 + 0x2000)
			}
			add := uint32(uint8(step))<<24 | uint32(hstep)
			for cols := widthB; ; {
				hm := uint8(uint16(hacc)>>8) & wmask
				hs := hacc&0xffff0000 | uint32(hm)<<8 | hacc&0xff
				vv := vstart
				for cur := start; cur < end; cur += stride {
					vm := vv & vmask
					if t := v.mem[tex+uint32(vm&0xff00|uint16(uint8(vm)|hm))]; t != 0 {
						v.mem[cur] = t
					}
					vv += vstep
				}
				var nh uint32
				if rtl {
					nh = hs - add
					if hs < add {
						start += stride
					}
				} else {
					nh = hs + add
					if nh < hs {
						start += stride
					}
				}
				if end <= start+delta {
					break
				}
				if rtl {
					start += delta - 1
				} else {
					start += delta + 1
				}
				hacc = nh
				cols--
				if cols == 0 {
					break
				}
			}
			if remaining < 1 {
				return
			}
			tex += subOff
			vc = 0
		}
	}

	if mid := remaining - img.vbarbot; mid > 0 {
		scr := v.lpix(sy+barheight-remaining) + precx
		for mr := uint8(mid); ; {
			v.texRow(scr, vc, vmask, hbase0, hstep, wmask, tex, dark, widthB, 1, false)
			vc += vstep
			scr += stride
			mr--
			if mr == 0 {
				break
			}
		}
	}

	if img.vbarbot <= 0 {
		return
	}
	bd := ld.botalt - ld.bothigh
	if bd == 0 || ld.bothigh > ld.botalt {
		return
	}
	bstep := v.aliasStep(bd, uint16(widthB))
	bix := rc + uint32(int32(ld.bottomType))
	bptr := v.ru32(bix + 4)
	if bptr == 0 {
		return
	}
	boff := v.ru32(bptr)
	bwmask := v.mem[bptr+boff+2]
	bvmask := v.ru16(bptr+boff+4) << (v.mem[bix] & 0x1f)
	btex := bptr + boff + 8
	bdk := uint8(img.vdarkw>>8) - uint8(cell>>8)>>6*2
	if int16(uint16(bdk)<<8) < 0 {
		bdk = 0
	}
	bdark := uint32(bdk) << 8
	scr := v.lpix(int16(ld.bothigh)) + precx
	frac := uint16(widthB)<<8 | 0xff
	dir, h := 1, hbase0
	if ld.botalt == ld.precboti {
		dir, h = -1, hbase0+0x2000
		scr += uint32(widthB)
	}
	var bv uint16
	for rows := uint8(img.vbarbot); ; {
		v.texRow(scr, bv, bvmask, h, hstep, bwmask, btex, bdark, uint8(frac>>8), dir, false)
		bv += vstep
		frac -= uint16(bstep)
		scr += stride
		rows--
		if rows == 0 {
			break
		}
	}
}

// aliasStep2 is the step lookup of the perspective subtile slope, which
// indexes the table transposed.
func (v *VM) aliasStep2(dist, width uint16) int16 {
	if dist < 0x41 {
		return v.r16(v.img.atalias + uint32(uint16(dist+(width-1)*0x40-1))*2)
	}
	dist >>= 2
	if dist > 0x40 {
		dist = 0x40
	}
	return v.r16(v.img.atalias+uint32(uint16(dist+((width-1)*0x40)>>1-1))*2) >> 1
}

func (v *VM) barlands(drawy uint16, barheight, barwidth int16) {
	if barheight == 0 {
		return
	}
	img := &v.img
	fill := v.land.fillColor&0x0f + 0x80
	dst := v.ru32(img.atlpix+uint32(uint16(int16(drawy)-img.wlogy1))*4) + uint32(int32(img.precx))
	for h := barheight; h != 0; h-- {
		for i := uint32(0); i < uint32(uint16(barwidth)); i++ {
			v.mem[dst+i] = fill
		}
		dst += uint32(uint16(img.wloglarg))
	}
}
