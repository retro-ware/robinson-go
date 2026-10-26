//go:build orig

package orig

import "encoding/binary"

// Linear memory layout of the emulated PC.
const (
	lowSize  = 0x110000 // conventional memory, VGA window, BIOS, HMA
	stubLin  = 0x700    // iretd: default PM interrupt handler (selStub:stubLin)
	mouseLin = 0x780    // buttons, x (0..639), y: the real-mode mouse handler's words
	gdtLin   = 0x800    // GDT (256 descriptors)
	gdtSlots = 256
	dosLin   = 0x20000 // first paragraph for AH=48h
	dosEnd   = 0x9f000
	pspLin   = 0x10000 // PSP (segment 0x1000)
	envLin   = 0x10100 // environment block
	vgaLin   = 0xa0000
	progLin  = 0x10000000 // X-32 program base: DS offset 0 (START.EXE keeps -progLin at 0x14d94)
	progSize = 0x2000000  // 32 MB, as in the shipped DOSBox setup
)

// Selectors the extender hands out at start.
const (
	selCode = 0x08
	selData = 0x10
	selPSP  = 0x18
	selEnv  = 0x20
	selZero = 0x28
	selStub = 0x30 // code at linear 0: default interrupt handlers
	selDyn  = 0x38 // first selector for AX=3501h
)

type descriptor struct {
	base  uint32
	limit uint32 // bytes-1
	code  bool
	used  bool
}

func (m *Machine) writeDesc(sel uint16) {
	d := m.desc[sel>>3]
	b := m.low[gdtLin+int(sel&^7):][:8]
	if !d.used {
		clear(b)
		return
	}
	limit := d.limit
	flags := byte(0x40) // 32-bit
	if limit > 0xfffff {
		limit >>= 12
		flags |= 0x80 // 4 KB granularity
	}
	access := byte(0x92) // present, ring 0, data, writable
	if d.code {
		access = 0x9a // present, ring 0, code, readable
	}
	binary.LittleEndian.PutUint16(b[0:], uint16(limit))
	binary.LittleEndian.PutUint16(b[2:], uint16(d.base))
	b[4] = byte(d.base >> 16)
	b[5] = access
	b[6] = flags | byte(limit>>16)&0x0f
	b[7] = byte(d.base >> 24)
}

func (m *Machine) setDesc(sel uint16, base, limit uint32, code bool) {
	m.desc[sel>>3] = descriptor{base: base, limit: limit, code: code, used: true}
	m.writeDesc(sel)
}

// allocSel returns a free data selector (AX=3501h).
func (m *Machine) allocSel() (uint16, bool) {
	for i := selDyn >> 3; i < gdtSlots; i++ {
		if !m.desc[i].used {
			sel := uint16(i << 3)
			m.setDesc(sel, 0, 0, false)
			return sel, true
		}
	}
	return 0, false
}

// selBase reads a selector's base back from the GDT in memory (the
// program patches descriptors directly).
func (m *Machine) selBase(sel uint16) uint32 {
	b := m.low[gdtLin+int(sel&^7):][:8]
	return uint32(b[2]) | uint32(b[3])<<8 | uint32(b[4])<<16 | uint32(b[7])<<24
}
