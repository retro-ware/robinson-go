package alis

// Darkness, angles, follow/view, films and terrain walking (alis opcodes.c,
// Robinson's Requiem DOS paths).

func cscsun(v *VM)  {}
func cscaset(v *VM) {} // TODO(START.EXE): alis has no v3 cscaset (no operands)
func cscamov(v *VM) {} // TODO(START.EXE): alis has no v3 cscamov (no operands)

func cdarkpal(v *VM) {
	var p [5]int16
	for i := range p {
		v.readexecOpername()
		p[i] = int16(v.varD7)
	}
	r, g, b, first, n := p[0], p[1], p[2], p[3], p[4]
	if first >= 0x100 {
		return
	}
	length := first + n
	if 0xff < length {
		length = 0x100
	}
	for i := int(first); i < int(length); i++ {
		v.img.dkpalet[i*3+0] = r
		v.img.dkpalet[i*3+1] = g
		v.img.dkpalet[i*3+2] = b
	}
	v.img.dkpalet[0], v.img.dkpalet[1], v.img.dkpalet[2] = 0, 0, 0
	// TODO(START.EXE): alis only checks the first 0xb6 entries.
	for i := 3; i < 0xb6; i++ {
		if v.img.dkpalet[i] != 0x100 {
			v.img.fdarkpal = 1
			return
		}
	}
	v.img.fdarkpal = 0
}

func cscdark(v *VM) {
	v.flagmain = 0
	scr := v.basemain + uint32(v.curScreenID())
	v.readexecOpername()
	v.w16(scr+0x98, int16(v.varD7))
	v.readexecOpername()
	dark := v.adresdes(v.varD7)
	v.w32(scr+0x9a, v.r32(uint32(dark))+dark)
	v.readexecOpername()
	v.w16(scr+0x9e, int16(v.varD7))
	v.mem[scr] |= 0x80
	h := v.r16(uint32(v.r32(scr+0x9a))+4) + 1
	v.w16(scr+0xa0, h)
	if v.r16(scr+0x9e) < 0 {
		v.w16(scr+0x9e, h>>1)
	}
	v.basedark = uint16(v.r16(scr + 0x9e))
	v.ptrdark = uint32(v.r32(scr+0x9a)) + 8
}

func caset(v *VM) {
	vram := v.script.vramOrg
	for _, o := range []uint32{scrWCAX, scrWCAY, scrWCAZ} {
		v.readexecOpername()
		v.w16(vram+o, int16(v.varD7))
	}
}

func camov(v *VM) {
	vram := v.script.vramOrg
	for _, o := range []uint32{scrWCAX, scrWCAY, scrWCAZ} {
		v.readexecOpername()
		v.add16(vram+o, int16(v.varD7))
	}
}

func cscfollow(v *VM) {
	scr := v.basemain + uint32(v.curScreenID())
	v.readexecOpername()
	v.w16(scr+0x60, int16(v.varD7))
	v.readexecOpername()
	v.mem[scr+0x84] = uint8(v.varD7)
	for o := uint32(0x86); o <= 0x90; o += 2 {
		v.readexecOpername()
		v.w16(scr+o, int16(v.varD7))
	}
}

func cscview(v *VM) {
	scr := v.basemain + uint32(v.curScreenID())
	for o := uint32(0x64); o <= 0x6e; o += 2 {
		v.readexecOpername()
		v.w16(scr+o, int16(v.varD7))
	}
	v.mem[scr] |= 0x80
}

func cfilm(v *VM) {
	v.filmStreamClose()
	v.film = film{}
	v.filmnode = 0
	v.readexecOpername()
	v.film.id = int16(v.varD7)
	v.readexecOpername()
	addr := uint32(v.adresdes(v.varD7)) + 0x20 + 4
	v.film.buf = v.mem
	v.film.addr1 = int(addr)
	v.film.addr2 = int(addr)
	v.readexecOpername()
	v.film.waitclock = int16(v.varD7)
	v.film.basemain = v.basemain
	v.inifilm()
	v.varD7 = int32(v.film.frames)
	v.cstoreContinue()
}

// headland turns the view pitch towards d4w, at most 15° per call.
func (v *VM) headland(d4w int16) {
	a := v.script.vramOrg + scrWCAX
	n := d4w - v.r16(a)
	if n == 0 {
		return
	}
	n >>= 2
	if n < 0 {
		if n < -0xf {
			n = -0xf
		}
	} else {
		if n == 0 {
			n = 1
		}
		if 0xf < n {
			n = 0xf
		}
	}
	n += v.r16(a)
	if n < 0x4c {
		if n < -0x4b {
			n = -0x4b
		}
	} else {
		n = 0x4b
	}
	v.w16(a, n)
}

