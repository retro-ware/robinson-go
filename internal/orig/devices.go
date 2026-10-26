//go:build orig

package orig

import (
	"fmt"
	"sort"
)

func (m *Machine) int10() {
	e := m.e
	ax := e.reg(rEAX) & 0xffff
	switch ax >> 8 {
	case 0x00:
		if ax&0x7f == 0x13 {
			clear(m.low[vgaLin : vgaLin+0x10000])
		}
		m.vga.mode = uint8(ax & 0x7f)
		m.logf("video mode %#x", ax&0x7f)
	case 0x01, 0x02, 0x09, 0x0e:
	case 0x03:
		e.setReg(rECX, e.reg(rECX)&^0xffff|0x0607)
		e.setReg(rEDX, e.reg(rEDX)&^0xffff)
	case 0x0f:
		m.setAX(uint16(80<<8 | uint32(m.vga.mode)))
		e.setReg(rEBX, e.reg(rEBX)&^0xff00)
	case 0x12: // EGA info (BL=10h): colour, 256 KB
		if e.reg(rEBX)&0xff == 0x10 {
			e.setReg(rEBX, e.reg(rEBX)&^0xffff|0x0003)
			e.setReg(rECX, e.reg(rECX)&^0xffff|0x0009)
		}
	case 0x1a: // display combination: VGA colour
		if ax&0xff == 0 {
			m.setAX(0x001a)
			e.setReg(rEBX, e.reg(rEBX)&^0xffff|0x0008)
		}
	default:
		m.fail(fmt.Errorf("int 10h ax=%#04x at eip %#x", ax, e.reg(rEIP)))
	}
}

// mouse: a two-button driver whose event handler (installed by the
// real-mode helper) keeps buttons, x (0..639) and y in mouseLin.
type mouse struct {
	rm      bool
	buttons uint16
	x, y    uint16
}

func (m *Machine) setMouse(buttons, x, y uint16) {
	m.mouse.buttons, m.mouse.x, m.mouse.y = buttons, x, y
	if m.mouse.rm {
		m.w16(mouseLin, buttons)
		m.w16(mouseLin+2, x)
		m.w16(mouseLin+4, y)
	}
}

func (m *Machine) int33() {
	e := m.e
	ax := e.reg(rEAX) & 0xffff
	switch ax {
	case 0:
		m.setAX(0xffff)
		e.setReg(rEBX, e.reg(rEBX)&^0xffff|2)
	case 1, 2, 7, 8:
	case 3:
		e.setReg(rEBX, e.reg(rEBX)&^0xffff|uint32(m.mouse.buttons))
		e.setReg(rECX, e.reg(rECX)&^0xffff|uint32(m.mouse.x))
		e.setReg(rEDX, e.reg(rEDX)&^0xffff|uint32(m.mouse.y))
	case 4:
		m.mouse.x, m.mouse.y = uint16(e.reg(rECX)), uint16(e.reg(rEDX))
	default:
		m.fail(fmt.Errorf("int 33h ax=%#04x at eip %#x", ax, e.reg(rEIP)))
	}
}

// kbd is the keyboard controller with scripted scancodes.
type kbd struct {
	data   uint8
	portB  uint8
	events []keyEvent // sorted by time
	bios   []uint16   // BIOS keyboard buffer: scancode<<8 | ASCII
	ext    uint16     // scancode still to return after a 0 (extended key)
}

// US layout ASCII of the unshifted keys the setup prompt reacts to.
var biosASCII = map[uint8]uint8{0x01: 0x1b, 0x1c: 0x0d, 0x39: ' ', 0x1f: 's', 0x34: '.'}

type keyEvent struct {
	t               uint64
	code            uint8 // scancode (bit 7 = release); 0 with mouse set = mouse event
	isMouse         bool
	buttons, mx, my uint16
}

// Key queues a scancode at virtual time t (PIT clocks).
func (m *Machine) Key(t uint64, code uint8) {
	m.kbd.events = append(m.kbd.events, keyEvent{t: t, code: code})
	sort.SliceStable(m.kbd.events, func(i, j int) bool { return m.kbd.events[i].t < m.kbd.events[j].t })
}

// Mouse queues a mouse state (x in 0..639, y in 0..199) at time t.
func (m *Machine) Mouse(t uint64, buttons, x, y uint16) {
	m.kbd.events = append(m.kbd.events, keyEvent{t: t, isMouse: true, buttons: buttons, mx: x, my: y})
	sort.SliceStable(m.kbd.events, func(i, j int) bool { return m.kbd.events[i].t < m.kbd.events[j].t })
}

func (k *kbd) next() (uint64, bool) {
	if len(k.events) == 0 {
		return 0, false
	}
	return k.events[0].t, true
}

