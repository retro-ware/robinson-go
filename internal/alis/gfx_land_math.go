package alis

// The land renderer's arithmetic (START.EXE 0xc8be..0xcdbf). gs addresses
// the land variables: 0xc00 bytes below the map variable (0x472 keeps the
// base at 0x14e1c). Angles are 9-bit fixed point sines and cosines; the
// helpers keep the register widths of the original, so that the overflows
// and truncations come out the same.

const dsGsBase = 0x14e1c

func (v *VM) gs(off uint32) uint32          { return v.ru32(dsGsBase) + off }
func (v *VM) gs16(off uint32) int16         { return v.r16(v.gs(off)) }
func (v *VM) gsu16(off uint32) uint16       { return v.ru16(v.gs(off)) }
func (v *VM) gs32(off uint32) uint32        { return v.ru32(v.gs(off)) }
func (v *VM) setGs16(off uint32, x int16)   { v.w16(v.gs(off), x) }
func (v *VM) setGsu16(off uint32, x uint16) { v.wu16(v.gs(off), x) }
func (v *VM) setGs32(off uint32, x uint32)  { v.wu32(v.gs(off), x) }

// mul16 is imul r/m16 with the product taken back into edx (rol edx,16;
// mov dx,ax).
func mul16(a, b int16) int32 { return int32(a) * int32(b) }

// clamp16 is the saturation after the 32-bit divisions.
func clamp16(x int32) int16 {
	if x > 0x7fff {
		return 0x7fff
	}
	if x < -0x8000 {
		return -0x8000
	}
	return int16(x)
}

// posDiv is "mov cx,di; add cx,m; jg; mov cx,1": the divisor, 1 unless
// the sum is positive (jg tests the true sign); it wraps to 16 bits.
func posDiv(a, b int16) int32 {
	if int32(a)+int32(b) > 0 {
		return int32(a + b)
	}
	return 1
}

// idiv16 is idiv r/m16 on dx:ax; a quotient outside 16 bits faults.
func (v *VM) idiv32by16(n int32, d int16) int16 {
	if d == 0 || n/int32(d) > 0x7fff || n/int32(d) < -0x8000 {
		v.fatal("land: division fault")
		return 0
	}
	return int16(n / int32(d))
}

// landRot is 0xc8be: (ax, bx) turned by the camera's heading around the
// eye (gs 0x994, 0x996), and their map cells; cx is lifted by the eye.
type landRot struct {
	eax, ebx uint32
	ecx      int32
	si, di   int16
}

func (v *VM) landRot(ax, bx, cx int16) landRot {
	c, s := v.gs16(0x972), v.gs16(0x970)
	esi := mul16(ax, c) - mul16(bx, s)
	edi := mul16(ax, s) + mul16(bx, c)
	eax := uint32(esi >> 9)
	ebx := uint32(edi >> 9)
	ax = int16(eax) + v.gs16(0x994)
	bx = int16(ebx) + v.gs16(0x996)
	eax = eax&0xffff0000 | uint32(uint16(ax))
	ebx = ebx&0xffff0000 | uint32(uint16(bx))
	sh := uint(v.gsu16(0x840)) & 31
	return landRot{eax: eax, ebx: ebx, ecx: int32(cx + v.gs16(0x998)), si: ax >> sh, di: bx >> sh}
}

// landProj is 0xc946: a point relative to the camera of scene a, turned
// by the scene's heading (+0xb2, +0xb4) and projected. Behind the near
// plane (+0x94) it is (0, 0) at depth -1.
type landProj struct {
	eax, ebx, ecx uint32
	dx            int16 // zoom
	si, di        int16 // screen x, y
}

func (v *VM) landProj(a uint32, ax, bx, cx int16) landProj {
	c, s := v.r16(a+0xb4), v.r16(a+0xb2)
	edi := -mul16(ax, c) + mul16(bx, s)
	esi := mul16(ax, s) + mul16(bx, c)
	esi >>= 9
	edi >>= 9
	di := int16(edi)
	if di > v.r16(a+0x94) {
		return landProj{eax: 0, ebx: 0xffffffff, ecx: 0, dx: 0x100}
	}
	edx := mul16(-di, v.r16(a+0xac)) >> 9
	ecx := -(int32(cx) + edx)
	esi = mul16(v.r16(a+0x76), int16(esi))
	z := v.r16(a + 0x7a)
	ecx = mul16(z, int16(ecx))
	ebx := mul16(0x100, z)

	d := posDiv(di, v.r16(a+0x70))
	x := clamp16(esi / d)
	v.w16(0x1aade, x)
	d = posDiv(di, v.r16(a+0x74))
	y := clamp16(ecx / d)
	v.w16(0x1aae0, y)
	zoom := clamp16(ebx / d)
	return landProj{eax: uint32(esi), ebx: uint32(edi), ecx: uint32(ecx), dx: zoom, si: x, di: y}
}

