package alis

// The land renderer proper (START.EXE 0xd6ff..0xf444). The code is a
// hand-written column caster that keeps fixed-point values with the
// integer part in the low word and the fraction in the high word, so that
// "add esi,d; adc si,0" steps both; it is ported register by register.
// fs is the land buffer at 0x1aa60 while it runs.

const dsFsSeg = 0x14e18

// setFs is 0x454: the base of fs.
func (v *VM) setFs(base uint32) { v.wu32(dsFsSeg, base) }

func (v *VM) fsb() uint32 { return v.ru32(dsFsSeg) }

// lregs are the registers of the renderer; push and pop keep its
// stack, which 0xd96e uses to reach the caller's registers.
type lregs struct {
	v                                 *VM
	eax, ebx, ecx, edx, esi, edi, ebp uint32
	fs                                uint32 // base of fs
	stk                               []uint32
}

func (r *lregs) m8(a uint32) uint8   { return r.v.mem[a] }
func (r *lregs) m16(a uint32) uint16 { return r.v.ru16(a) }
func (r *lregs) m32(a uint32) uint32 { return r.v.ru32(a) }
func (r *lregs) f16(a uint32) uint16 { return r.v.ru16(r.fs + a) }
func (r *lregs) f32(a uint32) uint32 { return r.v.ru32(r.fs + a) }
func (r *lregs) g8(a uint32) uint8   { return r.v.mem[r.v.gs(a)] }
func (r *lregs) g16(a uint32) uint16 { return r.v.gsu16(a) }
func (r *lregs) g32(a uint32) uint32 { return r.v.gs32(a) }

// gdi is gs:[di+off], 16-bit addressing.
func (r *lregs) gdi(off uint16) uint32 { return uint32(w(r.edi) + off) }

func (r *lregs) push(x uint32) { r.stk = append(r.stk, x) }
func (r *lregs) pop() uint32 {
	x := r.stk[len(r.stk)-1]
	r.stk = r.stk[:len(r.stk)-1]
	return x
}

func w(r uint32) uint16        { return uint16(r) }
func ws(r uint32) int16        { return int16(r) }
func setw(r *uint32, x uint16) { *r = *r&0xffff0000 | uint32(x) }
func l8(r uint32) uint8        { return uint8(r) }
func h8(r uint32) uint8        { return uint8(r >> 8) }
func setl8(r *uint32, x uint8) { *r = *r&0xffffff00 | uint32(x) }
func seth8(r *uint32, x uint8) { *r = *r&0xffff00ff | uint32(x)<<8 }
func rol16(r uint32) uint32    { return r<<16 | r>>16 }
func addw(r *uint32, x uint16) { setw(r, w(*r)+x) }
func subw(r *uint32, x uint16) { setw(r, w(*r)-x) }

// addAdc is "add r,d; adc r16,0", subSbb "sub r,d; sbb r16,0".
func addAdc(r *uint32, d uint32) {
	s := *r + d
	c := s < *r
	*r = s
	if c {
		addw(r, 1)
	}
}

func subSbb(r *uint32, d uint32) {
	b := *r < d
	*r -= d
	if b {
		subw(r, 1)
	}
}

// fill4 is rep stosd of eax, n dwords at edi.
func (v *VM) fill4(edi uint32, eax uint32, n uint32) uint32 {
	for range n {
		v.wu32(edi, eax)
		edi += 4
	}
	return edi
}

// movsb is rep movsb.
func (v *VM) movsb(edi, esi, n uint32) (uint32, uint32) {
	for range n {
		v.mem[edi] = v.mem[esi]
		edi++
		esi++
	}
	return edi, esi
}

// landSky is 0xf22f: the sky image (+0xa4) scrolled by the heading, and
// the rows above and below it in its first and last colours.
func (v *VM) landSky(a uint32) {
	if v.mem[0x1aae2] != 0 {
		return
	}
	eax := v.ru32(a + 0xa4)
	esi := eax + v.ru32(eax)
	var cx int16
	if v.mem[esi] == 3 {
		edi := esi
		esi = eax + uint32(int32(v.r16(edi+2)))*4
		esi += v.ru32(esi)
		cx = v.r16(edi + 6)
	}
	top := v.gs16(0x8ae) + v.r16(a+0xc) + cx - int16((v.ru16(esi+4)+1)>>1)
	v.w16(0x1aac0, top)
	p := mul16(v.r16(a+0x38), v.r16(a+0xa8))
	ax := int16(uint16(p>>8)&0xff | uint16(p>>16)<<8)
	ax -= v.r16(dsFenx1)
	bx := int16(v.ru16(esi+2) + 1)
	if bx == 0 || int32(ax)/int32(bx) > 0x7fff {
		v.fatal("land: division fault (0xf2ac)")
		return
	}
	dx := -int16(int32(ax) % int32(bx))
	if dx < 0 {
		dx += bx
	}
	v.w16(0x1aabe, dx)
	dx = -dx + bx
	if dx > v.r16(dsFenl) {
		dx = v.r16(dsFenl)
	}
	v.w16(0x1aac2, dx)
	fenl := uint32(v.ru16(dsFenl))
	row := uint32(v.ru16(dsFeny1) - v.ru16(dsWlogy1))
	edi := v.ru32(v.fsb()+row*4+0x16800) + uint32(v.ru16(dsFenx1))
	skip := uint32(uint16(v.ru16(dsWloglarg) - v.ru16(dsFenl)))
	fill := func(n int16, c uint8) {
		w := uint32(c) * 0x01010101
		for ; n != 0; n-- {
			edi = v.fill4(edi, w, fenl>>2) + skip
		}
	}
	if n := top - v.r16(dsFeny1); n > 0 {
		if n > v.r16(dsFenh) {
			n = v.r16(dsFenh)
		}
		fill(n, v.mem[esi+8])
	}
	y0 := top
	if y0 < v.r16(dsFeny1) {
		y0 = v.r16(dsFeny1)
	}
	y1 := top + v.r16(esi+4)
	if y1 > v.r16(dsFeny2) {
		y1 = v.r16(dsFeny2)
	}
	if n := y1 + 1 - y0; n > 0 {
		w := uint32(v.ru16(esi+2)) + 1
		var off uint32
		if d := v.r16(dsFeny1) - top; d > 0 {
			off = uint32(uint16(d)) * uint32(uint16(w))
		}
		src := off + uint32(v.ru16(0x1aabe)) + esi + 8
		first := uint32(v.ru16(0x1aac2))
		for ; n != 0; n-- {
			s := src
			if first != 0 {
				edi, s = v.movsb(edi, s, first)
			}
			if rest := int32(fenl - first); rest > 0 {
				edi, _ = v.movsb(edi, s-w, uint32(rest))
			}
			edi += skip
			src += w
		}
	}
	if n := v.r16(dsFeny2) - top - v.r16(esi+4); n > 0 {
		if n > v.r16(dsFenh) {
			n = v.r16(dsFenh)
		}
		last := uint32(uint16(v.ru16(esi+2)+1)) * uint32(v.ru16(esi+4))
		fill(n, v.mem[last+esi+8])
	}
}