// run applies the input events due. A scancode waits until the previous
// one has been taken (IRQ 1 delivered).
func (k *kbd) run(m *Machine) {
	for len(k.events) > 0 && k.events[0].t <= m.clk.t {
		ev := k.events[0]
		if ev.isMouse {
			m.setMouse(ev.buttons, ev.mx, ev.my)
		} else {
			if m.pic.irr&2 != 0 || m.pic.isr&2 != 0 {
				// still busy: retry one tick later
				k.events[0].t = m.clk.t + 1
				return
			}
			k.data = ev.code
			m.pic.raise(1)
		}
		k.events = k.events[1:]
	}
}

func (m *Machine) in(port uint16, size int) uint32 {
	switch {
	case port == 0x20 || port == 0x21:
		if port == 0x21 {
			return uint32(m.pic.imr)
		}
		return 0
	case port >= 0x40 && port <= 0x42:
		return uint32(m.pit.read(m, port))
	case port == 0x60:
		return uint32(m.kbd.data)
	case port == 0x61:
		return uint32(m.kbd.portB)
	case port == 0x64:
		return 0x1c
	case port == 0x3da:
		return uint32(m.status3DA())
	case port == 0x3c2:
		return 0
	case port == 0x3c9:
		v := m.vga.dac[m.vga.rIdx%768]
		m.vga.rIdx++
		return uint32(v)
	case port == 0x3d5:
		return uint32(m.vga.crtc[m.vga.crtcIdx&31])
	case port == 0x3c5:
		return uint32(m.vga.seq[m.vga.seqIdx&15])
	case port == 0x3cf:
		return uint32(m.vga.gc[m.vga.gcIdx&15])
	case port == 0x3cc:
		return uint32(m.vga.misc)
	case port == 0x388 || port == 0x389:
		return uint32(m.opl.status(m))
	case port >= 0x220 && port < 0x230:
		return uint32(m.sb.in(m, port-0x220))
	case port < 0x10 || port >= 0x80 && port < 0x90 || port >= 0xc0 && port < 0xe0:
		return uint32(m.dma.in(m, port))
	}
	m.unknownPort(port, "in")
	return 0xffffffff
}

func (m *Machine) out(port uint16, size int, val uint32) {
	if size == 2 {
		m.out(port, 1, val&0xff)
		m.out(port+1, 1, val>>8&0xff)
		return
	}
	v := uint8(val)
	switch {
	case port == 0x20 || port == 0x21:
		m.pic.write(m, port, v)
		if m.pic.pending() {
			m.e.stop()
		}
	case port == 0xa0 || port == 0xa1:
	case port >= 0x40 && port <= 0x43:
		m.pit.write(m, port, v)
	case port == 0x61:
		m.kbd.portB = v
	case port == 0x3c8:
		m.vga.wIdx = int(v) * 3
	case port == 0x3c7:
		m.vga.rIdx = int(v) * 3
	case port == 0x3c9:
		m.vga.dac[m.vga.wIdx%768] = v & 0x3f
		m.vga.wIdx = (m.vga.wIdx + 1) % 768
	case port == 0x3d4:
		m.vga.crtcIdx = v
	case port == 0x3d5:
		m.vga.crtc[m.vga.crtcIdx&31] = v
	case port == 0x3c4:
		m.vga.seqIdx = v
	case port == 0x3c5:
		m.vga.seq[m.vga.seqIdx&15] = v
	case port == 0x3ce:
		m.vga.gcIdx = v
	case port == 0x3cf:
		m.vga.gc[m.vga.gcIdx&15] = v
	case port == 0x3c2:
		m.vga.misc = v
	case port == 0x3c0:
	case port == 0x388:
		m.opl.idx = v
	case port == 0x389:
		m.opl.write(m, v)
	case port >= 0x220 && port < 0x230:
		m.sb.out(m, port-0x220, v)
	case port < 0x10 || port >= 0x80 && port < 0x90 || port >= 0xc0 && port < 0xe0:
		m.dma.out(m, port, v)
	default:
		m.unknownPort(port, fmt.Sprintf("out %#x", v))
	}
}

func (m *Machine) unknownPort(port uint16, what string) {
	if !m.unknownPorts[port] {
		m.unknownPorts[port] = true
		m.logf("port %#x: %s at eip %#x", port, what, m.e.reg(rEIP))
	}
}

// opl logs the register writes of the OPL2 at 388h with their time; its
// timers expire at once (the CPU is infinitely fast).
type opl struct {
	idx    uint8
	regs   [256]uint8
	flags  uint8
	Writes []OPLWrite
}

// OPLWrite is one OPL register write.
type OPLWrite struct {
	T        uint64
	Reg, Val uint8
}

func (o *opl) write(m *Machine, v uint8) {
	o.regs[o.idx] = v
	if o.idx == 4 {
		if v&0x80 != 0 {
			o.flags = 0
		} else {
			if v&1 != 0 && v&0x40 == 0 {
				o.flags |= 0xc0
			}
			if v&2 != 0 && v&0x20 == 0 {
				o.flags |= 0xa0
			}
		}
		return
	}
	o.Writes = append(o.Writes, OPLWrite{m.clk.t, o.idx, v})
}

func (o *opl) status(m *Machine) uint8 { return o.flags | 6 }