// walkState holds the walkland globals.
type walkState struct {
	cx, cy  int32
	cz, ocz int16
	dx, dy  int32
	haut    int16
	map3    uint8
	dim     uint16
	tete    int16
	// stand-ins for the C locals landalti may leave unwritten
	probeZ, sampleZ int16
}

func hi16(x int32) int16 { return int16(uint32(x) >> 16) }

// walkland moves the entity over the terrain of scene and returns the
// height change.
// TODO(START.EXE): the probes read uninitialised locals when landalti
// misses the map.
func (v *VM) walkland(scene uint32) int16 {
	w := &v.walk
	vram := v.script.vramOrg
	v.matmask = 0
	if w.tete < 0 {
		v.matmask = w.tete
	}
	az := int32(v.r16(vram + scrWCAZ))
	sin := int32(tabsin(az))
	cos := int32(tabcos(az))
	ox := int32(int8(v.mem[vram+scrWCX2]))
	oy := int32(int8(v.mem[vram+scrWCY2]))
	if w.tete != 0 {
		w.dx = -int32(v.r16(scene-0x3f4)) * sin * 0x80
		w.dy = int32(v.r16(scene-0x3f4)) * cos * 0x80
	}
	wx := v.r32wc(vram, scrWCX)
	wy := v.r32wc(vram, scrWCY)
	w.cx = wx + (cos*ox-sin*oy)*0x80
	w.cy = wy + (cos*oy+sin*ox)*0x80
	w.ocz = v.r16(vram + scrWCZ)
	var cell int16
	v.landalti(scene, vram, int16(w.cx>>16), int16(w.cy>>16), &w.cz, &cell)
	dz := w.cz - w.ocz
	if v.matmask != 0 {
		probe := func(x, y int32) {
			v.landalti(scene, vram, hi16(x), hi16(y), &w.probeZ, &cell)
			if 0xfe < w.probeZ {
				dz = w.probeZ - w.ocz
				w.cz = w.probeZ
			}
		}
		probe(wx+w.dy+w.dx, wy+w.dx+w.dy)
		probe(wx+(w.dx-w.dy), wy+(w.dy-w.dx))
		probe(wx+w.dx*2, wy+w.dy*2)
		probe(wx+w.dy+w.dx*2, wy+w.dx+w.dy*2)
		probe(wx+(w.dx*2-w.dy), wy+(w.dy*2-w.dx))
	}
	if 0 < w.tete {
		ad := dz
		if dz < 0 {
			ad = -dz
		}
		if ad <= w.haut {
			v.landalti(scene, vram, hi16(wx+w.dx), hi16(wy+w.dy), &w.sampleZ, &cell)
			w.sampleZ = (w.sampleZ - w.ocz) * 4
			w1 := w.sampleZ
			v.landalti(scene, vram, hi16(wx+w.dx*2), hi16(wy+w.dy*2), &w.sampleZ, &cell)
			w.sampleZ = (w.sampleZ - w.ocz) * 2
			v.landalti(scene, vram, hi16(wx+w.dx*4), hi16(wy+w.dy*4), &w.sampleZ, &cell)
			slope := uint16((w.sampleZ - w.ocz) + w1 + w.sampleZ)
			as := slope
			if int16(slope) < 0 {
				as = -slope
			}
			if 0x3c0 < as {
				slope = 0
			}
			wt := int32(int16(uint16(w.tete) >> 8))
			w.sampleZ = int16((int32(int16(slope)) * wt >> 2) / 0xc)
			ah := w.sampleZ
			if w.sampleZ < 0 {
				ah = -w.sampleZ
			}
			if uint8(ah) < w.map3 {
				w.sampleZ = 0
			}
			v.headland(w.sampleZ)
		}
	}
	adz := uint16(w.cz - w.ocz)
	if int32(uint32(adz)<<16) < 0 {
		adz = -adz
	}
	if w.haut < int16(adz) {
		v.landalti(scene, vram, v.r16(vram+scrWCX), v.r16(vram+scrWCY), &w.probeZ, &cell)
		v.w16(vram+scrWCZ, w.probeZ)
	} else {
		v.w32wc(vram, scrWCX, w.cx)
		v.w32wc(vram, scrWCY, w.cy)
		v.w16(vram+scrWCZ, w.cz)
	}
	return w.cz - w.ocz
}