// landE648 is 0xe648: the land of scene a, cast column by column from far
// to near, with the elements drawn in between by depth.
func (v *VM) landE648(a uint32) {
	r := &lregs{v: v, ebp: a}
	v.wu16(0x1ab00, 0x8000)
	bx := v.fsu16(v.ru16(a+2), spLink)
	if bx != 0 {
		v.wu16(0x1ab00, v.fsu16(bx, spNewd))
	}
	v.wu16(0x1ab02, bx)
	r.ebx = uint32(bx)
	r.eax = v.ru32(0x1aa60)
	v.setFs(r.eax)
	r.fs = r.eax
	if v.ru16(a+0xa2) != 0 {
		v.landSky(a)
	}
	v.mem[0x1aab2] = 0
	v.mem[0x1aab3] = 0
	v.wu32(0x1aa92, 0)
	for _, o := range []uint32{0x1aa9c, 0x1aaa4, 0x1aaa8, 0x1aa9e, 0x1aaa6, 0x1aaaa} {
		v.wu16(o, 0x3e8)
	}
	v.setGs32(0x9a4, 0)
	v.setGs32(0x980, 0)
	r.ecx = uint32(r.g16(0x85e))
	r.edx = uint32(r.g16(0x860))
	r.esi, r.edi = r.ecx, r.edx
	v.setGs32(0x954, rol16(r.esi))
	v.setGs32(0x958, rol16(r.edi))
	v.setGs32(0x93c, rol16(r.g32(0x93c)))
	v.setGs32(0x940, rol16(r.g32(0x940)))
	r.eax = uint32(r.g16(0x8ae) + r.g16(0x992))
	v.setGs32(0x98c, r.eax)
	r.push(r.edi)
	r.push(r.edx)
	r.push(r.ecx)
	r.colEdge()
	r.ecx = r.pop()
	r.edx = r.pop()
	r.edi = r.pop()
	goto eb0a

e802:
	r.push(r.g32(0x97c))
	r.push(r.esi)
	r.push(r.edi)
e80b:
	setw(&r.eax, r.g16(0x99c))
	setw(&r.ebx, r.g16(0x9a0))
	if ws(r.ebx) < ws(r.eax) {
		setw(&r.ebx, w(r.eax))
	}
	if ws(r.ebp) <= int16(r.g16(0x97e)) {
		goto e946
	}
e830:
	v.setGsu16(0x9a0, w(r.eax))
	addAdc(&r.esi, r.g32(0x93c))
	addAdc(&r.edi, r.g32(0x940))
	r.eax = r.g32(0x980)
	v.setGs32(0x97c, r.g32(0x97c)+r.eax)
	r.eax = 0
	if w(r.esi) > r.g16(0x968) {
		goto e897
	}
	if w(r.edi) <= r.g16(0x96a) {
		goto e8fa
	}
	if r.g8(0x802) != 1 {
		goto e911
	}
	r.eax = r.f32(uint32(w(r.esi))*4 + 0x18b20)
	goto e8d0
e897:
	if r.g8(0x802) != 1 {
		goto e911
	}
	setw(&r.eax, 0)
	if ws(r.esi) >= int16(r.g16(0x968)) {
		setw(&r.eax, r.g16(0x968)*2)
	}
	subw(&r.eax, w(r.esi))
	r.eax = r.f32(uint32(int32(ws(r.eax)))*4 + 0x18b20)
	if w(r.edi) <= r.g16(0x96a) {
		goto e905
	}
e8d0:
	if ws(r.edi) >= int16(r.g16(0x96a)) {
		addw(&r.eax, 4*r.g16(0x96a))
	}
	subw(&r.eax, 2*w(r.edi))
	goto e909
e8fa:
	r.eax = r.f32(uint32(w(r.esi))*4 + 0x18b20)
e905:
	setw(&r.eax, w(r.eax+r.edi*2))
e909:
	v.wu32(0x1aa92, r.eax)
	r.eax = uint32(r.m8(r.eax))
e911:
	r.eax += r.g32(0x9a4)
	setw(&r.eax, r.f16(r.eax*2))
	addw(&r.eax, r.g16(0x98c))
	v.setGsu16(0x99c, w(r.eax))
	if ws(r.eax) > ws(r.ebx) {
		setw(&r.ebx, w(r.eax))
	}
	if ws(r.ebp) > int16(r.g16(0x97e)) {
		goto e830
	}
e946:
	v.mem[0x1aab0] = 0
	if w(r.ecx) > r.g16(0x968) {
		goto e98f
	}
	if w(r.edx) <= r.g16(0x96a) {
		goto e9fd
	}
	if r.g8(0x802) != 1 {
		goto eac2
	}
	v.mem[0x1aab0] = 1
	r.eax = r.f32(uint32(w(r.ecx))*4 + 0x18b20)
	goto e9d3
e98f:
	if r.g8(0x802) != 1 {
		goto eac2
	}
	v.mem[0x1aab0] = 1
	setw(&r.eax, 0)
	if ws(r.ecx) >= int16(r.g16(0x968)) {
		setw(&r.eax, r.g16(0x968)*2)
	}
	subw(&r.eax, w(r.ecx))
	r.eax = r.f32(uint32(int32(ws(r.eax)))*4 + 0x18b20)
	if w(r.edx) <= r.g16(0x96a) {
		goto ea08
	}
e9d3:
	if ws(r.edx) >= int16(r.g16(0x96a)) {
		addw(&r.eax, 4*r.g16(0x96a))
	}
	subw(&r.eax, 2*w(r.edx))
	goto ea11
e9fd:
	r.eax = r.f32(uint32(w(r.ecx))*4 + 0x18b20)
ea08:
	setw(&r.eax, w(r.eax+r.edx*2))
	v.wu32(0x1aa96, r.eax)
ea11:
	r.push(r.edi)
	r.push(r.esi)
	setw(&r.edi, uint16(r.m8(r.eax+1)))
	v.wu16(0x1ab28, w(r.edi))
	setw(&r.edi, (w(r.edi)&0x3f)<<5)
	r.eax = uint32(r.m8(r.eax))
	v.wu16(0x1aa84, w(r.eax))
	r.eax += r.g32(0x9a8)
	setw(&r.eax, r.f16(r.eax*2))
	addw(&r.eax, r.g16(0x984))
	v.setGsu16(0x9ac, w(r.eax))
	{
		t := r.g16(0x99e)
		v.setGsu16(0x99e, w(r.eax))
		setw(&r.eax, t)
	}
	if ws(r.eax) > int16(r.g16(0x99e)) {
		v.setGsu16(0x9ac, w(r.eax))
		setw(&r.eax, r.g16(0x99e))
	}
	r.esi = uint32(r.g16(r.gdi(2)))
	if w(r.esi) != 0 {
		r.esi += r.g32(0x9a8)
		setw(&r.esi, r.f16(r.esi*2))
		addw(&r.eax, w(r.esi))
	}
	{
		b := ws(r.ebx)
		subw(&r.ebx, w(r.eax))
		if b > ws(r.eax) {
			r.column(0xd6ff)
		}
	}
	if c := r.g8(r.gdi(0x14)); c != 0 {
		if int8(c) < 0 {
			goto eefb
		}
		if c != 1 {
			goto eac0
		}
		r.landDA41()
	}
	v.mem[0x1aab2] = 0
	v.mem[0x1aab3] = 0
eac0:
	r.esi = r.pop()
	r.edi = r.pop()
eac2:
	v.wu16(0x1aa88, w(r.ebp))
	v.wu16(0x1ab20, v.ru16(0x1ab20)+0x2000)
	if ws(r.ebp) > v.r16(0x1ab3a) {
		goto eb01
	}
	addAdc(&r.ecx, r.g32(0x93c))
	addAdc(&r.edx, r.g32(0x940))
	addAdc(&r.ebp, r.g32(0x978))
	goto e80b
eb01:
	r.edi = r.pop()
	r.esi = r.pop()
	v.setGs32(0x97c, r.pop())
eb0a:
	r.ecx, r.edx = r.esi, r.edi
	setw(&r.eax, (w(r.ecx)+w(r.edx))<<4)
	v.wu16(0x1ab20, w(r.eax))
	r.eax = r.g32(0x944)
	v.setGs32(0x954, r.g32(0x954)+r.eax)
	r.eax = r.g32(0x948)
	v.setGs32(0x958, r.g32(0x958)+r.eax)
	switch {
	case r.g32(0x93c) == 1:
		addw(&r.edi, r.g16(0x962))
		if int16(r.g16(0x946)) >= 0 {
			for ws(r.esi) < int16(r.g16(0x956)) {
				addAdc(&r.esi, r.g32(0x93c))
				addAdc(&r.edi, r.g32(0x940))
			}
		} else {
			for ws(r.esi) > int16(r.g16(0x956)) {
				subSbb(&r.esi, r.g32(0x93c))
				subSbb(&r.edi, r.g32(0x940))
			}
		}
	case r.g32(0x93c) == 0xffff:
		addw(&r.edi, r.g16(0x962))
		if int16(r.g16(0x946)) >= 0 {
			for ws(r.esi) < int16(r.g16(0x956)) {
				subSbb(&r.esi, r.g32(0x93c))
				subSbb(&r.edi, r.g32(0x940))
			}
		} else {
			for ws(r.esi) > int16(r.g16(0x956)) {
				addAdc(&r.esi, r.g32(0x93c))
				addAdc(&r.edi, r.g32(0x940))
			}
		}
	case r.g32(0x940) == 1:
		addw(&r.esi, r.g16(0x95e))
		if int16(r.g16(0x94a)) >= 0 {
			for ws(r.edi) < int16(r.g16(0x95a)) {
				addAdc(&r.esi, r.g32(0x93c))
				addAdc(&r.edi, r.g32(0x940))
			}
		} else {
			for ws(r.edi) > int16(r.g16(0x95a)) {
				subSbb(&r.esi, r.g32(0x93c))
				subSbb(&r.edi, r.g32(0x940))
			}
		}
	default:
		addw(&r.esi, r.g16(0x95e))
		if int16(r.g16(0x94a)) >= 0 {
			for ws(r.edi) < int16(r.g16(0x95a)) {
				subSbb(&r.esi, r.g32(0x93c))
				subSbb(&r.edi, r.g32(0x940))
			}
		} else {
			for ws(r.edi) > int16(r.g16(0x95a)) {
				addAdc(&r.esi, r.g32(0x93c))
				addAdc(&r.edi, r.g32(0x940))
			}
		}
	}
	v.setGs32(0x9ae, r.g32(0x9ae)+r.g32(0x9b2))
	r.eax = r.g32(0x924)
	if t := r.g32(0x920) + r.eax; int32(t) >= 0 {
		v.setGs32(0x920, t)
	}
	r.eax = r.g32(0x92c)
	v.setGs32(0x928, r.g32(0x928)+r.eax)
	r.eax = rol16(r.g32(0x980))
	v.setGs32(0x978, r.eax)
	r.ebp = rol16(r.g32(0x97c))
	r.eax = r.g32(0x98c)
	v.setGs32(0x984, r.eax)
	r.push(r.edi)
	r.push(r.edx)
	r.push(r.ecx)
	if int16(r.g16(0x9b6)) >= 0 {
		r.colEdge()
		setw(&r.ebx, r.g16(0x922)+r.g16(0x858))
		r.eax = r.g32(0x928)
		q := v.idiv32by16(int32(r.eax), ws(r.ebx))
		r.eax = uint32(uint16(q) + r.g16(0x992))
		v.setGs32(0x98c, r.eax)
	}
	setw(&r.eax, r.g16(0x922))
	if ws(r.eax) <= v.r16(0x1ab00) {
		v.landSprites(ws(r.eax))
	}
	v.setGs32(0x9a8, r.g32(0x9a4))
	{
		e := int32(r.g32(0x920)-r.g32(0x938)) >> 8
		q := v.idiv32by16(e, int16(r.g16(0x99a)))
		r.eax = uint32(uint16(q))
		if int16(r.g16(0x9b6)) == 1 {
			r.eax = 0xb3
		}
		if ws(r.eax) > 0xb3 {
			r.eax = 0xb3
		}
		if ws(r.eax) < 0 {
			r.eax = 0
		}
		r.eax = r.eax << 9 >> 1
		v.setGs32(0x9a4, r.eax)
	}
	v.wu16(0x1aa88, v.ru16(dsFenx1)-0x10)
	{
		y := v.ru16(dsFeny2)
		v.setGsu16(0x99c, y)
		v.setGsu16(0x99e, y)
		r.ecx = r.g32(0x978)
		r.ecx = r.ecx<<8 | r.ecx>>24
		d := uint32(w(r.ecx))
		if d == 0 || 0x200000/d > 0xffff {
			v.fatal("land: division fault (0xee9c)")
			return
		}
		q := 0x200000 / d
		if 0x200000%d != 0 {
			q++
		}
		r.eax = uint32(uint16(q))
		v.wu16(0x1ab1a, uint16(q))
	}
	v.mem[0x1aab2] = 0
	v.mem[0x1aab3] = 0
	r.ecx = r.pop()
	r.edx = r.pop()
	r.edi = r.pop()
	{
		n := int16(r.g16(0x9b6)) - 1
		v.setGs16(0x9b6, n)
		if n >= 0 || n >= -3 {
			goto e802
		}
	}
	v.landSprites(-1)
	v.setFs(v.ru32(dsFsBase))
	return

eefb:
	if v.mem[0x1aab0] == 1 {
		goto eac0
	}
	r.push(r.ebp)
	r.push(r.edi)
	r.push(r.esi)
	r.push(r.ecx)
	v.mem[0x1aab5] = 1
	v.wu16(0x1aab6, w(r.ebp))
	setw(&r.esi, v.ru16(0x1aa9c))
	setw(&r.ebp, v.ru16(0x1aa9e))
	r.push(uint32(w(r.edi)))
	r.push(uint32(w(r.ecx)))
	r.eax = v.ru32(0x1aa92)
	if r.eax == 0 {
		goto f07c
	}
	setw(&r.edi, uint16(r.m8(r.eax)))
	v.wu16(0x1aa86, w(r.edi))
	setw(&r.edi, uint16(r.m8(r.eax+1)&0x3f)<<5)
	if int8(r.g8(r.gdi(0x14))) >= 0 {
		goto f00d
	}
	setw(&r.ecx, r.g16(r.gdi(0x16)))
	r.eax += r.g32(r.gdi(0x10))
	r.eax = uint32(r.m8(r.eax))
	r.ebx = r.eax
	{
		k := v.r16(0x1aa86)
		b := ws(r.ebx)
		subw(&r.ebx, uint16(k))
		if b <= k {
			goto f00d
		}
		c := ws(r.ecx)
		subw(&r.ecx, w(r.ebx))
		if c <= ws(r.ebx) {
			goto f00d
		}
		addw(&r.ecx, uint16(k))
		setw(&r.ebx, w(r.eax))
		if ws(r.ecx) < ws(r.eax) {
			subw(&r.ecx, uint16(k))
			setw(&r.ebx, w(r.ecx))
		}
	}
	subw(&r.ebx, w(r.eax))
	setw(&r.ebx, -w(r.ebx))
	r.eax += r.g32(0x9a4)
	setw(&r.eax, r.f16(r.eax*2)+r.g16(0x98c))
	v.wu16(0x1aa9c, w(r.eax))
	r.ebx += r.g32(0x9a4)
	setw(&r.ebx, r.f16(r.ebx*2)+r.g16(0x98c))
	v.wu16(0x1aa9e, w(r.ebx))
	v.wu32(0x1aa92, 0)
	if v.mem[0x1aab3] != 0 {
		goto f016
	}
	v.mem[0x1aab3] = 1
	v.wu16(0x1aaa8, 0x1388)
	v.wu16(0x1aaaa, 0xec78)
	goto f07c
f00d:
	v.mem[0x1aab3] = 0
	goto f07c
f016:
	setw(&r.ecx, v.ru16(0x1aaa8))
	v.wu16(0x1aaa4, w(r.ecx))
	setw(&r.ecx, v.ru16(0x1aaaa))
	v.wu16(0x1aaa6, w(r.ecx))
	if ws(r.eax) > ws(r.esi) {
		setw(&r.esi, w(r.eax))
	}
	v.wu16(0x1aaa8, w(r.esi))
	if ws(r.esi) < v.r16(0x1aaa4) {
		setw(&r.esi, v.ru16(0x1aaa4))
	}
	if ws(r.ebx) < ws(r.ebp) {
		setw(&r.ebp, w(r.ebx))
	}
	v.wu16(0x1aaaa, w(r.ebp))
	if ws(r.ebp) > v.r16(0x1aaa6) {
		setw(&r.ebp, v.ru16(0x1aaa6))
	}
	if ws(r.esi) < ws(r.ebp) {
		v.mem[0x1aab5] = 0
	}
f07c:
	setw(&r.ecx, uint16(r.pop()))
	setw(&r.edi, uint16(r.pop()))
	r.eax = r.f32(uint32(w(r.ecx))*4 + 0x18b20)
	setw(&r.eax, w(r.eax+r.edx*2))
	r.eax += r.g32(r.gdi(0x10))
	setw(&r.ecx, r.g16(r.gdi(0x16)))
	v.mem[0x1aab4] = 0
	setw(&r.edi, uint16(r.m8(r.eax+1)&0x3f)<<5)
	r.eax = uint32(r.m8(r.eax))
	r.ebx = r.eax
	{
		k := v.r16(0x1aa84)
		b := ws(r.ebx)
		subw(&r.ebx, uint16(k))
		if b <= k {
			goto f226
		}
		c := ws(r.ecx)
		subw(&r.ecx, w(r.ebx))
		if c <= ws(r.ebx) {
			goto f226
		}
		addw(&r.ecx, uint16(k))
		if ws(r.ecx) < ws(r.eax) {
			subw(&r.ecx, uint16(k))
			setw(&r.ebx, w(r.ecx))
			v.mem[0x1aab4] = 1
		}
	}
	subw(&r.ebx, w(r.eax))
	setw(&r.ebx, -w(r.ebx))
	r.eax += r.g32(0x9a8)
	setw(&r.eax, r.f16(r.eax*2)+r.g16(0x984))
	v.wu16(0x1aa9a, w(r.eax))
	setw(&r.ecx, r.g16(0x9ac))
	v.setGsu16(0x9ac, w(r.eax))
	{
		t := v.ru16(0x1aaa0)
		v.wu16(0x1aaa0, w(r.eax))
		setw(&r.eax, t)
	}
	if ws(r.eax) > v.r16(0x1aaa0) {
		v.setGsu16(0x9ac, w(r.eax))
		setw(&r.eax, v.ru16(0x1aaa0))
	}
	if v.mem[0x1aab4] == 0 {
		setw(&r.ebx, w(r.ecx))
	} else {
		r.ebx += r.g32(0x9a8)
		setw(&r.ebx, r.f16(r.ebx*2)+r.g16(0x984))
	}
	v.wu16(0x1aaae, w(r.ebx))
	{
		t := v.ru16(0x1aaa2)
		v.wu16(0x1aaa2, w(r.ebx))
		setw(&r.ebx, t)
	}
	if ws(r.ebx) < v.r16(0x1aaa2) {
		v.wu16(0x1aaae, w(r.ebx))
		setw(&r.ebx, v.ru16(0x1aaa2))
	}
	v.wu16(0x1aaac, w(r.ebx))
	if v.mem[0x1aab2] == 0 {
		goto f21f
	}
	{
		b := ws(r.ebx)
		subw(&r.ebx, w(r.eax))
		if b <= ws(r.eax) {
			goto f21f
		}
	}
	if v.mem[0x1aab5] != 0 {
		goto f1f1
	}
	{
		s := ws(r.esi)
		subw(&r.esi, w(r.eax))
		if s <= ws(r.eax) {
			addw(&r.ebx, w(r.eax))
			setw(&r.eax, w(r.ebp))
			b := ws(r.ebx)
			subw(&r.ebx, w(r.ebp))
			if b <= ws(r.ebp) {
				goto f21f
			}
			goto f1f1
		}
		subw(&r.ebp, w(r.eax))
		if ws(r.ebx) > ws(r.ebp) {
			goto f1f1
		}
		v.mem[0x1aab4] = 0
		setw(&r.ebx, w(r.esi))
	}
f1f1:
	r.push(uint32(r.g16(0x99e)))
	v.setGsu16(0x99e, v.ru16(0x1aaa0))
	setw(&r.ebp, v.ru16(0x1aab6))
	setw(&r.esi, 0)
	r.column(0xd7de)
	v.setGsu16(0x99e, uint16(r.pop()))
f21f:
	v.mem[0x1aab2] = 1
f226:
	r.ecx = r.pop()
	r.esi = r.pop()
	r.edi = r.pop()
	r.ebp = r.pop()
	goto eac0
}

