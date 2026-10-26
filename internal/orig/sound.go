//go:build orig

package orig

import "fmt"

// dma is the 8237 pair with the page registers.
type dma struct {
	ff    bool
	addr  [8]uint16
	count [8]uint16
	page  [8]uint8
	mode  [8]uint8
	mask  uint8
}

var dmaPagePort = map[uint16]int{0x87: 0, 0x83: 1, 0x81: 2, 0x82: 3, 0x8f: 4, 0x8b: 5, 0x89: 6, 0x8a: 7}

func (d *dma) out(m *Machine, port uint16, v uint8) {
	if ch, ok := dmaPagePort[port]; ok {
		d.page[ch] = v
		return
	}
	if port >= 0x80 {
		if port < 0xc0 {
			return
		}
		m.unknownPort(port, fmt.Sprintf("dma out %#x", v)) // 16-bit controller
		return
	}
	switch {
	case port < 8:
		ch := int(port >> 1)
		p := &d.addr[ch]
		if port&1 != 0 {
			p = &d.count[ch]
		}
		if !d.ff {
			*p = *p&0xff00 | uint16(v)
		} else {
			*p = *p&0x00ff | uint16(v)<<8
		}
		d.ff = !d.ff
	case port == 0x0a:
		if v&4 != 0 {
			d.mask |= 1 << (v & 3)
		} else {
			d.mask &^= 1 << (v & 3)
		}
	case port == 0x0b:
		d.mode[v&3] = v
	case port == 0x0c:
		d.ff = false
	case port == 0x0d:
		d.mask = 0x0f
		d.ff = false
	case port == 0x0f:
		d.mask = v & 0x0f
	default:
		m.unknownPort(port, fmt.Sprintf("dma out %#x", v))
	}
}

func (d *dma) in(m *Machine, port uint16) uint8 {
	m.unknownPort(port, "dma in")
	return 0xff
}

// sb is a Sound Blaster DSP at 220h, IRQ 7, DMA 1 (DOSBox's defaults),
// with 8-bit single-cycle DMA output, as the game drives it.
type sb struct {
	resetArmed bool
	outq       []uint8
	cmd        uint8
	args       []uint8
	need       int
	tc         uint8
	speaker    bool

	busy   bool
	end    uint64
	n      int
	Blocks []SBBlock
}

// SBBlock is one played DMA block of unsigned 8-bit samples.
type SBBlock struct {
	T    uint64 // virtual time at its end
	Rate int    // samples per second (1000000/(256-tc))
	Data []byte
}

const sbIRQ = 7

func (s *sb) out(m *Machine, reg uint16, v uint8) {
	switch reg {
	case 6:
		if v&1 != 0 {
			s.resetArmed = true
		} else if s.resetArmed {
			s.resetArmed = false
			s.outq = []uint8{0xaa}
			s.need = 0
			s.busy = false
			s.speaker = false
		}
	case 0xc:
		if s.need > 0 {
			s.args = append(s.args, v)
			s.need--
			if s.need == 0 {
				s.exec(m)
			}
			return
		}
		s.cmd = v
		s.args = s.args[:0]
		s.need = map[uint8]int{0x10: 1, 0x14: 2, 0x40: 1, 0x48: 2, 0x91: 0, 0xe0: 1}[v]
		if s.need == 0 {
			s.exec(m)
		}
	case 4, 5: // mixer
	default:
		m.unknownPort(0x220+reg, fmt.Sprintf("sb out %#x", v))
	}
}

func (s *sb) exec(m *Machine) {
	switch s.cmd {
	case 0x10: // direct DAC
	case 0x14:
		n := int(s.args[0]) | int(s.args[1])<<8 + 1
		s.start(m, n)
	case 0x40:
		s.tc = s.args[0]
	case 0xd1:
		s.speaker = true
	case 0xd3:
		s.speaker = false
	case 0xd0:
		if s.busy {
			m.logf("sb: pause during a block")
		}
	case 0xe0:
		s.outq = append(s.outq, ^s.args[0])
	case 0xe1:
		s.outq = append(s.outq, 4, 5)
	case 0xf2:
		m.pic.raise(sbIRQ)
	default:
		m.logf("sb: dsp command %#x %x", s.cmd, s.args)
	}
}

func (s *sb) rate() int { return 1000000 / (256 - int(s.tc)) }

func (s *sb) start(m *Machine, n int) {
	if s.busy {
		m.logf("sb: block started while busy")
	}
	s.busy = true
	s.n = n
	s.end = m.clk.t + uint64(n)*pitHz*uint64(256-int(s.tc))/1000000
}

func (s *sb) next() (uint64, bool) { return s.end, s.busy }

// run ends the block when due: the samples are taken from the DMA
// buffer now, as the program refills only blocks that have played.
func (s *sb) run(m *Machine) {
	if !s.busy || m.clk.t < s.end {
		return
	}
	s.busy = false
	d := &m.dma
	lin := uint32(d.page[1])<<16 | uint32(d.addr[1])
	n := min(s.n, int(d.count[1])+1)
	data := append([]byte(nil), m.mem(lin, n)...)
	s.Blocks = append(s.Blocks, SBBlock{T: s.end, Rate: s.rate(), Data: data})
	d.addr[1] += uint16(n)
	d.count[1] = 0xffff
	m.pic.raise(sbIRQ)
}

func (s *sb) in(m *Machine, reg uint16) uint8 {
	switch reg {
	case 0xa:
		if len(s.outq) == 0 {
			return 0xff
		}
		v := s.outq[0]
		s.outq = s.outq[1:]
		return v
	case 0xc:
		return 0 // ready for commands
	case 0xe:
		if len(s.outq) > 0 {
			return 0x80
		}
		return 0
	case 5:
		return 0
	}
	m.unknownPort(0x220+reg, "sb in")
	return 0xff
}
