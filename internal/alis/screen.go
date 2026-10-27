//go:build ignore

package alis

// Scene list (alis screen.c).

type screenState struct {
	ptscreen uint16 // first scene of the display list
}

func (v *VM) setScrToNext(idx, x uint16) { v.wu16(v.sca(idx)+scToNext, x) }

func (v *VM) scadd(idx uint16) {
	v.setScrToNext(idx, 0)
	if v.scr.ptscreen == 0 {
		v.scr.ptscreen = idx
		return
	}
	cur := v.scr.ptscreen
	for v.scrToNext(cur) != 0 {
		cur = v.scrToNext(cur)
	}
	v.setScrToNext(cur, idx)
}

func (v *VM) scbreak(idx uint16) {
	if v.scr.ptscreen == 0 {
		return
	}
	if idx == v.scr.ptscreen {
		v.scr.ptscreen = v.scrToNext(idx)
		return
	}
	for prev := v.scr.ptscreen; prev != 0; {
		next := v.scrToNext(prev)
		if next == idx {
			v.setScrToNext(prev, v.scrToNext(idx))
			return
		}
		prev = next
	}
}

func (v *VM) scdosprite(idx uint16) {
	s := v.spr(v.scrScreenID(idx))
	s.setNumelem(v.sc8(idx, 1))
	x, y := v.sc16(idx, 0x0e), v.sc16(idx, 0x10)
	w, h := v.sc16(idx, 0x12), v.sc16(idx, 0x14)
	s.setNewx(x)
	s.setDepx(x + w)
	s.setNewy(y)
	s.setNewd(0x7fff)
	s.setDepy(y + h)
}

// vectoriel completes the scene's rotation matrix with the cross product of
// its first two rows.
func (v *VM) vectoriel(idx uint16) {
	m := v.mem[v.sca(idx)+0x20:]
	g := func(i int) int8 { return int8(m[i]) }
	m[6] = uint8(g(1)*g(5) - g(4)*g(2))
	m[7] = uint8(g(2)*g(3) - g(5)*g(0))
	m[8] = uint8(g(0)*g(4) - g(3)*g(1))
}