// colEdge is 0xe76a/0xed39: the screen column of the scan line's start
// and its step per map cell, from the map point (si, di).
func (r *lregs) colEdge() {
	v := r.v
	cl := uint(r.g16(0x840)) & 31
	ax := int16(w(r.esi)<<cl) - int16(r.g16(0x89a))
	bx := int16(w(r.edi)<<cl) - int16(r.g16(0x89c))
	cx := ax + int16(r.g16(0x888)) - int16(r.g16(0x882))
	dx := bx + int16(r.g16(0x88a)) - int16(r.g16(0x884))
	q1, q2 := v.landPersp2(ax, bx, cx, dx, int16(r.g16(0x922)))
	bx = q2 - q1
	r.eax = uint32(uint16(q1+int16(r.g16(0x990)))) << 16
	v.setGs32(0x97c, r.eax)
	q := v.idiv32by16(int32(bx)<<8, int16(r.g16(0x85c)))
	r.eax = uint32(int32(q) << 8)
	v.setGs32(0x980, r.eax)
}

// shade is the darkness of the column (0xdd64 and its copies): ah gets
// the row of the darkness table, which fs then addresses.
func (r *lregs) shade() {
	r.eax = uint32(r.m16(0x1ab28))
	setw(&r.eax, w(r.eax)<<2)
	ah := -(h8(r.eax) * 2)
	if ah += r.g8(0x9b0); int8(ah) < 0 {
		ah = 0
	}
	seth8(&r.eax, ah)
	r.fs = r.v.ptrdark
}