// landPersp is 0xcab9: the screen column of map point (ax, bx) at depth
// cx by the land's lens (gs 0x854..0x85a), and the scale at that depth.
func (v *VM) landPersp(ax, bx, cx int16) (int16, int16) {
	r := (mul16(bx, v.gs16(0x970)) + mul16(ax, v.gs16(0x972))) >> 9
	col := v.idiv32by16(mul16(v.gs16(0x856), int16(r)), cx+v.gs16(0x854))
	scale := v.idiv32by16(int32(v.gs16(0x85a))<<8, cx+v.gs16(0x858))
	return col, scale
}

// landPersp2 is 0xcb2c: the screen columns of (ax, bx) and (cx, dx) at
// depth di.
func (v *VM) landPersp2(ax, bx, cx, dx, di int16) (int16, int16) {
	c, s := v.gs16(0x972), v.gs16(0x970)
	bp := (mul16(ax, c) + mul16(bx, s)) >> 9
	b := (mul16(cx, c) + mul16(dx, s)) >> 9
	d := di + v.gs16(0x854)
	q2 := v.idiv32by16(mul16(v.gs16(0x856), int16(b)), d)
	q1 := v.idiv32by16(mul16(v.gs16(0x856), int16(bp)), d)
	return q1, q2
}

// landPersp3 is 0xcbc6: as 0xcb2c, but at the depth of (ax, bx) itself,
// which it returns, with 32-bit divisions that saturate.
func (v *VM) landPersp3(ax, bx, cx, dx int16) (x1, x2, depth int16) {
	c, s := v.gs16(0x972), v.gs16(0x970)
	edi := (-mul16(ax, s) + mul16(bx, c)) >> 9
	bp := (mul16(ax, c) + mul16(bx, s)) >> 9
	b := (mul16(cx, c) + mul16(dx, s)) >> 9
	d := int32(int16(edi) + v.gs16(0x854))
	if d == 0 {
		v.fatal("land: division by zero (0xcc78)")
		return
	}
	x2 = clamp16(mul16(v.gs16(0x856), int16(b)) / d)
	x1 = clamp16(mul16(v.gs16(0x856), int16(bp)) / d)
	return x1, x2, int16(edi)
}

// landUnproj is 0xccb9: the map point under screen point (ax, bx) at
// depth cx.
func (v *VM) landUnproj(ax, bx, cx int16) (int16, int16, int16) {
	c, s := v.gs16(0x972), v.gs16(0x970)
	x := ax - v.gs16(0x990)
	r := mul16(x, c) >> 9
	px := v.idiv32by16(mul16(cx+v.gs16(0x854), int16(r)), v.gs16(0x856))
	px -= int16(mul16(cx, s) >> 9)
	px += v.gs16(0x994)
	r = mul16(x, s) >> 9
	py := v.idiv32by16(mul16(cx+v.gs16(0x854), int16(r)), v.gs16(0x856))
	py += int16(mul16(cx, c) >> 9)
	py += v.gs16(0x996)
	y := -bx + v.gs16(0x992)
	pz := v.idiv32by16(mul16(cx+v.gs16(0x858), y), v.gs16(0x85a))
	pz += int16(mul16(cx, v.gs16(0x96c)) >> 9)
	pz += v.gs16(0x998)
	return px, py, pz
}

// idiv32 is cdq; idiv r32.
func (v *VM) idiv32(n, d int32) int32 {
	if d == 0 || n == -0x80000000 && d == -1 {
		v.fatal("land: division fault")
		return 0
	}
	return n / d
}

func abs16(x int16) int16 {
	if x < 0 {
		return -x
	}
	return x
}

// unitSign is "mov eax,0x10000; jns; mov eax,0xffff0000".
func unitSign(neg bool) uint32 {
	if neg {
		return 0xffff0000
	}
	return 0x10000
}

