package alis

// Terrain texture table and dynamic tables (alis opcodes.c).

var pow2m1 = [...]uint16{1, 3, 7, 15, 31, 63, 127, 255, 511, 1023, 2047, 4095, 8191, 16383, 32767}

// ctexmap defines texture slot D6 of a scene: 0x20-byte entries 0xc00
// bytes below the scene.
// TODO(START.EXE): check the opaque-row scan and the y_mask snap.
func ctexmap(v *VM) {
	v.flagmain = 0
	scene := v.readVaddr()
	v.readexecOpernameSaveD6()
	entry := uint32(int32(scene) + v.varD6*0x20 - 0xc00)
	v.readexecOpername()
	v.mem[entry+8] = uint8(v.varD7)
	v.readexecOpername()
	if exists := int8(v.mem[entry+8]); exists > 0 {
		v.w32(entry+4, v.adresdes(v.varD7))
		if res := uint32(v.r32(entry + 4)); res != 0 {
			v.texOrigin(entry, res+uint32(v.r32(res)))
		}
	} else if exists == 0 {
		v.w32(entry+4, 0)
		v.mem[entry] = 0
		v.w16(entry+2, 0)
	}
	v.readexecOpername()
	v.w16(entry+10, int16(v.varD7))
	v.readexecOpername()
	v.mem[entry+9] = uint8(v.varD7)
	v.readexecOpername()
	v.w16(entry+0xe, int16(v.varD7))
	v.readexecOpername()
	typ := int8(v.varD7)
	v.mem[entry+0x14] = uint8(typ)
	switch {
	case typ == 0:
		for range 4 {
			v.readexecOpername()
		}
	case typ < 0:
		v.readexecOpername()
		v.w16(entry+0x10, int16(v.varD7))
		v.readexecOpername()
		v.w16(entry+0x12, int16(v.varD7))
		v.readexecOpername()
		v.w16(entry+0x16, int16(v.varD7))
		v.readexecOpername()
		v.w16(entry+0x1a, int16(v.varD7))
		off := v.calctop(scene, v.r16(entry+0x10), v.r16(entry+0x12))
		v.w32(entry+0x10, off)
		if off < 0 {
			v.mem[entry+0x14] = 0
		}
	case typ == 2:
		v.readexecOpername()
		v.w16(entry+0x16, int16(v.varD7))
		v.readexecOpername()
		v.readexecOpername()
		v.readexecOpername()
		v.w16(entry+0x1a, int16(v.varD7))
	default:
		v.readexecOpername()
		v.w32(entry+0x10, v.adresdes(v.varD7))
		v.w16(entry+0x1c, v.r16(v.script.vramOrg-0xe))
		v.readexecOpername()
		v.w16(entry+0x16, int16(v.varD7))
		v.readexecOpername()
		v.mem[entry+0x15] = uint8(v.varD7)
		v.readexecOpername()
		v.w16(entry+0x1a, int16(v.varD7))
	}
}

// texOrigin stores the texture's row shift and its first fully opaque row.
func (v *VM) texOrigin(entry, bm uint32) {
	pixOff := uint32(6)
	switch v.mem[bm] {
	case 0x1c, 0x1e, 0x14, 0x16:
		pixOff = 8
	}
	xmask := v.ru16(bm + 2)
	var yshift uint8
	for t := xmask; ; {
		yshift++
		t >>= 1
		if t == 0 {
			break
		}
	}
	v.mem[entry] = yshift
	v.w16(entry+2, 0)
	ymask := v.r16(bm + 4)
	p := bm + pixOff
	rows := ymask
	found := false
	for rows >= 0 {
		zero := false
		for cols := int16(xmask); cols >= 0; cols-- {
			if v.mem[p] == 0 {
				zero = true
			}
			p++
		}
		if !zero {
			found = true
			break
		}
		rows--
	}
	if !found {
		return
	}
	yorigin := ymask - rows
	v.w16(entry+2, yorigin)
	if yorigin == 0 {
		return
	}
	avail := uint16(ymask - yorigin)
	for i, p2 := range pow2m1 {
		if p2 == avail {
			break
		}
		if p2 > avail {
			if i > 0 {
				v.wu16(bm+4, pow2m1[i-1]+uint16(yorigin))
			}
			break
		}
	}
}

func calloctab(v *VM) {
	cell := v.readVaddr()
	if int8(v.mem[cell-1]) >= 0 || v.r32(cell) != 0 {
		return
	}
	size := v.r32(cell - 6 - uint32(v.mem[cell-1]&0xf)*4)
	v.w32(cell, v.ioMalloc(size))
}

func cfreetab(v *VM) {
	cell := v.readVaddr()
	h := v.r32(cell)
	if int8(v.mem[cell-1]) >= 0 || h == 0 {
		return
	}
	v.w32(cell, 0)
	v.ioMfree(h)
}
