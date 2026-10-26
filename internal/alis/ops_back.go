package alis

// Background cache, page scroll, collision material and shrink (alis opcodes.c).

// iniback allocates the background cache (a full-screen 8 bpp shadow).
func (v *VM) iniback() {
	v.img.backmap = make([]byte, len(v.img.logic))
	v.img.backlarg = gameWidth
	v.img.backdes = 0
	v.img.backaddr = 0
}

func cscback(v *VM) {
	img := &v.img
	old := img.sback
	v.readexecOpername()
	img.sback = uint8(v.varD7)
	v.readexecOpername()
	img.backprof = int16(v.varD7)
	v.readexecOpername()
	img.backx1 = int16(v.varD7)
	v.readexecOpername()
	img.backy1 = int16(v.varD7)
	v.readexecOpername()
	img.backx2 = int16(v.varD7)
	v.readexecOpername()
	img.backy2 = int16(v.varD7)
	if old == img.sback {
		return
	}
	id := v.curScreenID()
	tgt := v.basemain + uint32(id)
	bs := uint16(img.backsprite)
	if img.sback == 0 {
		img.backmap = nil
		img.cback = 0
		img.pback = 0
		v.mem[tgt+1] &= 0xfb
		for e := v.ru16(tgt + 2); e != 0; e = v.spr(e).link() {
			if sp := v.spr(e); bs == sp.link() {
				sp.setLink(v.spr(bs).link())
				return
			}
		}
		return
	}
	v.iniback()
	b := v.spr(bs)
	b.setScreenID(int16(id))
	b.setNewx(img.backx1)
	b.setNewy(img.backy1)
	b.setNewd(img.backprof)
	b.setCordspr(0x80)
	b.setNumelem(-0x80)
	b.setClinking(-1)
	b.setChsprite(0xff)
	b.setState(0)
	b.setNewad(uint32(img.backdes))
	b.setNewf(0)
	e := v.ru16(tgt + 2)
	v.mem[tgt+1] |= 4
	img.wback = 0
	v.rangesprite(bs, 0xffff, e)
	e = v.spr(e).link()
	for e != 0 {
		if e == bs {
			return
		}
		sp := v.spr(e)
		sp.setState(2)
		e = sp.link()
	}
}

func cscrolpage(v *VM) {
	v.readexecOpername()
	v.img.spag = int8(v.varD7)
	v.readexecOpername()
	v.img.pagdx = int16(v.varD7)
	v.readexecOpername()
	v.img.pagdy = int16(v.varD7)
	v.readexecOpername()
	v.img.pagcount = int16(v.varD7)
	if v.img.spag != 0 {
		v.mem[v.basemain+uint32(v.curScreenID())] |= 0x20
	}
}

func cmatent(v *VM) {
	v.varD7 = int32(v.matent[v.ptrent-1])
	v.cstoreContinue()
}

func cshrink(v *VM) {
	v.readexecOpername()
	v.readexecOpernameSaveD6()
	v.flagmain = 0
	addr := uint32(v.adresdes(v.varD7))
	addr += uint32(v.r32(addr))
	var width int16
	switch v.mem[addr] {
	case 0, 2, 0x12, 0x10:
		width = (v.r16(addr+2) + 1) >> 1
	case 0x1a, 0x18, 0x16, 0x14, 0x1e, 0x1c:
		width = v.r16(addr+2) + 1
	default:
		return
	}
	height := v.r16(addr + 4)
	if height == 0 {
		return
	}
	height++
	bits := int32(width) * int32(height)
	org := uint32(v.r32(v.script.vramOrg - 0x14))
	l := uint32(v.r32(org + 0xe))
	e := v.r16(org + l + 4)
	a := uint32(v.r32(org+l)) + l
	for i := int32(0); i < int32(e); i, a = i+1, a+4 {
		t := v.r32(org + a)
		if addr < a+org+uint32(t) {
			v.w32(org+a, t-bits)
		}
	}
	v.w32(org+l+6, v.r32(org+l+6)-bits)
	v.w32(org+l+0xc, v.r32(org+l+0xc)-bits)
	v.w16(addr+2, 0xf)
	v.w16(addr+4, 0)
	v.shrinkprog(int32(addr+6), bits, 0)
	v.script.data.sz -= uint32(bits)
}
