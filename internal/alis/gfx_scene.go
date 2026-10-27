package alis

import "fmt"

// The scene opcodes (START.EXE 0x42db..0x48c5). They take the scene of the
// running entity, mostly without checking that it has one, and mark it
// changed (bit 7). 0x9d24, the operand reader, first copies D7 to D6 (cx);
// 0x9d17 keeps D7 and leaves the operand in D6.

// getval is 0x9d24.
func (v *VM) getval() int16 {
	v.readexecOpernameSaveD7()
	return int16(v.varD7)
}

// operAX is the low word of eax after the last operand: 0x9d24 enters the
// operand's routine with eax zero.
func (v *VM) operAX() int16 {
	switch v.operCode {
	case 0x00, 0x02, 0x06, 0x12: // oimmb, oimmw, olocb, odirb: ax = dx
		return int16(v.varD7)
	case 0x08: // olocw: the offset
		return v.r16(v.operPC)
	case 0x14: // odirw: the byte offset
		return int16(v.mem[v.operPC])
	}
	v.fatal(fmt.Sprintf("%s: eax after operand %#x is not ported", v.script.name, v.operCode))
	return 0
}

// curScene is the scene of the running entity (movsx, unchecked).
func (v *VM) curScene() uint32 { return v.sca(v.curScreenID()) }

// cscamov (0x42db) moves the scene's camera angles, cscaset (0x430e) and
// csczoom (0x433b) set its angles and zoom; the middle word gets D6, which
// 0x9d24 has set to the first operand.
func cscamov(v *VM) {
	x, y, z := v.getval(), v.getval(), v.getval()
	a := v.curScene()
	v.w16(a+0x38, v.r16(a+0x38)+z)
	v.w16(a+0x36, v.r16(a+0x36)+y)
	v.w16(a+0x34, v.r16(a+0x34)+x)
	v.mem[a] |= 0x80
}

func (v *VM) scset3(off uint32) {
	x := v.getval()
	v.readexecOpernameSaveD6()
	z := v.getval()
	a := v.curScene()
	v.w16(a+off+4, z)
	v.w16(a+off+2, int16(v.varD6))
	v.w16(a+off, x)
	v.mem[a] |= 0x80
}

func cscaset(v *VM) { v.scset3(0x34) }
func csczoom(v *VM) { v.scset3(0x3a) }

// cscfollow (0x4368): the entity to follow, what to follow and the
// offsets; the last one is stored from ax.
func cscfollow(v *VM) {
	var w [7]int16
	for i := range w {
		w[i] = v.getval()
	}
	v.getval()
	a := v.curScene()
	v.w16(a+0x90, v.operAX())
	for i, off := range []uint32{0x8e, 0x8c, 0x8a, 0x88, 0x86} {
		v.w16(a+off, w[6-i])
	}
	v.mem[a+0x84] = uint8(w[1])
	v.w16(a+0x60, w[0])
	v.mem[a] |= 0x80
}

// cscdark (0x43f0): the scene's darkness tables; the selector 0x1aafc
// gets their base.
func cscdark(v *VM) {
	a := v.curScene()
	v.w16(a+0x98, v.getval())
	res, ok := v.resaddr(uint16(v.getval()))
	if !ok {
		return
	}
	v.wu32(a+0x9a, res)
	d := v.getval()
	v.w16(a+0x9e, d)
	v.w16(0x1aafe, d)
	cx := v.ru16(v.ru32(a+0x9a)+4) + 1
	v.wu16(a+0xa0, cx)
	if v.r16(a+0x9e) < 0 {
		cx >>= 1
		v.wu16(a+0x9e, cx)
		v.wu16(0x1aafe, cx)
	}
	v.ptrdark = v.ru32(a+0x9a) + 8
	v.mem[a] |= 0x80
}

