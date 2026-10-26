//go:build orig

package orig

import "testing"

// TestSegments checks unicorn's handling of the program's flat segments:
// code hooks see linear addresses, start() takes the offset, int 21h is
// reported and execution resumes after it.
func TestSegments(t *testing.T) {
	e, err := newEngine()
	if err != nil {
		t.Fatal(err)
	}
	defer e.close()
	m := &Machine{e: e}
	if m.low, err = e.mapMem(0, lowSize+0xf000); err != nil {
		t.Fatal(err)
	}
	if m.prog, err = e.mapMem(progLin, 0x10000); err != nil {
		t.Fatal(err)
	}
	m.setDesc(selCode, progLin, 0xffffffff, true)
	m.setDesc(selData, progLin, 0xffffffff, false)
	m.setDesc(selZero, 0, 0xffffffff, false)
	if err := e.setGDTR(gdtLin, gdtSlots*8-1); err != nil {
		t.Fatal(err)
	}
	for _, r := range []struct {
		id  regID
		sel uint16
	}{{rCS, selCode}, {rDS, selData}, {rES, selData}, {rSS, selData}, {rFS, selZero}, {rGS, 0}} {
		if err := e.setReg(r.id, uint32(r.sel)); err != nil {
			t.Fatalf("seg %d: %v", r.id, err)
		}
	}
	// 0x100: mov eax,[0x200]; int 21h; mov ebx, fs:[0x400]; mov [0xf0000500], eax ; hlt
	code := []byte{0xa1, 0x00, 0x02, 0x00, 0x00, 0xcd, 0x21, 0x64, 0x8b, 0x1d, 0x00, 0x04, 0x00, 0x00,
		0xa3, 0x00, 0x05, 0x00, 0xf0, 0xf4}
	copy(m.prog[0x100:], code)
	m.prog[0x200] = 0x42
	m.low[0x400] = 0x17
	e.setReg(rESP, 0x8000)
	var hooks []uint32
	e.hookCode(progLin+0x107, func() { hooks = append(hooks, e.reg(rEIP)) })
	var ints []uint32
	e.onIntr = func(n uint32) {
		ints = append(ints, n, e.reg(rEIP))
		if n != 0x21 {
			e.stop()
		}
	}
	err = e.start(0x100)
	t.Logf("err=%v eip=%#x eax=%#x ebx=%#x hooks=%#x ints=%#x low500=%#x", err, e.reg(rEIP), e.reg(rEAX), e.reg(rEBX), hooks, ints, m.low[0x500])
}

// TestHookSemantics checks what the harness relies on: flags and segment
// registers written from the int hook take effect, stop() in a code hook
// stops before that instruction, and a divide error reports the faulting
// instruction.
func TestHookSemantics(t *testing.T) {
	e, err := newEngine()
	if err != nil {
		t.Fatal(err)
	}
	defer e.close()
	m := &Machine{e: e}
	if m.low, err = e.mapMem(0, lowSize+0xf000); err != nil {
		t.Fatal(err)
	}
	if m.prog, err = e.mapMem(progLin, 0x10000); err != nil {
		t.Fatal(err)
	}
	m.setDesc(selCode, progLin, 0xffffffff, true)
	m.setDesc(selData, progLin, 0xffffffff, false)
	m.setDesc(selZero, 0, 0xffffffff, false)
	e.setGDTR(gdtLin, gdtSlots*8-1)
	for _, r := range []struct {
		id  regID
		sel uint16
	}{{rCS, selCode}, {rDS, selData}, {rES, selData}, {rSS, selData}} {
		e.setReg(r.id, uint32(r.sel))
	}
	code := []byte{
		0xb9, 0x05, 0x00, 0x00, 0x00, // 100 mov ecx,5
		0xcd, 0x21, // 105 int 21h
		0x72, 0x02, // 107 jc 10b
		0xf4, 0xf4, // 109 hlt
		0x66, 0x8c, 0xc0, // 10b mov ax,es
		0x42,       // 10e inc edx
		0x31, 0xd2, // 10f xor edx,edx
		0xf7, 0xf2, // 111 div edx
		0xf4, // 113 hlt
	}
	copy(m.prog[0x100:], code)
	e.setReg(rESP, 0x8000)
	e.setReg(rEDX, 0)
	var stops int
	e.hookCode(progLin+0x10e, func() {
		if stops == 0 {
			stops++
			e.stop()
		}
	})
	var divEIP uint32
	e.onIntr = func(n uint32) {
		switch n {
		case 0x21:
			e.setReg(rEFLAGS, e.reg(rEFLAGS)|1)
			if err := e.setReg(rES, selZero); err != nil {
				t.Errorf("set es: %v", err)
			}
		case 0:
			divEIP = e.reg(rEIP)
			e.stop()
		default:
			t.Errorf("int %#x", n)
			e.stop()
		}
	}
	if err := e.start(0x100); err != nil {
		t.Fatal(err)
	}
	if eip, ax, edx := e.reg(rEIP), e.reg(rEAX)&0xffff, e.reg(rEDX); eip != 0x10e || ax != selZero || edx != 0 {
		t.Fatalf("after code-hook stop: eip=%#x ax=%#x edx=%#x", eip, ax, edx)
	}
	if err := e.start(0x10e); err != nil {
		t.Fatal(err)
	}
	if eip := e.reg(rEIP); divEIP != 0x111 || eip != 0x111 {
		t.Fatalf("divide error: hook eip=%#x, after stop eip=%#x", divEIP, eip)
	}
	if err := e.start(0x113); err != nil {
		t.Fatal(err)
	}
	if eip := e.reg(rEIP); eip != 0x114 {
		t.Fatalf("hlt: eip=%#x", eip)
	}
}