// landF445 is 0xf445: the steps of the floor scan between the view's
// corners, its shading and the depth steps of its rows.
func (v *VM) landF445(a uint32) {
	cl := uint(v.gsu16(0x840)) & 31
	dx0 := v.gs16(0x888) - v.gs16(0x882)
	v.w16(0x1ab40, dx0)
	dy0 := v.gs16(0x88a) - v.gs16(0x884)
	v.w16(0x1ab42, dy0)
	ax, bx := abs16(dx0), abs16(dy0)
	if ax < bx {
		v.setGs16(0x950, bx)
		v.setGs16(0x952, abs16(v.gs16(0x896)-v.gs16(0x890)))
		q := v.idiv32(int32(dx0)<<16, int32(bx))
		v.setGs32(0x93c, uint32(q))
		v.setGs32(0x940, unitSign(dy0 < 0))
		t := int16(mul16(v.gs16(0x884)-v.gs16(0x890), int16(q>>2)) >> 14)
		if int32(v.gs32(0x940)) < 0 {
			t = -t
		}
		v.setGs16(0x9b6, abs16(t+v.gs16(0x88e)-v.gs16(0x882))>>cl)
	} else {
		v.setGs16(0x950, ax)
		v.setGs16(0x952, abs16(v.gs16(0x894)-v.gs16(0x88e)))
		q := v.idiv32(int32(dy0)<<16, int32(ax))
		v.setGs32(0x940, uint32(q))
		v.setGs32(0x93c, unitSign(dx0 < 0))
		t := int16(mul16(v.gs16(0x882)-v.gs16(0x88e), int16(q>>2)) >> 14)
		if int32(v.gs32(0x93c)) < 0 {
			t = -t
		}
		v.setGs16(0x9b6, abs16(t+v.gs16(0x890)-v.gs16(0x884))>>cl)
	}
	n := int32(v.gs16(0x9b6))
	s := v.idiv32(int32(v.gs16(0x88e)-v.gs16(0x882))<<16, n) >> cl
	v.setGs32(0x944, uint32(s))
	t := v.idiv32(int32(v.gs16(0x890)-v.gs16(0x884))<<16, n) >> cl
	v.setGs32(0x948, uint32(t))
	v.setGs32(0x95c, unitSign(s < 0))
	v.setGs32(0x960, unitSign(t < 0))
	q := v.idiv32by16(int32(v.gs16(0x952)-v.gs16(0x950))<<8, -0x24a)
	v.setGs32(0x94c, uint32(int32(q)<<8>>cl))

	v.setGs32(0x9b2, 0)
	v.setGs32(0x9ae, uint32(v.ru16(a+0x9e))<<16)
	if v.ru16(a+0x98) != 0 {
		v.setGs32(0x9ae, 0x60000)
		d := v.r16(a + 0x9e)
		if d <= 6 {
			v.setGs32(0x9ae, 0)
		}
		if v.r16(a+0x98) >= 0 {
			v.setGs32(0x9ae, uint32(v.ru16(a+0xa0))<<16-1)
		}
		e := int32(uint32(uint16(d))<<16 - v.gs32(0x9ae))
		v.setGs32(0x9b2, uint32(v.idiv32(e, n+1)))
	}
	step := func(dst, from, to uint32, sh8 bool) {
		e := int32(v.gs32(to) - v.gs32(from))
		if sh8 {
			v.setGs32(dst, uint32(v.idiv32(e<<8, n)<<8))
		} else {
			v.setGs32(dst, uint32(v.idiv32(e, n)))
		}
	}
	step(0x91c, 0x8d0, 0x8e8, false)
	step(0x924, 0x904, 0x910, true)
	step(0x92c, 0x8d8, 0x8f0, false)
	step(0x934, 0x908, 0x914, true)
	v.setGs32(0x918, v.gs32(0x8d0))
	v.setGs32(0x920, v.gs32(0x8d4)<<16)
	v.setGs32(0x928, v.gs32(0x8d8))
	v.setGs32(0x930, v.gs32(0x908))
}

