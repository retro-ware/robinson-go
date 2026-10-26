//go:build orig

package orig

import "fmt"

// Virtual time, in PIT input clocks (1.193182 MHz). The CPU is infinitely
// fast: time only advances where the program waits (wait-point hooks and
// repeated polls of a status port), straight to the next event.

const (
	pitHz    = 1193182
	vgaDotHz = 25175000
	vgaLineD = 800 // dots per line
	vgaLines = 449 // lines per frame (70 Hz modes)
	vgaFrame = vgaLineD * vgaLines
)

type clock struct {
	t       uint64
	limit   uint64 // stop here (0 = none)
	runFor  uint64 // limit after the game timer starts
	started bool   // the game timer runs
}

// gameTimer notes the start of the game timer at t: input and the run
// limit count from here, as the alis package's clock.
func (m *Machine) gameTimer(t uint64) {
	if m.clk.started {
		return
	}
	m.clk.started = true
	if m.clk.runFor != 0 {
		m.clk.limit = t + m.clk.runFor
	}
	m.kbd.arm(t)
}

func (c *clock) init() {}

func (c *clock) ms() float64 { return float64(c.t) * 1000 / pitHz }

// pic is the master 8259A (IRQ 0-7), fully nested, normal EOI.
type pic struct {
	irr, isr, imr uint8
}

func (p *pic) raise(irq int) { p.irr |= 1 << irq }

// take returns the IRQ to deliver now, if any, and marks it in service.
func (p *pic) take() (int, bool) {
	for i := 0; i < 8; i++ {
		b := uint8(1) << i
		if p.isr&b != 0 {
			return 0, false // a higher or equal priority IRQ is in service
		}
		if p.irr&b != 0 && p.imr&b == 0 {
			p.irr &^= b
			p.isr |= b
			return i, true
		}
	}
	return 0, false
}

func (p *pic) pending() bool {
	for i := 0; i < 8; i++ {
		b := uint8(1) << i
		if p.isr&b != 0 {
			return false
		}
		if p.irr&b != 0 && p.imr&b == 0 {
			return true
		}
	}
	return false
}

func (p *pic) eoi(irq int) { p.isr &^= 1 << irq }

func (p *pic) write(m *Machine, port uint16, v uint8) {
	switch {
	case port == 0x21:
		p.imr = v
	case v == 0x20: // non-specific EOI: highest priority in service
		for i := 0; i < 8; i++ {
			if p.isr&(1<<i) != 0 {
				p.isr &^= 1 << i
				break
			}
		}
	case v&0xf8 == 0x60: // specific EOI
		p.isr &^= 1 << (v & 7)
	case v&0x18 == 0x08: // OCW3
	default:
		m.logf("pic: port %#x <- %#x", port, v)
	}
}

// pit channel 0 drives IRQ 0. As in DOSBox, writing a new count restarts
// the period.
type pit struct {
	div    [3]uint32
	start  [3]uint64
	mode   [3]uint8
	access [3]uint8
	wlo    [3]bool
	wval   [3]uint16
	latch  [3]uint16
	nlatch [3]int
	next0  uint64
}

func (p *pit) init(m *Machine) {
	for i := range p.div {
		p.div[i] = 65536
		p.access[i] = 3
		p.mode[i] = 3
	}
	p.next0 = 65536
}

func (p *pit) count(ch int, t uint64) uint16 {
	return uint16(uint64(p.div[ch]) - (t-p.start[ch])%uint64(p.div[ch]))
}

func (p *pit) write(m *Machine, port uint16, v uint8) {
	t := m.clk.t
	if port == 0x43 {
		ch := int(v >> 6)
		if ch == 3 {
			return // read-back
		}
		if v&0x30 == 0 {
			p.latch[ch] = p.count(ch, t)
			p.nlatch[ch] = 2
			return
		}
		p.access[ch] = v >> 4 & 3
		p.mode[ch] = v >> 1 & 7
		p.wlo[ch] = false
		return
	}
	ch := int(port - 0x40)
	switch p.access[ch] {
	case 1:
		p.load(m, ch, uint16(v))
	case 2:
		p.load(m, ch, uint16(v)<<8)
	case 3:
		if !p.wlo[ch] {
			p.wval[ch] = uint16(v)
			p.wlo[ch] = true
			return
		}
		p.wlo[ch] = false
		p.load(m, ch, p.wval[ch]|uint16(v)<<8)
	}
}

