//go:build orig

package orig

import (
	"encoding/binary"
	"os"
	"testing"

	"github.com/retro-ware/robinson-go/internal/alis"
)

// TestDivideHandler runs faulting divisions through START.EXE's own divide
// error handler (0x20d9, delivered as the Machine delivers exceptions) and
// checks the port's model of it, alis.DivRetry.
func TestDivideHandler(t *testing.T) {
	if _, err := os.Stat("../../orig/start32.bin"); err != nil {
		t.Skip(err)
	}
	cases := []struct {
		name          string
		code          []byte
		kind          int
		edx, eax, ecx uint32 // ecx is also ebx and esi, and the memory word
	}{
		{"idiv cx", []byte{0x66, 0xf7, 0xf9}, alis.DivSigned | alis.DivWord, 0xabcd00ff, 0x56780000, 1},
		{"idiv cx neg", []byte{0x66, 0xf7, 0xf9}, alis.DivSigned | alis.DivWord, 0x1111ff00, 0x2222e000, 3},
		{"idiv bx", []byte{0x66, 0xf7, 0xfb}, alis.DivSigned | alis.DivWord, 0x0007fff3, 0xfffe1234, 2},
		{"div bx", []byte{0x66, 0xf7, 0xf3}, alis.DivWord, 0x00030020, 0, 0x10},
		{"idiv ecx", []byte{0xf7, 0xf9}, alis.DivSigned, 5, 0, 2},
		{"div ecx cdq", []byte{0xf7, 0xf1}, 0, 0xffffffff, 0xfffffff0, 0x40},
		{"idiv mem", []byte{0x66, 0xf7, 0x3d, 0x00, 0x02, 0x00, 0x00}, alis.DivSigned | alis.DivWord | alis.DivRM4, 0x5a5a0040, 0xa5a50000, 3},
		{"idiv mem neg", []byte{0x66, 0xf7, 0x3d, 0x00, 0x02, 0x00, 0x00}, alis.DivSigned | alis.DivWord | alis.DivRM4, 0x0000ff80, 0x00000001, 1},
		{"div mem", []byte{0x66, 0xf7, 0x35, 0x00, 0x02, 0x00, 0x00}, alis.DivWord | alis.DivRM4, 0x9abc0100, 0x12340000, 1},
		{"div si", []byte{0x66, 0xf7, 0xf6}, alis.DivWord | alis.DivRM4, 0x00000300, 0, 2},
	}
	const code, word = 0x1f00000, 0x1f00100 // unused memory above the program
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m, err := New(Config{Image: "../../orig/start32.bin"})
			if err != nil {
				t.Fatal(err)
			}
			e := m.e
			defer e.close()
			m.pmVec[0] = farPtr{selCode, 0x20d9}
			insn := append([]byte{}, tc.code...)
			if len(insn) == 7 { // the memory operand addresses the word
				binary.LittleEndian.PutUint32(insn[3:], word)
			}
			copy(m.prog[code:], append(insn, 0xf4))
			binary.LittleEndian.PutUint16(m.prog[word:], uint16(tc.ecx))
			e.setReg(rESP, 0x1f80000)
			e.setReg(rEDX, tc.edx)
			e.setReg(rEAX, tc.eax)
			e.setReg(rECX, tc.ecx)
			e.setReg(rEBX, tc.ecx)
			e.setReg(rESI, tc.ecx)
			e.onIntr = func(n uint32) {
				if n != 0 || e.reg(rEIP) != code {
					t.Errorf("int %#x at %#x", n, e.reg(rEIP))
				}
				m.exc = 0
				e.stop()
			}
			end := uint32(code + len(insn) + 1)
			eip, faults := uint32(code), 0
			for ; faults < 100; faults++ {
				if err := e.start(eip); err != nil {
					t.Fatal(err)
				}
				if eip = e.reg(rEIP); m.exc < 0 {
					break
				}
				m.exc = -1
				if err := e.clearOldException(); err != nil {
					t.Fatal(err)
				}
				eip = m.deliver(0, eip)
			}
			if eip != end {
				t.Fatalf("stopped at %#x after %d faults", eip, faults)
			}
			eax, edx, ok := alis.DivRetry(tc.kind, tc.edx, tc.eax, tc.ecx)
			if !ok || eax != e.reg(rEAX) || edx != e.reg(rEDX) {
				t.Errorf("%d faults: START.EXE eax %#x edx %#x, port %#x %#x (ok %v)", faults, e.reg(rEAX), e.reg(rEDX), eax, edx, ok)
			}
		})
	}
}