func cwalkmap(v *VM) {
	scene := v.readVaddr()
	v.readexecOpername()
	v.walk.haut = int16(v.varD7)
	v.readexecOpername()
	v.walk.tete = int16(v.varD7)
	v.readexecOpernameSaveD6()
	v.walk.dim = uint16(v.varD6)
	v.varD7 = int32(v.walkland(scene))
	v.cstoreContinue()
}

type vector struct{ x, y, z int32 }

// polarmov turns a local move by the yaw and pitch (DOS: only the y terms
// are scaled by 0x80).
func polarmov(wcx2, wcy2, wcz2, wcax, wcaz int16) vector {
	sinax := int32(tabsin(int32(wcax)))
	sinaz := int32(tabsin(int32(wcaz)))
	cosax := int32(tabcos(int32(wcax)))
	cosaz := int32(tabcos(int32(wcaz)))
	return vector{
		x: cosaz*int32(wcx2) - int32(int16(sinaz*cosax>>9))*int32(wcy2)*0x80,
		y: int32(int16(cosaz*cosax>>9))*int32(wcy2)*0x80 + sinaz*int32(wcx2),
		z: (int32(wcy2)*sinax + int32(wcz2)*cosax) * 0x80,
	}
}

// landalti reads the terrain cell under (x, y) of the map at addr; it
// leaves alt and cell alone outside the map.
func (v *VM) landalti(addr, vram uint32, x, y int16, alt, cell *int16) {
	sh := uint16(v.r16(addr-0x3c0)) & 0x3f
	gx := uint16(x >> sh)
	gy := uint16(y >> sh)
	if gy > v.ru16(addr-0x292) || gx > v.ru16(addr-0x294) {
		return
	}
	ca := uint32(int32(int16(gy))*2 + v.r32(uint32(int32(v.img.atlland)+int32(int16(gx*4)))))
	*cell = v.r16(ca)
	*alt = *cell & 0xff
	to := int32(((*cell & 0x3f00) >> 3) - 0xc00)
	at := func(o uint32) uint32 { return uint32(int32(addr+o) + to) }
	flag := int8(v.mem[at(0x14)])
	if flag == 0 {
		return
	}
	if flag < 0 {
		ca += uint32(v.r32(at(0x10)))
		diff := (v.r16(ca) & 0xff) - *alt
		t := v.r16(at(0x16)) - diff
		if diff <= t || (v.r16(ca)&0xff)-t <= v.r16(vram+scrWCZ) {
			*cell = v.r16(ca)
			*alt = *cell & 0xff
		}
		return
	}
	// flag 1 with a texture: alis/DOS add 0xff to the pointer, not the value
	if v.matmask != 0 && v.r16(at(0x1a)) != 0 && flag != 1 {
		*alt += v.r16(at(0x16))
	}
}

func catstmap(v *VM) {
	addr := v.readVaddr()
	v.readexecOpername()
	wcx2 := int16(v.varD7)
	v.readexecOpername()
	wcy2 := int16(v.varD7)
	v.readexecOpernameSaveD6()
	wcz2 := int16(v.varD6)
	vram := v.script.vramOrg
	vec := polarmov(wcx2, wcy2, wcz2, v.r16(vram+scrWCAX), v.r16(vram+scrWCAZ))
	v.matmask = 0
	var alt int16
	t7 := int16(v.varD7)
	v.landalti(addr, vram, hi16(v.r32wc(vram, scrWCX)+vec.x), hi16(v.r32wc(vram, scrWCY)+vec.y), &alt, &t7)
	v.varD7 = int32(t7)
	v.cstoreContinue()
}

func (v *VM) polarVec() vector {
	vram := v.script.vramOrg
	return polarmov(int16(int8(v.mem[vram+scrWCX2])), int16(int8(v.mem[vram+scrWCY2])),
		int16(int8(v.mem[vram+scrWCZ2])), v.r16(vram+scrWCAX), v.r16(vram+scrWCAZ))
}

func cavtstmov(v *VM) {
	vram := v.script.vramOrg
	vec := v.polarVec()
	v.wcx = int16((v.r32wc(vram, scrWCX) + vec.x) >> 16)
	v.wcy = int16((v.r32wc(vram, scrWCY) + vec.y) >> 16)
	v.wcz = int16((v.r32wc(vram, scrWCZ) + vec.z) >> 16)
	v.readexecOpername()
	v.matmask = int16(v.varD7)
	v.wforme = v.cforme(vram)
	v.clipform()
	crstent(v)
}