// texel is "or bl,dh; mov al,[ebx+ebp]; xor bl,dh" (xor: without it).
func (r *lregs) texel(xor bool) uint8 {
	setl8(&r.ebx, l8(r.ebx)|h8(r.edx))
	al := r.m8(r.ebx + r.ebp)
	if xor {
		setl8(&r.ebx, l8(r.ebx)^h8(r.edx))
	}
	return al
}

// dark is "mov al,fs:[eax]" with the darkness table.
func (r *lregs) dark(al uint8) uint8 {
	setl8(&r.eax, al)
	al = r.m8(r.fs + r.eax)
	setl8(&r.eax, al)
	return al
}

// mulw is "mul word [m]": dx:ax.
func mulw(ax, m uint16) (uint16, uint16) {
	p := uint32(ax) * uint32(m)
	return uint16(p), uint16(p >> 16)
}

// wallSetup is 0xdc1e/0xdfcc.
func (r *lregs) wallSetup() {
	v := r.v
	r.ebx = uint32(int32(v.r16(0x1aa88)))
	v.wu32(0x1aa88, r.ebx)
	r.ebx = uint32(v.ru16(dsWloglarg))
	v.wu32(0x1ab08, r.ebx)
	r.ebx -= r.ecx
	v.wu32(0x1ab0c, r.ebx)
	v.mem[0x1ab16] = l8(r.ecx)
	sh := (8 - r.g8(r.gdi(0))) & 31
	setw(&r.ebx, v.ru16(0x1ab1a)>>sh+1)
	v.wu16(0x1ab1e, w(r.ebx))
}

// wallStep is 0xdc8f..0xdcd7 and 0xe034..0xe07c: the texture step for
// a column of bx rows, cx wide.
func (r *lregs) wallStep() {
	if w(r.ebx) <= 0x40 {
		setw(&r.ebx, (w(r.ebx)-1)<<6+w(r.ecx)-1)
		setw(&r.ebx, r.f16(r.ebx*2+0x16b20))
		return
	}
	setw(&r.ebx, w(r.ebx)>>2)
	if w(r.ebx) > 0x40 {
		setw(&r.ebx, 0x40)
	}
	setw(&r.ebx, (w(r.ebx)-1)<<6+w(r.ecx)>>1-1)
	setw(&r.ebx, uint16(int16(r.f16(r.ebx*2+0x16b20))>>1))
}

// texture is the start of 0xdd17 and its copies: ebp to the texture of
// map type di and its masks; with part (0xdee4) the masks are of the
// rows below gs:[di+2], and esi gets their offset.
func (r *lregs) texture(part bool) {
	v := r.v
	r.ebp = r.g32(r.gdi(4))
	r.ebp += r.m32(r.ebp)
	setl8(&r.ecx, r.m8(r.ebp+2))
	v.mem[0x1ab22] = l8(r.ecx)
	setw(&r.esi, r.m16(r.ebp+4))
	if part {
		subw(&r.esi, r.g16(r.gdi(2)))
	}
	setl8(&r.ecx, r.g8(r.gdi(0)))
	setw(&r.esi, w(r.esi)<<(l8(r.ecx)&31))
	v.wu16(0x1ab24, w(r.esi))
	if part {
		r.esi = uint32(r.g16(r.gdi(2))) << (l8(r.ecx) & 31)
	}
}

