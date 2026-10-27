package alis

import "math/bits"

// Tile maps (START.EXE 0x493b..0x4c46). A map is an array variable; its
// header lies in the 0x400 bytes below the data.

// mapvar is the variable an opcode names: a word offset into the running
// entity's variables, or after a zero word into the main script's.
func (v *VM) mapvar() uint32 {
	base := v.script.vramOrg
	off := v.read16()
	if off == 0 {
		base = v.basemain
		off = v.read16()
	}
	return base + uint32(int32(off))
}

// log2w is the bit counting loops of cdefmap: the highest set bit, 0 for 0.
func log2w(x uint16) uint16 {
	if x == 0 {
		return 0
	}
	return uint16(bits.Len16(x) - 1)
}

// cdefmap (0x493b) sets up the map header; the word at -0x3b6 is cleared
// relative to the running entity, not to the map.
func cdefmap(v *VM) {
	m := v.mapvar()
	v.mem[m-0x400] = uint8(v.getval())
	d := uint16(v.getval())
	v.mem[m-0x3fe] = uint8(d >> 8)
	v.mem[m-0x3fd] = uint8(d)
	v.wu16(m-0x3fc, v.ru16(v.script.vramOrg-0xe))
	x := v.getval()
	v.w16(m-0x3fa, x)
	v.w16(m-0x3f8, x)
	v.w16(m-0x3f8, v.r16(m-0x3f8)+v.getval()-1)
	v.w16(m-0x3f4, v.getval())
	v.w16(m-0x3f2, v.getval())
	v.w16(m-0x3f0, v.getval())
	v.mem[m-0x3ff] = 0
	v.wu16(m-0x3dc, 4)
	v.wu16(m-0x3da, 0)
	v.mem[m-0x3d8] = 0x7f
	var n, w uint32
	if int8(v.mem[m-1]) < 0 {
		w, n = v.ru32(m-6), v.ru32(m-0xa)
	} else {
		w, n = uint32(v.ru16(m-4)), uint32(v.ru16(m-6))
	}
	if w == 0 || int32(n) < 0 { // cdq: a negative size overflows div
		v.fatal("cdefmap: division fault")
		return
	}
	v.wu16(m-0x3c6, uint16(n/w))
	v.wu16(m-0x3cc, uint16(n/w))
	v.wu16(m-0x3c4, uint16(w))
	v.wu16(m-0x3ca, uint16(w)>>1)
	v.wu16(v.script.vramOrg-0x3b6, 0)
	v.wu16(m-0x3c0, log2w(v.ru16(m-0x3f4)))
	v.wu16(m-0x3be, log2w(v.ru16(m-0x3f2)))
	v.wu16(m-0x3bc, log2w(v.ru16(m-0x3f0)))
	// 0x4a87 tests -0x3d8 for 0xa, but it was just set to 0x7f
}

// csetmap (0x4ab1) sets the map's position and visible size.
func csetmap(v *VM) {
	m := v.mapvar()
	v.w16(m-0x3ee, v.getval())
	v.w16(m-0x3ec, v.getval())
	v.w16(m-0x3ea, v.getval())
	d := v.getval()
	v.w16(m-0x3e8, d)
	d--
	v.w16(m-0x3d6, d)
	v.w16(m-0x3ee, v.r16(m-0x3ee)+int16(uint16(d)>>1))
	v.w16(m-0x3e6, v.getval())
	d = v.getval()
	v.w16(m-0x3e4, d)
	d--
	v.w16(m-0x3d4, d)
	v.w16(m-0x3ea, v.r16(m-0x3ea)+int16(uint16(d)>>1))
}

// cputmap (0x4b4d) puts the map as an element of the running entity: its
// image is the header at -0x3dc.
func cputmap(v *VM) {
	m := v.mapvar()
	v.w16(m-0x3e2, v.getval())
	v.w16(m-0x3e0, v.getval())
	v.w16(m-0x3de, v.getval())
	v.w16(dsNumelem, v.getval())
	w := v.ru16(m - 0x3f0)
	if w == 0 {
		v.fatal("cputmap: division by zero")
		return
	}
	v.wu16(m-0x3d2, v.ru16(m-0x3de)%w)
	v.mem[dsPutSub] = 0
	di, bx, found := v.searchelem()
	if !found {
		var ok bool
		if di, ok = v.createlem(di, bx); !ok {
			return
		}
	}
	if int8(v.fs8(di, spState)) >= 0 {
		v.setFs8(di, spState, 2)
	}
	v.putfill(di, m-0x3dc)
	v.putend()
	v.mem[dsPutAgain] = 0
	if di, _, found = v.searchelem(); found {
		v.setFs8(di, spCredon, 0xff)
		v.setFs8(di, spInvx, 0)
		v.setFs8(di, spZoom, 0)
		v.setFs16(di, spDepx, v.r16(m-0x3ee))
		v.setFs16(di, spDepy, v.r16(m-0x3ec))
		v.setFs16(di, spDepz, v.r16(m-0x3ea))
	}
}

func csavepal(v *VM) {
	v.readexecOpername()
	v.dosCsavepal(int16(v.varD7))
}
