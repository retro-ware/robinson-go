//go:build orig

package orig

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// Wait points of START.EXE: loops that spin until an interrupt handler
// changes something. Each hook runs before the instruction at its address
// and idles only when the loop would go round again.

// sbProbeTimeout is how long the Sound Blaster IRQ probe at 0x11121 spins
// (0x8fffd rounds of 7 instructions at DOSBox's 20000 cycles per ms).
const sbProbeTimeout = 0x8fffd * 7 * pitHz / 20000 / 1000

func (m *Machine) installHooks() error {
	var sbDeadline uint64
	hooks := map[uint32]func(){
		// timer setup: wait for the first tick
		0x1e8c: func() {
			if m.D8(0x1605f) == 0 {
				m.idle()
			}
		},
		// CPU speed count until the second tick: a fast CPU saturates it
		0x1eb2: func() {
			if m.D8(0x1605f) != 2 {
				m.setAX(0x7fff)
				m.idle()
			}
		},
		// draw: wait until ctiming ticks have passed
		0x6352: func() {
			if uint8(m.e.reg(rEAX)) > m.D8(0x1605f) {
				m.idle()
			}
		},
		// Sound Blaster IRQ probe: two IRQs within the timeout
		0x11121: func() { sbDeadline = m.clk.t + sbProbeTimeout },
		0x11130: func() {
			if m.D16(0x1b5f8) >= 2 {
				return
			}
			if m.clk.t >= sbDeadline {
				m.e.setReg(rECX, 1)
				return
			}
			if !m.pic.pending() {
				m.advance(min(m.nextEvent(), sbDeadline))
			}
			m.e.stop()
		},
		// CD speed test: 150000 bytes read in no time would divide by
		// zero ticks; DOSBox's emulated reads take about one tick.
		0x32c7: func() {
			if m.D32(0x16058) == m.D32(0x18e78) {
				m.idle()
			}
		},
		// film: wait for the frame's tick
		0x13e5d: func() {
			if m.e.reg(rEAX) > m.D32(0x16058) {
				m.idle()
			}
		},
		// wait for the next tick
		0x1166c: func() {
			if m.e.reg(rEAX) == m.D32(0x16058) {
				m.idle()
			}
		},
		// getkey: wait for a key press, then for its release
		0x2492: func() {
			if m.D8(0x18b68) == 0 {
				m.idle()
			}
		},
		0x249f: func() {
			if m.D8(0x18b68) != 0 {
				m.idle()
			}
		},
		0x6340: func() {
			if m.OnDraw != nil {
				m.OnDraw()
			}
		},
	}
	for addr, f := range hooks {
		if err := m.e.hookCode(progLin+addr, m.counted(f)); err != nil {
			return err
		}
	}
	return nil
}

// Hook runs f before the instruction at addr (a DS offset), ahead of the
// machine's own hook there. f must not change the machine's state.
func (m *Machine) Hook(addr uint32, f func()) error {
	if prev, ok := m.e.onCode[progLin+addr]; ok {
		return m.e.hookCode(progLin+addr, func() { f(); prev() })
	}
	return m.e.hookCode(progLin+addr, f)
}

// Watch calls f for every write to the DS offsets [from, to) (one range
// per machine), with the writing instruction's EIP.
func (m *Machine) Watch(from, to uint32, f func(eip, addr uint32, size int, val uint32)) error {
	return m.e.hookWrite(ds(from), ds(to), func(addr uint32, size int, val uint32) {
		f(m.e.reg(rEIP), addr-progLin, size, val)
	})
}

func (m *Machine) counted(f func()) func() {
	return func() {
		m.progress.Add(1)
		f()
	}
}

// watchdog samples the EIP when the program runs for a second of real time
// without reaching a wait point, and fails after ten such samples.
func (m *Machine) watchdog(done <-chan struct{}) {
	t := time.NewTicker(time.Second)
	defer t.Stop()
	last := m.progress.Load()
	for {
		select {
		case <-done:
			return
		case <-t.C:
		}
		p := m.progress.Load()
		if p == last {
			m.sample.Store(true)
			m.e.stop()
		}
		last = p
	}
}

// takeSample records the EIP for the watchdog; it fails with the EIP
// histogram after ten samples.
func (m *Machine) takeSample() {
	m.sample.Store(false)
	if m.samples == nil {
		m.samples = map[uint32]int{}
	}
	m.samples[m.e.reg(rEIP)]++
	n := 0
	for _, c := range m.samples {
		n += c
	}
	if n < 10 {
		return
	}
	var eips []uint32
	for k := range m.samples {
		eips = append(eips, k)
	}
	sort.Slice(eips, func(i, j int) bool { return eips[i] < eips[j] })
	var b strings.Builder
	for _, k := range eips {
		fmt.Fprintf(&b, " %#x×%d", k, m.samples[k])
	}
	m.err = fmt.Errorf("no wait point reached for 10 s at %.1f ms; eips:%s; %s", m.clk.ms(), b.String(), m.Regs())
}