// wallRows is the inner loops of 0xdd85, 0xde61, 0xe129, 0xe1f4: ch rows
// from edi, cl = dh pixels each going up (rev) or right, the texture row
// in bx stepped by [0x1ab1e] and the count in dx by step.
func (r *lregs) wallRows(rev bool, step func(*uint32)) {
	v := r.v
	for {
		setl8(&r.ecx, h8(r.edx))
		r.push(r.edi)
		r.push(uint32(w(r.edx)))
		r.push(uint32(w(r.ebx)))
		setw(&r.edx, v.ru16(0x1ab20))
		seth8(&r.edx, h8(r.edx)&v.mem[0x1ab22])
		setw(&r.ebx, w(r.ebx)&v.ru16(0x1ab24))
		for {
			al := r.dark(r.texel(true))
			if rev {
				r.edi--
				v.mem[r.edi] = al
				subw(&r.edx, w(r.esi))
			} else {
				v.mem[r.edi] = al
				r.edi++
				addw(&r.edx, w(r.esi))
			}
			seth8(&r.edx, h8(r.edx)&v.mem[0x1ab22])
			setl8(&r.ecx, l8(r.ecx)-1)
			if l8(r.ecx) == 0 {
				break
			}
		}
		setw(&r.ebx, uint16(r.pop()))
		setw(&r.edx, uint16(r.pop()))
		r.edi = r.pop()
		addw(&r.ebx, v.ru16(0x1ab1e))
		step(&r.edx)
		r.edi += v.ru32(0x1ab08)
		seth8(&r.ecx, h8(r.ecx)-1)
		if h8(r.ecx) == 0 {
			break
		}
	}
}

// column is 0xd6ff (a wall seen from the front) and 0xd7de (the top of a
// raised cell): rows ax.. at screen column bp, cx wide.
func (r *lregs) column(entry uint32) {
	v := r.v
	r.push(r.ebp)
	r.push(r.edi)
	r.push(r.esi)
	r.push(r.edx)
	r.push(r.ecx)
	v.wu16(0x1ab2e, w(r.esi))
	setw(&r.esi, w(r.eax))
	{
		c1 := w(r.eax) - v.ru16(0x1ab38)
		setw(&r.ecx, c1-v.ru16(0x1ab3c))
		if c1 > v.ru16(0x1ab3c) {
			goto clipLow
		}
		setw(&r.ecx, -w(r.ecx))
		if int16(w(r.ecx)-w(r.ebx)) < 0 {
			goto clipLow
		}
		setw(&r.edx, w(r.ebx))
	}
clipped:
	if ws(r.ebp) < v.r16(dsFenx1) {
		goto d789
	}
	r.ecx = uint32(w(r.ebp))
	{
		c := ws(r.ecx)
		subw(&r.ecx, v.ru16(0x1aa88))
		if c <= v.r16(0x1aa88) || w(r.ecx) >= 0x80 {
			goto d789
		}
	}
	if entry == 0xd6ff {
		r.ebp = uint32(r.g16(0x9ac))
		v.wu16(0x1ab36, 0)
		if int16(r.g16(0x9b6)) < 0 {
			goto d789
		}
	} else {
		if int16(r.g16(0x9b6)) < 0 {
			goto d789
		}
		r.ebp = uint32(r.g16(0x9ac))
		if ws(r.ebp) > v.r16(0x1aaae) {
			v.wu16(0x1aaae, w(r.ebp))
		}
		v.wu16(0x1ab36, 0)
		if v.mem[0x1aab4] != 0 {
			v.wu16(0x1ab34, w(r.edi))
			if ws(r.eax) >= v.r16(0x1aaae) {
				r.wallSetup()
				goto e011
			}
			b := ws(r.eax) + ws(r.edx)
			setw(&r.ebx, uint16(b)-v.ru16(0x1aaae))
			if b > v.r16(0x1aaae) {
				v.wu16(0x1ab36, w(r.ebx))
			}
		}
	}
	if v.mem[0x1aae2] != 0 {
		r.hit()
		goto d789
	}
	if r.g8(r.gdi(8)) != 0 {
		goto dc1e
	}
	// 0xd9f4: a flat column in the cell's colour
	if w(r.edx) == 0 {
		goto d789
	}
	subw(&r.eax, v.ru16(dsWlogy1))
	r.eax = uint32(w(r.eax))
	r.edi = r.f32(r.eax*4 + 0x16800)
	r.eax = uint32(int32(v.r16(0x1aa88)))
	r.edi += r.eax
	r.ebp = uint32(v.ru16(dsWloglarg))
	subw(&r.ebp, w(r.ecx))
	setl8(&r.eax, v.mem[0x1aa96]&0xf+0x80)
	r.ebx = r.ecx
	for {
		for range r.ecx {
			v.mem[r.edi] = l8(r.eax)
			r.edi++
		}
		r.ecx = r.ebx
		r.edi += r.ebp
		subw(&r.edx, 1)
		if w(r.edx) == 0 {
			break
		}
	}
	goto d789

clipLow:
	if ws(r.eax) > v.r16(dsFeny2) {
		goto d789
	}
	setw(&r.edx, w(r.ebx))
	setw(&r.ebx, w(r.ebx)+w(r.eax)-1)
	if ws(r.ebx) < v.r16(0x1ab38) {
		goto d789
	}
	if ws(r.eax) < v.r16(0x1ab38) {
		setw(&r.edx, w(r.edx)-v.ru16(0x1ab38)+w(r.eax))
		setw(&r.eax, v.ru16(0x1ab38))
	}
	if ws(r.ebx) > v.r16(dsFeny2) {
		setw(&r.edx, w(r.edx)-w(r.ebx)+v.ru16(dsFeny2))
	}
	goto clipped

dc1e:
	r.wallSetup()
	{
		b := ws(r.ebp)
		setw(&r.ebx, w(r.ebp)-w(r.eax))
		if b <= ws(r.eax) {
			goto decc
		}
	}
	if r.g16(r.gdi(2)) != 0 {
		goto e257
	}
	{
		d := ws(r.edx)
		subw(&r.edx, w(r.ebx))
		if d < ws(r.ebx) {
			addw(&r.ebx, w(r.edx))
		}
	}
	r.push(uint32(w(r.edx)))
	setw(&r.edx, w(r.ebx))
	r.ebx = uint32(w(r.ebp))
	subw(&r.ebx, w(r.esi))
	r.wallStep()
	v.wu16(0x1ab14, w(r.ebx))
	seth8(&r.edx, 1)
	r.ebx = 0
	if ws(r.eax) > ws(r.esi) {
		d := w(r.eax) - w(r.esi)
		lo, _ := mulw(d, v.ru16(0x1ab1e))
		setw(&r.ebx, lo)
		lo, _ = mulw(d, v.ru16(0x1ab14))
		seth8(&r.edx, h8(r.edx)+uint8(lo>>8))
	}
	{
		rev := w(r.ebp) != r.g16(0x99e)
		r.texture(false)
		r.ebp += 8
		subw(&r.eax, v.ru16(dsWlogy1))
		r.edi = r.f32(r.eax*4+0x16800) + v.ru32(0x1aa88)
		if rev {
			r.edi += v.ru32(0x1ab16)
			v.wu16(0x1ab20, v.ru16(0x1ab20)+0x2000)
		}
		r.shade()
		seth8(&r.ecx, l8(r.edx))
		setw(&r.esi, v.ru16(0x1ab1a))
		setl8(&r.edx, 0)
		r.wallRows(rev, func(dx *uint32) { addw(dx, v.ru16(0x1ab14)) })
		if rev {
			r.edi -= v.ru32(0x1ab16)
			v.wu16(0x1ab20, v.ru16(0x1ab20)-0x2000)
		}
	}
	setw(&r.edx, uint16(r.pop()))
	if ws(r.edx) <= 0 {
		goto d782
	}
	goto df44

decc:
	r.texture(true)
	r.ebp += r.esi + 8
	subw(&r.eax, v.ru16(dsWlogy1))
	r.edi = r.f32(r.eax*4+0x16800) + v.ru32(0x1aa88)
	r.shade()
	r.ebx = 0
	setw(&r.esi, v.ru16(0x1ab1a))
df44:
	{
		d := ws(r.edx)
		subw(&r.edx, v.ru16(0x1ab36))
		if d > v.r16(0x1ab36) {
			seth8(&r.ecx, l8(r.edx))
			for {
				setl8(&r.ecx, v.mem[0x1ab16])
				r.push(uint32(w(r.ebx)))
				setw(&r.edx, v.ru16(0x1ab20))
				seth8(&r.edx, h8(r.edx)&v.mem[0x1ab22])
				setw(&r.ebx, w(r.ebx)&v.ru16(0x1ab24))
				for {
					v.mem[r.edi] = r.dark(r.texel(true))
					r.edi++
					addw(&r.edx, w(r.esi))
					seth8(&r.edx, h8(r.edx)&v.mem[0x1ab22])
					setl8(&r.ecx, l8(r.ecx)-1)
					if l8(r.ecx) == 0 {
						break
					}
				}
				setw(&r.ebx, uint16(r.pop()))
				addw(&r.ebx, v.ru16(0x1ab1e))
				r.edi += v.ru32(0x1ab0c)
				seth8(&r.ecx, h8(r.ecx)-1)
				if h8(r.ecx) == 0 {
					break
				}
			}
		}
	}
	if v.ru16(0x1ab36) == 0 {
		goto d782
	}
	r.eax = uint32(v.ru16(0x1aaae))
	r.ecx = uint32(v.mem[0x1ab16])
	r.edi = uint32(v.ru16(0x1ab34))
	r.edx = uint32(v.ru16(0x1ab36))
	r.fs = v.fsb()
	goto e011

e011:
	if ws(r.edx) <= 0 {
		goto d782
	}
	setw(&r.esi, v.ru16(0x1aaae))
	setw(&r.ebp, v.ru16(0x1aaac))
	r.ebx = uint32(w(r.ebp))
	{
		b := ws(r.ebx)
		subw(&r.ebx, w(r.esi))
		if b <= ws(r.esi) {
			goto d782
		}
	}
	r.wallStep()
	v.wu16(0x1ab14, w(r.ebx))
	seth8(&r.edx, l8(r.ecx))
	r.ebx = 0
	if ws(r.eax) > ws(r.esi) {
		d := w(r.eax) - w(r.esi)
		lo, _ := mulw(d, v.ru16(0x1ab1e))
		setw(&r.ebx, lo)
		lo, _ = mulw(d, v.ru16(0x1ab14))
		seth8(&r.edx, h8(r.edx)-uint8(lo>>8))
	}
	{
		rev := w(r.ebp) == v.ru16(0x1aaa2)
		r.texture(false)
		r.ebp += 8
		subw(&r.eax, v.ru16(dsWlogy1))
		r.edi = r.f32(r.eax*4+0x16800) + v.ru32(0x1aa88)
		if rev {
			r.edi += v.ru32(0x1ab16)
			v.wu16(0x1ab20, v.ru16(0x1ab20)+0x2000)
		}
		r.shade()
		seth8(&r.ecx, l8(r.edx))
		setw(&r.esi, v.ru16(0x1ab1a))
		setl8(&r.edx, 0xff)
		r.wallRows(rev, func(dx *uint32) { subw(dx, v.ru16(0x1ab14)) })
		if rev {
			v.wu16(0x1ab20, v.ru16(0x1ab20)-0x2000)
		}
	}
	goto d782

e257:
	if r.partWall() {
		goto df44
	}
	goto d782

d782:
	r.fs = v.fsb()
d789:
	r.ecx = r.pop()
	r.edx = r.pop()
	r.esi = r.pop()
	r.edi = r.pop()
	r.ebp = r.pop()
}