// landF7E6 is 0xf7e6 (openland): the depth of each of the 0xb3 rows
// below the horizon, 0xff steps per row, into 0x1aa60.
func (v *VM) landF7E6(a uint32) {
	m := v.r16(a + 0x6c)
	v.w16(a+0x6c, m+v.gs16(0x80c))
	v.w16(a+0x66, v.r16(a+0x66)-v.gs16(0x80c))
	v.landC365(a)
	v.w16(a+0x6c, v.r16(a+0x6c)-v.gs16(0x80c))
	v.w16(a+0x66, v.r16(a+0x66)+v.gs16(0x80c))
	v.setGs32(0x938, v.gs32(0x8d4)<<16)
	esi := v.ru32(0x1aa60)
	edi := v.idiv32(int32(v.gs32(0x8ec)-v.gs32(0x8d4))<<8, 0xb3)
	ebx := v.idiv32(int32(v.gs32(0x914)-v.gs32(0x908)), 0xb3)
	eax := v.gs32(0x908)
	ebp := v.gs32(0x8d4) << 16
	edi--
	v.setGsu16(0x99a, uint16(edi))
	edi <<= 8
	for range 0xb3 {
		cx := int16(ebp>>16) + v.gs16(0x858)
		q := int32(v.idiv32by16(int32(eax), cx)) << 8
		v.wu16(esi, 0x4e20)
		esi += 2
		edx := q
		for range 0xff {
			v.w16(esi, -int16(edx>>16))
			esi += 2
			edx += q
		}
		ebp += uint32(edi)
		eax += uint32(ebx)
	}
	for range 0x100 {
		v.wu16(esi, 0x4e20)
		esi += 2
	}
	// 0xf8d9 computes the sky's scroll and stores cx, 0x168
	q := v.idiv32by16(mul16(int16(mul16(0x648/0x168, m)), 0x168), 0x140)
	q = int16(uint16(q+0x80) & 0xff00)
	v.idiv32by16(mul16(q, 0x140), 0x168)
	v.wu16(0x1aabc, 0x168)
}

// landF91C is 0xf91c (openland): the row tables of the map and of the
// scene's buffer, and the table of (n-1)*256/d at 0x1aa70.
func (v *VM) landF91C(a uint32) {
	t := v.r16(a+0x6c) - v.r16(a+0x66)
	v.w16(0x1aab8, t>>2)
	v.w16(0x1aaba, t>>4)
	v.setGs16(0x964, 0)
	v.setGs16(0x966, 0)
	v.setGsu16(0x968, v.gsu16(0x818))
	v.setGsu16(0x96a, v.gsu16(0x81a))
	ebx := uint32(v.gsu16(0x836))
	n := uint32(v.gsu16(0x834))
	eax := v.ru32(dsGsBase) + 0xc00
	if int8(v.mem[v.gs(0xbff)]) < 0 {
		if eax = v.gs32(0xc00); eax == 0 {
			v.alisError(0x1a)
			return
		}
		eax = v.ru32(eax)
	}
	if n == 0 {
		v.fatal("land: loop of 2^32 rows (0xf9cb)")
		return
	}
	esi := v.ru32(0x1aa50)
	for i := range n {
		v.wu32(esi+i*4, eax+i*ebx*2)
	}
	buf := v.ru32(v.ru32(a + 0x30))
	esi = v.ru32(0x1aa58)
	for i := range uint32(0xc8) {
		v.wu32(esi+i*4, buf+i*0x140)
	}
	edi := v.ru32(0x1aa70)
	for d := uint16(1); d < 0x41; d++ {
		for c := uint16(1); c < 0x41; c++ {
			v.wu16(edi, (c-1)<<8/d)
			edi += 2
		}
	}
}

// landFA29 is 0xfa29 (openland): the lens of the scene from its view
// (+0x64..+0x6e) and size.
func (v *VM) landFA29(a uint32) {
	v.w16(a+0x94, v.r16(a+0x6c)-2*v.gs16(0x80e))
	v.w16(a+0x92, v.r16(a+0x66)>>2)
	lens := func(n0, n1, d0, d1, size uint32) (int16, int16) {
		e := uint32(v.ru16(a+0x6c))*uint32(v.ru16(a+n0)) - uint32(v.ru16(a+0x66))*uint32(v.ru16(a+n1))
		f := v.idiv32by16(int32(e), v.r16(a+d1)-v.r16(a+d0))
		g := v.idiv32by16(mul16(v.r16(a+0x6c)+f, v.r16(a+size)+1), v.r16(a+d1))
		return f, g
	}
	f, g := lens(0x64, 0x6a, 0x64, 0x6a, 0x12)
	v.w16(a+0x70, f)
	v.w16(a+0x76, g)
	f, g = lens(0x68, 0x6e, 0x68, 0x6e, 0x14)
	v.w16(a+0x74, f)
	v.w16(a+0x7a, g)
	v.setGs16(0x854, v.r16(a+0x70))
	v.setGs16(0x856, v.r16(a+0x76))
	v.setGs16(0x858, v.r16(a+0x74))
	v.setGs16(0x85a, v.r16(a+0x7a))
	v.setGs16(0x85c, v.idiv32by16(int32(v.r16(a+0x6a)), v.gs16(0x80c)))
}