// cscsky (0x447b): the sky image and its scroll per angle.
func cscsky(v *VM) {
	a := v.curScene()
	v.w16(a+0xa2, v.getval())
	entry, ok := v.resentry(uint16(v.getval()))
	if !ok {
		return
	}
	v.wu32(a+0xa4, entry)
	v.w16(a+0xa8, v.getval())
	res := entry + v.ru32(entry)
	if v.mem[res] == 3 {
		e := entry + uint32(int32(v.r16(res+2)))*4
		res = e + v.ru32(e)
	}
	t := int32(int16(4*v.r16(a+0x6c))) >> 3
	w := int64(v.ru16(res+2)) + 1
	q := int64(t)*0x168/w + 0x80
	q = int64(uint32(q) & 0xff00)
	v.wu16(a+0xa8, uint16(q*w/0x168))
	v.mem[a] |= 0x80
}

// cscsun (0x450b) and cscview (0x4556) set words of the scene.
func (v *VM) scwords(off uint32, n int) {
	var w [6]int16
	for i := range n {
		w[i] = v.getval()
	}
	a := v.curScene()
	for i := n - 1; i >= 0; i-- {
		v.w16(a+off+uint32(i)*2, w[i])
	}
	v.mem[a] |= 0x80
}

func cscsun(v *VM)  { v.scwords(0x7c, 4) }
func cscview(v *VM) { v.scwords(0x64, 6) }

// cscmov (0x45b7) moves the scene, cscset (0x4607) places it.
func cscmov(v *VM) {
	x, y, z := v.getval(), v.getval(), v.getval()
	a := v.curScene()
	for _, d := range []struct {
		off uint32
		d   int16
	}{{0x1a, z}, {0x18, y}, {0x16, x}} {
		if d.d != 0 {
			v.w16(a+d.off, v.r16(a+d.off)+d.d)
			v.mem[a] |= 0x80
		}
	}
}

func cscset(v *VM) {
	x, y, z := v.getval(), v.getval(), v.getval()
	a := v.curScene()
	for _, d := range []struct {
		off uint32
		x   int16
	}{{0x1a, z}, {0x18, y}, {0x16, x}} {
		if d.x != v.r16(a+d.off) {
			v.w16(a+d.off, d.x)
			v.mem[a] |= 0x80
		}
	}
}

// cschoriz (0x4740) and cscvertic (0x477b) set rows of the scene's
// matrix; without a scene they read no operands.
func cschoriz(v *VM) {
	if v.curScreenID() == 0 {
		return
	}
	x, y, z := v.getval(), v.getval(), v.getval()
	a := v.curScene()
	v.mem[a+0x22] = uint8(z)
	v.mem[a+0x21] = uint8(y)
	v.mem[a+0x20] = uint8(x)
	v.mem[a] |= 0x80
}

func cscvertic(v *VM) {
	if v.curScreenID() == 0 {
		return
	}
	x, y, z := v.getval(), v.getval(), v.getval()
	a := v.curScene()
	v.mem[a+0x25] = uint8(z)
	v.mem[a+0x24] = uint8(y)
	v.mem[a+0x23] = uint8(x)
	v.vectoriel(a)
	v.mem[a] |= 0x80
}

// cscreduce (0x4840): the perspective shift, the last reduced image and
// the reduction shift; without a scene the third operand is not read.
func cscreduce(v *VM) {
	x := v.getval()
	v.readexecOpernameSaveD6()
	if v.curScreenID() == 0 {
		return
	}
	a := v.curScene()
	v.mem[a+0x1d] = uint8(x)
	v.mem[a+0x1e] = uint8(v.varD6) - 1
	v.mem[a+0x1f] = uint8(v.getval())
	v.mem[a] |= 0x80
}

// cscscale (0x4875): the scale shift; the scene's flags byte gets cl, D7
// before the operand.
func cscscale(v *VM) {
	x := v.getval()
	if v.curScreenID() == 0 {
		return
	}
	a := v.curScene()
	v.mem[a+0x1c] = uint8(x)
	v.mem[a] = uint8(v.varD6) | 0x80
}

// cscmap (0x4895): the scene's map entity and number; the operand is read
// and dropped.
func cscmap(v *VM) {
	v.getval()
	cx := v.ru16(v.script.vramOrg - 0xe)
	ax := uint16(v.read16())
	if ax == 0 {
		cx = 0
		ax = uint16(v.read16())
	}
	if id := v.curScreenID(); id != 0 {
		a := v.sca(id)
		v.wu16(a+0x42, ax)
		v.wu16(a+0x40, cx)
	}
}