// hit is 0xd90f (pointpix): whether the column covers the point
// 0x1aae4/0x1aae6; then the map cell under it, from the caller's cx and
// dx, goes to 0x1aae8.., with 0xfffe for "the land".
func (r *lregs) hit() {
	v := r.v
	if ws(r.eax) < ws(r.ebp) {
		if ws(r.eax)+ws(r.edx) <= ws(r.ebp) {
			return
		}
		setw(&r.edx, w(r.edx)+w(r.ebp)-w(r.eax))
		setw(&r.eax, w(r.ebp))
	}
	if ws(r.eax) > v.r16(0x1aae6) {
		return
	}
	addw(&r.eax, w(r.edx))
	if ws(r.eax) <= v.r16(0x1aae6) {
		return
	}
	setw(&r.ebx, v.ru16(0x1aa88))
	if ws(r.ebx) > v.r16(0x1aae4) {
		return
	}
	addw(&r.ebx, w(r.ecx))
	if ws(r.ebx) <= v.r16(0x1aae4) {
		return
	}
	n := len(r.stk)
	ecx, edx := r.stk[n-1], r.stk[n-2]
	r.ebx = r.f32(uint32(w(ecx))*4 + 0x18b20)
	setw(&r.ebx, w(r.ebx+edx*2))
	setw(&r.eax, 0)
	setl8(&r.eax, r.m8(r.ebx+1))
	v.wu16(0x1aaf0, w(r.eax))
	setl8(&r.eax, r.m8(r.ebx))
	v.wu16(0x1aaec, w(r.eax))
	cl := uint8(r.g16(0x840)) & 31
	r.ecx = ecx&0xffff0000 | uint32(r.g16(0x840))
	setw(&r.eax, (w(ecx)*2-r.g16(0x93c))<<cl>>1)
	v.wu16(0x1aae8, w(r.eax))
	v.wu16(0x1aaea, (w(edx)*2-r.g16(0x940))<<cl>>1)
	v.wu16(0x1aaf2, 0xfffe)
	setw(&r.eax, r.g16(0x922))
	v.wu16(0x1aaee, w(r.eax))
}

// partWall is 0xe257: a wall whose texture has see-through rows from
// gs:[di+2] on; it reports whether its solid rows follow (0xdf44).
func (r *lregs) partWall() bool {
	v := r.v
	{
		d := ws(r.edx)
		subw(&r.edx, w(r.ebx))
		if d < ws(r.ebx) {
			addw(&r.ebx, w(r.edx))
		}
	}
	r.push(uint32(w(r.edx)))
	setw(&r.edx, w(r.ebx))
	setw(&r.ebx, w(r.esi)-v.ru16(0x1ab2e))
	if w(r.ebp) == w(r.ebx) {
		goto e548
	}
	r.push(uint32(w(r.ecx)))
	r.ebx = uint32(w(r.ebp))
	setw(&r.ebx, w(r.ebx)-w(r.esi)+v.ru16(0x1ab2e))
	if w(r.ebx) > 0x40 {
		setw(&r.ebx, w(r.ebx)>>2)
		if w(r.ebx) > 0x40 {
			setw(&r.ebx, 0x40)
		}
		setw(&r.ecx, ((w(r.ecx)-1)<<6)>>1)
		setw(&r.ebx, w(r.ebx)+w(r.ecx)-1)
		setw(&r.ebx, uint16(int16(r.f16(r.ebx*2+0x16b20))>>1))
	} else {
		setw(&r.ecx, (w(r.ecx)-1)<<6)
		setw(&r.ebx, w(r.ebx)+w(r.ecx)-1)
		setw(&r.ebx, r.f16(r.ebx*2+0x16b20))
	}
	v.wu16(0x1ab14, w(r.ebx))
	setw(&r.ecx, uint16(r.pop()))
	v.mem[0x1ab1d] = l8(r.ebx)
	r.ebx = uint32(h8(r.ebx))
	r.ebx = r.f32(r.ebx*4+0x16800) - v.ru32(dsWlogic)
	v.wu32(0x1ab10, r.ebx)
	r.ebx = 0
	if ws(r.eax) > ws(r.esi) {
		lo, _ := mulw(w(r.eax)-w(r.esi), v.ru16(0x1ab1e))
		setw(&r.ebx, lo)
	}
	{
		rev := w(r.ebp) != r.g16(0x99e)
		r.texture(true)
		v.wu32(0x1ab2a, r.esi)
		r.ebp += 8
		subw(&r.eax, v.ru16(dsWlogy1))
		r.esi = r.f32((uint32(l8(r.edx))+r.eax)*4+0x16800) + v.ru32(0x1aa88)
		r.edi = r.f32(r.eax*4+0x16800) + v.ru32(0x1aa88)
		if rev {
			r.edi += v.ru32(0x1ab16)
			v.wu16(0x1ab20, v.ru16(0x1ab20)+0x2000)
			v.mem[0x1ab1d] = -v.mem[0x1ab1d]
		}
		r.shade()
		r.edx = uint32(v.ru16(0x1ab20))
		setl8(&r.ecx, v.mem[0x1ab16])
		for {
			seth8(&r.edx, h8(r.edx)&v.mem[0x1ab22])
			r.push(r.edi)
			r.push(uint32(w(r.ebx)))
			for {
				r.push(uint32(w(r.ebx)))
				setw(&r.ebx, w(r.ebx)&v.ru16(0x1ab24))
				al := r.texel(false)
				setl8(&r.eax, al)
				if al != 0 {
					v.mem[r.edi] = r.dark(al)
				}
				setw(&r.ebx, uint16(r.pop()))
				addw(&r.ebx, v.ru16(0x1ab1e))
				r.edi += v.ru32(0x1ab08)
				if r.edi >= r.esi {
					break
				}
			}
			setw(&r.ebx, uint16(r.pop()))
			r.edi = r.pop()
			d := v.ru32(0x1ab1a)
			if rev {
				b := r.edx < d
				r.edx -= d
				if b {
					r.edi += v.ru32(0x1ab08)
				}
			} else {
				c := r.edx+d < r.edx
				r.edx += d
				if c {
					r.edi += v.ru32(0x1ab08)
				}
			}
			r.edi += v.ru32(0x1ab10)
			if r.edi >= r.esi {
				break
			}
			if rev {
				r.edi--
			} else {
				r.edi++
			}
			setl8(&r.ecx, l8(r.ecx)-1)
			if l8(r.ecx) == 0 {
				break
			}
		}
		if rev {
			v.wu16(0x1ab20, v.ru16(0x1ab20)-0x2000)
		}
	}
	setw(&r.edx, uint16(r.pop()))
	if ws(r.edx) <= 0 {
		return false
	}
	r.ebp += v.ru32(0x1ab2a)
	r.edi = r.esi
	setw(&r.esi, v.ru16(0x1ab1a))
	r.ebx = 0
	return true

e548:
	r.ebx = 0
	if ws(r.eax) > ws(r.esi) {
		lo, _ := mulw(w(r.eax)-w(r.esi), v.ru16(0x1ab1e))
		setw(&r.ebx, lo)
	}
	r.texture(true)
	r.ebp += 8
	v.wu32(0x1ab2a, r.esi)
	subw(&r.eax, v.ru16(dsWlogy1))
	r.edi = r.f32(r.eax*4+0x16800) + v.ru32(0x1aa88)
	r.shade()
	seth8(&r.ecx, l8(r.edx))
	setw(&r.esi, v.ru16(0x1ab1a))
	setl8(&r.edx, 0)
	for {
		setl8(&r.ecx, v.mem[0x1ab16])
		r.push(uint32(w(r.ebx)))
		setw(&r.edx, v.ru16(0x1ab20))
		seth8(&r.edx, h8(r.edx)&v.mem[0x1ab22])
		setw(&r.ebx, w(r.ebx)&v.ru16(0x1ab24))
		for {
			al := r.texel(true)
			setl8(&r.eax, al)
			if al != 0 {
				v.mem[r.edi] = r.dark(al)
			}
			r.edi++
			addw(&r.edx, w(r.esi))
			seth8(&r.edx, h8(r.edx)&v.mem[0x1ab22])
			setl8(&r.ecx, l8(r.ecx)-1)
			if l8(r.ecx) == 0 {
				break
			}
		}
		setw(&r.ebx, uint16(r.pop()))
		addw(&r.ebx, v.ru16(0x1ab1e))
		r.edi += v.ru32(0x1ab0c)
		seth8(&r.ecx, h8(r.ecx)-1)
		if h8(r.ecx) == 0 {
			break
		}
	}
	setw(&r.edx, uint16(r.pop()))
	if ws(r.edx) <= 0 {
		return false
	}
	r.ebp += v.ru32(0x1ab2a)
	r.ebx = 0
	return true
}