func cavmov(v *VM) {
	vram := v.script.vramOrg
	vec := v.polarVec()
	v.add16(vram+scrWCX, int16(vec.x>>16))
	v.add16(vram+scrWCY, int16(vec.y>>16))
	v.add16(vram+scrWCZ, int16(vec.z>>16))
}

// getangle turns (x, y) into (angle, length).
func getangle(atx, aty *int16) {
	var angle int16
	mag := *aty
	x, y := *atx, *aty
	if x == 0 {
		if y == 0 {
			mag = 0
		} else if y < 0 {
			angle = 0xb4
		}
	} else if y == 0 {
		mag = x
		angle = 0x5a
		if x >= 0 {
			angle = -0x5a
		}
	} else {
		ax, ay := x, y
		if ax < 0 {
			ax = -ax
		}
		if ay < 0 {
			ay = -ay
		}
		angle = 0x5a
		if ay < ax {
			r := int16((int32(ax) << 5) / int32(ay))
			if uint16(r) < uint16(len(tabatan)) {
				angle = int16(tabatan[r])
			}
			if uint16(r) >= uint16(len(tabatan)) || angle != 0 {
				if y < 0 {
					angle = 0xb4 - angle
				}
				if -1 < x {
					angle = -angle
				}
				mag = int16((int32(x) * 0x200) / int32(tabsin(int32(angle))))
			} else {
				angle = 0
				if y < 0 {
					angle = 0xb4
				}
			}
		} else {
			r := int16((int32(ay) << 5) / int32(ax))
			if uint16(r) < uint16(len(tabatan)) {
				angle = int16(tabatan[r])
			}
			if uint16(r) >= uint16(len(tabatan)) || angle != 0 {
				if y < 0 {
					angle = -angle
				}
				angle -= 0x5a
				if x < 0 {
					angle = -angle
				}
				mag = int16((int32(y) * 0x200) / int32(tabcos(int32(angle))))
			} else {
				mag = x
				angle = 0x5a
				if x >= 0 {
					angle = -0x5a
				}
			}
		}
	}
	if mag < 0 {
		mag = -mag
	}
	*atx = angle
	*aty = mag
}

func polarang(cx, cy, cz *int16, x, y, z int16) {
	*cx -= x
	*cy -= y
	*cz -= z
	getangle(cx, cy)
	var elev int16
	if *cz != 0 {
		elev = -*cz
		getangle(&elev, cy)
	}
	*cz = *cy
	*cy = elev
}

func caim(v *VM) {
	vram := v.script.vramOrg
	v.readexecOpername()
	cx := int16(v.varD7)
	v.readexecOpername()
	cy := int16(v.varD7)
	v.readexecOpername()
	cz := int16(v.varD7)
	polarang(&cx, &cy, &cz, v.r16(vram+scrWCX), v.r16(vram+scrWCY), v.r16(vram+scrWCZ))
	if cz == 0 {
		cx = v.r16(vram + scrWCAZ)
	}
	v.readexecOpername()
	maxTurn := uint16(v.varD7)
	if 0xb3 >= maxTurn {
		cy = v.r16(vram + scrWCAZ)
		d := uint16(cx - cy)
		switch {
		case int16(d) < 0 && int16(d) < -0xb3:
			d += 0x168
			if int16(d) < 0 {
				if int16(d) <= int16(-maxTurn) {
					d = -maxTurn
				}
				cx = cy + int16(d)
			}
			if int16(maxTurn) <= int16(d) {
				d = maxTurn
			}
			cx = cy + int16(d)
		case int16(d) < 0:
			if int16(d) <= int16(-maxTurn) {
				d = -maxTurn
			}
			cx = cy + int16(d)
		case 0xb3 < int16(d):
			d -= 0x168
			if int16(d) < 0 {
				if int16(d) <= int16(-maxTurn) {
					d = -maxTurn
				}
				cx = cy + int16(d)
			}
		default:
			if int16(maxTurn) <= int16(d) {
				d = maxTurn
			}
			cx = cy + int16(d)
		}
	}
	if cx < 0x168 {
		if cx < -0x167 {
			cx += 0x168
		}
	} else {
		cx -= 0x168
	}
	v.varD7 = int32(cy)
	v.cstoreContinue()
	v.varD7 = int32(cx)
	v.cstoreContinue()
	v.varD7 = int32(cz)
	v.cstoreContinue()
}

func cpointpix(v *VM) {
	v.readexecOpername()
	v.readexecOpernameSaveD6()
	v.pointpix(int16(v.varD7), int16(v.varD6))
}
