package alis

// Collision chart maps, sky and zoom (alis opcodes.c, DOS paths).

func cchartmap(v *VM) {
	addr := v.readVaddr()
	v.readexecOpername()
	var fill int16
	switch value := int16(v.varD7); value {
	case 0:
	case 1:
		fill = -1
	case 2, 3:
		v.readexecOpername()
		v.w16(addr-0x3b6, int16(v.varD7))
		return
	case 4:
		v.readexecOpername()
		v.w16(addr-0x3ae, int16(v.varD7))
		return
	case 5:
		v.readexecOpername()
		v.w16(addr-0x3b0, int16(v.varD7))
		return
	default:
		v.readexecOpername()
		v.chartClear(addr, int16(v.varD7)-1)
		return
	}
	loops := int16(v.r16(addr-0x3c6)*v.r16(addr-0x3c4)) >> 1
	p := addr
	if int8(v.mem[p-1]) < 0 {
		p = v.ru32(p)
		p = v.ru32(p)
	}
	for i := int16(0); i < loops; i, p = i+1, p+2 {
		v.w16(p, fill)
	}
	v.w32(addr-0x3ba, int32(v.readVaddr()))
}

// chartClear is 0x4f14: it clears the chart bits covered by a square of the
// given radius around the script's position (es:[ebp], es:[ebp+8]).
func (v *VM) chartClear(addr uint32, radius int16) {
	if radius < 0 {
		radius = 0
	}
	ed := v.ru32(addr - 0x3ba)
	cellW := int32(v.r16(ed - 0x3f4))
	cellH := int32(v.r16(ed - 0x3f2))
	maxCols := v.r16(ed - 0x3e8)
	maxRows := v.r16(ed - 0x3e6)
	vram := v.script.vramOrg
	x, y := v.r16(vram+scrWCX), v.r16(vram+scrWCY)
	clamp := func(d int32, max int16) int16 {
		r := int16(d)
		if d < 0 {
			r = 0
		}
		if max <= r {
			r = max
		}
		return r
	}
	minCol := clamp(int32(x-radius)/cellW, maxCols)
	maxCol := clamp(int32(x+radius)/cellW, maxCols)
	minRow := clamp(int32(y-radius)/cellH, maxRows)
	maxRow := clamp(int32(y+radius)/cellH, maxRows)
	shx := uint16((v.r16(addr-0x3c0) - 4) - v.r16(ed-0x3c0))
	minX := uint16(minCol >> (shx & 0x3f))
	maxX := uint16(maxCol >> (shx & 0x3f))
	shy := uint16(v.r16(addr-0x3be) - v.r16(ed-0x3be))
	startRow := minRow >> (shy & 0x3f)
	rowCount := (maxRow >> (shy & 0x3f)) - startRow
	left := ^uint16(0xffff >> (minX & 0xf))
	aligned := maxX | 0xf
	right := ^uint16(0xffff << ((aligned - maxX) & 0x3f))
	span := (aligned - minX&0xfff0) >> 4
	mid := int16(span) - 1
	if mid < 0 {
		left |= right
		right = 0
	}
	if right != 0 {
		mid = int16(span) - 2
	}
	base := addr
	if int8(v.mem[addr-1]) < 0 {
		base = v.ru32(v.ru32(addr))
	}
	stride := v.r16(addr - 0x3c4)
	row := uint32(int32(base)+int32(startRow)*2) + uint32(minX>>4)*uint32(uint16(stride))
	for {
		v.wu16(row, left&v.ru16(row))
		p := uint32(int32(row) + int32(stride))
		for n := mid; n > -1; n-- {
			v.w16(p, 0)
			p = uint32(int32(p) + int32(stride))
		}
		if right != 0 {
			v.wu16(p, right&v.ru16(p))
		}
		row += 2
		rowCount--
		if rowCount == -1 {
			break
		}
	}
}

func czoom(v *VM) {
	v.readexecOpername()
	v.w16(v.script.vramOrg-0x36, int16(v.varD7))
	v.readexecOpername()
	v.w16(v.script.vramOrg-0x38, int16(v.varD7))
}