// landSprite is 0xfc85: the element at esi drawn into the land, scaled to
// +0x28/+0x2a, shaded unless +0x21 bit 0; with pointpix it is only tested
// for the point instead.
func (v *VM) landSprite(esi uint32) {
	r := &lregs{v: v, esi: esi}
	r.ebx = r.m32(esi + 8)
	r.ebx += r.m32(r.ebx)
	v.wu16(0x1ab46, r.m16(esi+0x28)+1)
	v.wu16(0x1ab44, r.m16(esi+0x2a)+1)
	r.edx = uint32(r.m16(r.ebx+2)) + 1
	v.wu32(0x1ab5a, r.edx)
	r.edx = (uint32(r.m16(r.ebx+4)) + 1) << 8
	{
		d := uint32(v.ru16(0x1ab44))
		if d == 0 || r.edx/d > 0xffff {
			v.fatal("land: division fault (0xfcc8)")
			return
		}
		r.eax = uint32(int32(int16(r.edx/d)) << 8)
	}
	v.wu32(0x1ab4e, r.eax)
	v.wu32(0x1ab52, r.eax)
	setw(&r.eax, r.m16(esi+0xe))
	v.wu16(0x1ab4a, w(r.eax))
	setw(&r.ebp, v.ru16(0x1ab44))
	setw(&r.edx, w(r.eax))
	{
		a1 := w(r.eax) - v.ru16(dsFeny1)
		setw(&r.eax, a1-v.ru16(dsFenh))
		if a1 > v.ru16(dsFenh) || int16(-w(r.eax)-w(r.ebp)) < 0 {
			// 0xfbd5
			if ws(r.edx) > v.r16(dsFeny2) {
				return
			}
			setw(&r.eax, w(r.ebp)+w(r.edx)-1)
			if ws(r.eax) < v.r16(dsFeny1) {
				return
			}
			if ws(r.edx) < v.r16(dsFeny1) {
				setw(&r.ebp, w(r.ebp)-v.ru16(dsFeny1)+w(r.edx))
				setw(&r.edx, v.ru16(dsFeny1))
			}
			if ws(r.eax) > v.r16(dsFeny2) {
				setw(&r.ebp, w(r.ebp)-w(r.eax)+v.ru16(dsFeny2))
			}
		}
	}
	v.wu16(0x1ab44, w(r.ebp))
	v.wu16(0x1ab4c, w(r.edx))
	setw(&r.eax, r.m16(esi+0xc))
	v.wu16(0x1ab48, w(r.eax))
	setw(&r.ebp, v.ru16(0x1ab46))
	setw(&r.ecx, w(r.eax))
	{
		a1 := w(r.eax) - v.ru16(dsFenx1)
		setw(&r.eax, a1-v.ru16(dsFenl))
		if a1 > v.ru16(dsFenl) || int16(-w(r.eax)-w(r.ebp)) < 0 {
			// 0xfc2d
			if ws(r.ecx) > v.r16(dsFenx2) {
				return
			}
			setw(&r.eax, w(r.ebp)+w(r.ecx)-1)
			if ws(r.eax) < v.r16(dsFenx1) {
				return
			}
			if ws(r.ecx) < v.r16(dsFenx1) {
				setw(&r.ebp, w(r.ebp)-v.ru16(dsFenx1)+w(r.ecx))
				setw(&r.ecx, v.ru16(dsFenx1))
			}
			if ws(r.eax) > v.r16(dsFenx2) {
				setw(&r.ebp, w(r.ebp)-w(r.eax)+v.ru16(dsFenx2))
			}
		}
	}
	v.wu16(0x1ab46, w(r.ebp))
	if v.mem[0x1aae2] != 0 {
		r.spriteHit()
		return
	}
	r.edi = v.ru32(dsWlogic)
	lo, hi := mulw(v.ru16(0x1ab4c)-v.ru16(dsWlogy1), v.ru16(dsWloglarg))
	setw(&r.eax, lo+w(r.ecx))
	_ = hi
	r.eax = uint32(w(r.eax))
	r.edi += r.eax
	v.mem[0x1ab5f] = 0
	r.eax = uint32(v.ru16(0x1ab4c) - v.ru16(0x1ab4a))
	if w(r.eax) != 0 {
		r.edx = uint32(int32(v.ru32(0x1ab52)) >> 8)
		lo, hi := mulw(w(r.eax), w(r.edx))
		v.mem[0x1ab5f] = uint8(lo)
		r.eax = uint32(lo>>8 | (hi&0xff)<<8)
		lo, hi = mulw(w(r.eax), v.ru16(0x1ab5a))
		r.eax = uint32(lo)
		r.edx = r.edx&0xffff0000 | uint32(hi)
	}
	r.ebx, r.esi = r.esi, r.ebx
	mirror := r.m8(r.ebx+0x22) != 0
	if mirror {
		r.eax += v.ru32(0x1ab5a) - 1
		v.mem[0x1ab5e] = 0xff
	} else {
		v.mem[0x1ab5e] = 0
	}
	subw(&r.ecx, v.ru16(0x1ab48))
	if w(r.ecx) != 0 {
		e := uint32(int32(v.ru32(0x1ab52)) >> 8)
		lo, hi := mulw(w(e), w(r.ecx))
		dx := uint16(hi&0xff)<<8 | lo>>8
		if mirror {
			v.mem[0x1ab5e] = -uint8(lo) + 0xff
			r.eax -= uint32(dx)
		} else {
			v.mem[0x1ab5e] = uint8(lo)
			addw(&r.eax, dx)
		}
		r.edx = r.edx&0xffff0000 | uint32(dx)
	}
	r.esi = r.eax + r.esi + 8
	direct := r.m8(r.ebx+0x21)&1 != 0
	seth8(&r.ebx, 0xff)
	if !direct {
		r.ebx = uint32(r.m16(0x1ab28))
		setw(&r.ebx, w(r.ebx)<<2)
		bh := -(h8(r.ebx) * 2)
		if bh += r.g8(0x9b0); int8(bh) < 0 {
			bh = 0
		}
		seth8(&r.ebx, bh)
	}
	v.wu32(0x1ab56, uint32(int32(int16(v.ru16(dsWloglarg)-v.ru16(0x1ab46)))))
	setw(&r.ecx, v.ru16(0x1ab4e)&0xff00)
	r.ecx = rol16(r.ecx)
	lo, _ = mulw(v.ru16(0x1ab50), v.ru16(0x1ab5a))
	r.eax = uint32(lo)
	r.edx = uint32(v.mem[0x1ab5f]) << 24
	seth8(&r.edx, v.mem[0x1ab53])
	r.ebp = uint32(v.mem[0x1ab54])
	shaded := int8(h8(r.ebx)) >= 0
	if shaded {
		r.fs = v.ptrdark
	}
	for {
		r.push(r.esi)
		setl8(&r.edx, v.mem[0x1ab5e])
		setw(&r.ecx, v.ru16(0x1ab46))
		for {
			setl8(&r.ebx, r.m8(r.esi))
			if l8(r.ebx) != 0 {
				if shaded {
					setl8(&r.ebx, r.m8(r.fs+r.ebx))
				}
				v.mem[r.edi] = l8(r.ebx)
			}
			r.edi++
			dl, dh := l8(r.edx), h8(r.edx)
			if mirror {
				setl8(&r.edx, dl-dh)
				r.esi -= r.ebp
				if dl < dh {
					r.esi--
				}
			} else {
				setl8(&r.edx, dl+dh)
				r.esi += r.ebp
				if uint16(dl)+uint16(dh) > 0xff {
					r.esi++
				}
			}
			subw(&r.ecx, 1)
			if w(r.ecx) == 0 {
				break
			}
		}
		r.esi = r.pop()
		r.esi += r.eax
		if r.edx+r.ecx < r.edx {
			r.esi += v.ru32(0x1ab5a)
		}
		r.edx += r.ecx
		r.edi += v.ru32(0x1ab56)
		n := v.ru16(0x1ab44) - 1
		v.wu16(0x1ab44, n)
		if n == 0 {
			break
		}
	}
}

