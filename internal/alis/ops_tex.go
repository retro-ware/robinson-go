package alis

// Terrain texture table and dynamic tables.

// texmasks are the row masks a texture's height is cut to (START.EXE
// 0x1620d; the original has 3 where 63 belongs).
var texmasks = [15]uint16{0, 1, 3, 7, 15, 31, 3, 127, 255, 511, 1023, 2047, 4095, 8191, 16383}

// ctexmap defines texture slot D6 of a scene: 0x20-byte entries 0xc00
// bytes below the scene (START.EXE 0x4c50).
func ctexmap(v *VM) {
	v.mem[dsFlagmain] = 0
	scene := v.readVaddr()
	v.readexecOpernameSaveD6()
	entry := uint32(int32(scene) + int32(int16(uint16(v.varD6)*0x20+0xf400)))
	v.readexecOpername()
	v.mem[entry+8] = uint8(v.varD7)
	v.readexecOpername()
	if exists := int8(v.mem[entry+8]); exists > 0 {
		res := v.adresdes(v.varD7)
		v.w32(entry+4, res)
		if res != 0 { // the original would read at 0
			v.texOrigin(entry, uint32(res)+uint32(v.r32(uint32(res))))
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
	case typ < 0:
		v.readexecOpername()
		v.w16(entry+0x10, int16(v.varD7))
		v.readexecOpername()
		v.w16(entry+0x12, int16(v.varD7))
		v.readexecOpername()
		v.w16(entry+0x16, int16(v.varD7))
		v.readexecOpername()
		v.w16(entry+0x1a, int16(v.varD7))
		// 0x4dea: the texture's first cell in the map
		top := int32(v.r16(entry+0x10))*int32(v.r16(scene-0x3ca)) + int32(v.r16(entry+0x12))
		v.w32(entry+0x10, top*2)
	case typ == 2:
		v.readexecOpername()
		v.w16(entry+0x16, int16(v.varD7))
		v.readexecOpername()
		v.readexecOpername()
		v.readexecOpername()
		v.w16(entry+0x1a, int16(v.varD7))
	default:
		v.readexecOpername()
		if typ == 0 {
			v.w32(entry+0x10, 0)
		} else {
			v.w32(entry+0x10, v.adresdes(v.varD7))
		}
		v.readexecOpername()
		v.w16(entry+0x16, int16(v.varD7))
		v.readexecOpername()
		v.mem[entry+0x15] = uint8(v.varD7)
		v.readexecOpername()
		v.w16(entry+0x1a, int16(v.varD7))
		v.w16(entry+0x1c, v.r16(v.script.vramOrg-0xe))
	}
}

// texOrigin stores the texture's row shift and its first row without a
// clear pixel, and cuts the height below it to a mask (START.EXE
// 0x4c99-0x4d16). The row scan counts columns in a byte (dec cl; jns) and
// steps over that many pixels from offset 8 whatever the format.
func (v *VM) texOrigin(entry, bm uint32) {
	var yshift uint8
	for t := v.ru16(bm + 2); ; {
		yshift++
		if t >>= 1; t == 0 {
			break
		}
	}
	v.mem[entry] = yshift
	v.w16(entry+2, 0)
	ymask := v.r16(bm + 4)
	p := bm + 8
	row := ymask
	for {
		zero := false
		for c := v.mem[bm+2]; ; {
			if v.mem[p] == 0 {
				zero = true
			}
			p++
			if c--; int8(c) < 0 {
				break
			}
		}
		if !zero {
			break
		}
		if row--; row < 0 {
			return
		}
	}
	yorigin := ymask - row
	v.w16(entry+2, yorigin)
	if yorigin == 0 {
		return
	}
	avail := uint16(ymask - yorigin)
	i := 0
	for ; i < len(texmasks); i++ {
		if avail == texmasks[i] {
			return
		}
		if avail < texmasks[i] {
			break
		}
	}
	v.wu16(bm+4, texmasks[i-1]+uint16(yorigin))
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