func (p *pit) load(m *Machine, ch int, c uint16) {
	d := uint32(c)
	if d == 0 {
		d = 65536
	}
	p.div[ch] = d
	p.start[ch] = m.clk.t
	if ch == 0 {
		p.next0 = m.clk.t + uint64(d)
		if d != 65536 {
			m.gameTimer(m.clk.t)
		}
		m.logf("pit: channel 0 divisor %#x mode %d", d, p.mode[0])
	}
}

func (p *pit) read(m *Machine, port uint16) uint8 {
	ch := int(port - 0x40)
	if p.nlatch[ch] > 0 {
		p.nlatch[ch]--
		if p.nlatch[ch] == 1 {
			return uint8(p.latch[ch])
		}
		return uint8(p.latch[ch] >> 8)
	}
	return uint8(p.count(ch, m.clk.t))
}

// nextEvent is the time of the next timer, sound or input event.
func (m *Machine) nextEvent() uint64 {
	t := m.pit.next0
	if s, ok := m.sb.next(); ok && s < t {
		t = s
	}
	if s, ok := m.kbd.next(); ok && s < t {
		t = s
	}
	return t
}

// advance moves time to t, raising the IRQs of the events due.
func (m *Machine) advance(t uint64) {
	if t < m.clk.t {
		t = m.clk.t
	}
	if m.clk.limit != 0 && t > m.clk.limit {
		t = m.clk.limit
	}
	m.clk.t = t
	for m.pit.next0 <= t {
		m.pic.raise(0)
		m.pit.next0 += uint64(m.pit.div[0])
	}
	m.sb.run(m)
	m.kbd.run(m)
}

// idle is called where the program waits for an interrupt: it delivers a
// pending IRQ or advances to the next event, and stops emulation so the
// run loop delivers it.
func (m *Machine) idle() {
	if !m.pic.pending() {
		m.advance(m.nextEvent())
	}
	m.e.stop()
	if m.clk.limit != 0 && m.clk.t >= m.clk.limit {
		return
	}
	if m.e.reg(rEFLAGS)&0x200 == 0 {
		m.fail(fmt.Errorf("wait with interrupts disabled at eip %#x", m.e.reg(rEIP)))
	}
}

// vga timing: mode 13h is 400 lines double-scanned, 449 total, vertical
// retrace on lines 412 and 413.
type vga struct {
	dac           [768]byte
	wIdx, rIdx    int
	crtcIdx       uint8
	crtc          [32]uint8
	seqIdx, gcIdx uint8
	seq, gc       [16]uint8
	misc          uint8
	mode          uint8

	lastEIP uint32 // last 3DAh poll, for wait detection
	lastVal uint8
}

func vgaDots(t uint64) uint64 { return t * vgaDotHz / pitHz }

func vgaStatus(t uint64) uint8 {
	pos := vgaDots(t) % vgaFrame
	line, x := pos/vgaLineD, pos%vgaLineD
	var s uint8
	if line >= 400 || x >= 640 {
		s |= 1
	}
	if line >= 412 && line < 414 {
		s |= 8
	}
	return s
}

// nextRetraceEdge is the first time after t where bit 3 of 3DAh changes.
func nextRetraceEdge(t uint64) uint64 {
	d := vgaDots(t)
	frame := d / vgaFrame * vgaFrame
	line := d % vgaFrame / vgaLineD
	var target uint64
	switch {
	case line < 412:
		target = frame + 412*vgaLineD
	case line < 414:
		target = frame + 414*vgaLineD
	default:
		target = frame + vgaFrame + 412*vgaLineD
	}
	// first t' with vgaDots(t') >= target
	nt := (target*pitHz + vgaDotHz - 1) / vgaDotHz
	for vgaDots(nt) < target {
		nt++
	}
	if nt <= t {
		nt = t + 1
	}
	return nt
}

// status3DA reads the input status register. A second poll from the same
// instruction with the same result is a wait: time moves on to the next
// retrace edge or event.
func (m *Machine) status3DA() uint8 {
	eip := m.e.reg(rEIP)
	v := vgaStatus(m.clk.t)
	if eip == m.vga.lastEIP && v&8 == m.vga.lastVal&8 && !m.pic.pending() {
		to := nextRetraceEdge(m.clk.t)
		if ev := m.nextEvent(); ev < to {
			to = ev
		}
		m.advance(to)
		if m.pic.pending() {
			m.e.stop()
		}
		v = vgaStatus(m.clk.t)
	}
	m.vga.lastEIP, m.vga.lastVal = eip, v
	return v
}
