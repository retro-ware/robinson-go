//go:build orig

package orig

import (
	"encoding/binary"
	"fmt"
	"sync"
)

// QEMU remembers the last exception in env->old_exception and resets it
// when it delivers one, which unicorn skips when a hook takes it. So the
// program's second divide error became a double fault and the third a
// triple fault, which halts the CPU. clearOldException resets the field
// after each exception the Machine delivers itself.

// oldExcOff is the field's offset in the CPU state, found by probing.
var oldExcOff = sync.OnceValues(probeOldException)

func (e *engine) clearOldException() error {
	off, err := oldExcOff()
	if err != nil {
		return err
	}
	return e.editCPUState(func(b []byte) error {
		binary.LittleEndian.PutUint32(b[off:], 0xffffffff)
		return nil
	})
}

// probeOldException divides by zero twice in a scratch engine: the field
// is the one word that goes from -1 to 0 (the divide error) to 8 (the
// double fault).
func probeOldException() (int, error) {
	e, err := newEngine()
	if err != nil {
		return 0, err
	}
	defer e.close()
	m := &Machine{e: e}
	if m.low, err = e.mapMem(0, lowSize); err != nil {
		return 0, err
	}
	if m.prog, err = e.mapMem(progLin, 0x10000); err != nil {
		return 0, err
	}
	m.setDesc(selCode, progLin, 0xffffffff, true)
	m.setDesc(selData, progLin, 0xffffffff, false)
	if err := e.setGDTR(gdtLin, gdtSlots*8-1); err != nil {
		return 0, err
	}
	for _, r := range []regID{rCS, rDS, rES, rSS} {
		e.setReg(r, selData)
	}
	e.setReg(rCS, selCode)
	e.setReg(rESP, 0x8000)
	copy(m.prog[0x100:], []byte{0x31, 0xd2, 0xf7, 0xf2, 0xf4}) // xor edx,edx; div edx; hlt
	var excs []uint32
	e.onIntr = func(n uint32) {
		excs = append(excs, n)
		e.stop()
	}
	var snaps [3][]byte
	snap := func(i int) error {
		return e.editCPUState(func(b []byte) error {
			snaps[i] = append([]byte(nil), b...)
			return nil
		})
	}
	if err := snap(0); err != nil {
		return 0, err
	}
	for i, eip := range []uint32{0x100, 0x102} {
		if err := e.start(eip); err != nil {
			return 0, err
		}
		if err := snap(i + 1); err != nil {
			return 0, err
		}
	}
	if fmt.Sprint(excs) != "[0 8]" {
		return 0, fmt.Errorf("old_exception probe: exceptions %v, want [0 8]", excs)
	}
	off := -1
	for i := 0; i+4 <= len(snaps[0]); i += 4 {
		w := func(s []byte) uint32 { return binary.LittleEndian.Uint32(s[i:]) }
		if w(snaps[0]) == 0xffffffff && w(snaps[1]) == 0 && w(snaps[2]) == 8 {
			if off >= 0 {
				return 0, fmt.Errorf("old_exception probe: offsets %#x and %#x", off, i)
			}
			off = i
		}
	}
	if off < 0 {
		return 0, fmt.Errorf("old_exception probe: no offset")
	}
	return off, nil
}
