// Package vmstate writes the state of the running scripts in a form that
// doesn't depend on where the arena lies, so the Go VM and START.EXE (the
// orig package) can be compared.
package vmstate

import (
	"fmt"
	"io"
)

// Mem reads the arena at an address of the VM's own.
type Mem func(addr uint32) byte

func (m Mem) r16(a uint32) uint16 { return uint16(m(a)) | uint16(m(a+1))<<8 }
func (m Mem) r32(a uint32) uint32 { return uint32(m.r16(a)) | uint32(m.r16(a+2))<<16 }

// Scripts writes every entity in the order the main loop runs them (the
// list at atent: dword context, word next): its context header, with the
// code pointers relative to the script data, and its variables.
func Scripts(w io.Writer, m Mem, atent uint32) {
	for ent, n := uint16(0), 0; n < 256; n++ {
		o := m.r32(atent + uint32(ent))
		data := m.r32(o - 0x14)
		size := uint32(m.r16(data + 0x14))
		fmt.Fprintf(w, "ent %d id %#x pc %#x size %#x\n", ent, m.r16(o-0x10), m.r32(o-8)-data, size)
		hexdump(w, m, o-0x3e, o-0x14, -0x3e)
		hexdump(w, m, o-0x10, o-8, -0x10)
		hexdump(w, m, o-4, o, -4)
		hexdump(w, m, o, o+size, 0)
		if ent = m.r16(atent + uint32(ent) + 4); ent == 0 {
			break
		}
	}
}

func hexdump(w io.Writer, m Mem, from, to uint32, off int) {
	for a := from; a < to; a += 32 {
		fmt.Fprintf(w, "%5x:", off+int(a-from))
		for i := a; i < min(a+32, to); i++ {
			fmt.Fprintf(w, " %02x", m(i))
		}
		fmt.Fprintln(w)
	}
}