// spriteHit is 0xff03: whether the element's visible pixels cover the
// point 0x1aae4/0x1aae6; then its entity, position and image go to
// 0x1aae8...
func (r *lregs) spriteHit() {
	v := r.v
	if ws(r.edx) > v.r16(0x1aae6) {
		return
	}
	addw(&r.edx, v.ru16(0x1ab44))
	if ws(r.edx) <= v.r16(0x1aae6) {
		return
	}
	if ws(r.ecx) > v.r16(0x1aae4) {
		return
	}
	addw(&r.ecx, v.ru16(0x1ab46))
	if ws(r.ecx) <= v.r16(0x1aae4) {
		return
	}
	r.edi = r.ebx
	bl := r.m8(r.esi + 0x22)
	r.ebp = uint32(int32(v.ru32(0x1ab52)) >> 8)
	lo, hi := mulw(v.ru16(0x1aae4)-v.ru16(0x1ab48), w(r.ebp))
	r.ecx = uint32(lo>>8) | uint32(hi&0xff)<<8
	lo, hi = mulw(v.ru16(0x1aae6)-v.ru16(0x1ab4a), w(r.ebp))
	ax := lo>>8 | (hi&0xff)<<8
	setw(&r.ebp, r.m16(r.edi+2)+1)
	r.edx = uint32(ax) * uint32(w(r.ebp))
	if bl != 0 {
		r.edx += uint32(w(r.ebp)) - 1
		r.ecx = -r.ecx
	}
	r.edx += r.ecx
	r.edi = r.edx + r.edi + 8
	if r.m8(r.edi) == 0 {
		return
	}
	if r.m32(uint32(r.m16(r.esi+0x24))+v.ru32(0x15f72)) == 0 {
		return
	}
	v.wu16(0x1aae8, r.m16(r.esi+0x16))
	v.wu16(0x1aaea, r.m16(r.esi+0x18))
	v.wu16(0x1aaec, r.m16(r.esi+0x1a))
	v.wu16(0x1aaf2, r.m16(r.esi+0x24))
	v.wu32(0x1aaf4, r.m32(r.esi+8))
	v.wu16(0x1aaee, r.m16(r.esi+0x10))
}

// landDA41 is 0xda41: the object of map type di on cell (cx, dx), placed
// into the element record at [0x15f98] and drawn by 0xfc85.
func (r *lregs) landDA41() {
	v := r.v
	esi := v.ru32(0x15f98)
	ebp := r.g32(r.gdi(0x10))
	if ebp == 0 {
		return
	}
	cl := uint(r.g16(0x840)) & 31
	ax := int16(w(r.ecx)<<cl) - int16(r.g16(0x994))
	bx := int16(w(r.edx)<<cl) - int16(r.g16(0x996))
	if cx := int16(r.g16(0x922)); cx < v.r16(0x1aab8) {
		a := v.ru32(0x1aa48)
		p := v.landProj(a, ax, bx, int16(v.ru16(0x1aa84)-r.g16(0x998)))
		d := int16(p.ebx)
		if d < v.r16(a+0x92) {
			d = -1
		}
		v.w16(esi+0xc, p.si+int16(r.g16(0x990)))
		v.w16(esi+0xe, p.di+int16(r.g16(0x992)))
		v.w16(esi+0x10, d)
		v.w16(esi+0x2e, p.dx)
		v.w16(esi+0x2c, p.dx)
		if d < 0 {
			return
		}
	} else {
		col, scale := v.landPersp(ax, bx, cx)
		v.w16(esi+0xc, col+int16(r.g16(0x990)))
		e := uint32(v.ru16(0x1aa84)) + r.g32(0x9a8)
		v.wu16(esi+0xe, r.f16(e*2)+r.g16(0x984))
		v.w16(esi+0x10, cx+int16(r.g16(0x858)))
		v.w16(esi+0x2e, scale)
		v.w16(esi+0x2c, scale)
	}
	v.wu16(esi+0x24, r.g16(r.gdi(0x1c)))
	v.mem[esi+0x1c] = r.g8(r.gdi(0x15))
	v.mem[esi+0x21] = 0
	if r.g8(r.gdi(0x14)) == 3 {
		v.mem[esi+0x21] = 1
	}
	hdr := uint32(0x1ab60)
	eax := ebp
	ebx := ebp + v.ru32(ebp)
	if v.mem[ebx] == 3 {
		hdr = ebx
		eax += uint32(int32(v.r16(hdr+2))) * 4
		ebx = eax + v.ru32(eax)
	}
	v.wu32(esi+8, eax)
	v.mem[esi+0x22] = v.mem[hdr+1]
	sh := (v.mem[esi+0x1c] + 9) & 31
	size := func(n uint16) uint16 {
		e := int32((uint32(n+1)*uint32(v.ru16(esi+0x2e)))*2) >> sh
		if e-1 < 0 {
			return 0
		}
		return uint16(e - 1)
	}
	v.wu16(esi+0x28, size(v.ru16(ebx+2)))
	v.wu16(esi+0x2a, size(v.ru16(ebx+4)))
	off := func(n uint16, o uint16, zoom uint32) uint16 {
		p := uint32(n>>1-o) * uint32(v.ru16(zoom))
		return uint16(int32(-p*2) >> sh)
	}
	v.wu16(esi+0xe, v.ru16(esi+0xe)+off(v.ru16(ebx+4), v.ru16(hdr+6), esi+0x2c))
	v.wu16(esi+0xc, v.ru16(esi+0xc)+off(v.ru16(ebx+2), v.ru16(hdr+4), esi+0x2e))
	v.landSprite(esi)
}
