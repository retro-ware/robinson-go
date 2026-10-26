//go:build orig

package orig

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"sync/atomic"
)

// Config selects the files of a run.
type Config struct {
	Image string // START.EXE's 32-bit image (orig/start32.bin)
	CD    string // the CD files: drive D:, the current drive
	Save  string // writable drive C: (START.STP, saves); new files go here
	Log   io.Writer
	// Calls logs every DOS and extender call.
	Calls bool
}

type farPtr struct {
	sel uint16
	off uint32
}

// Machine is the emulated PC running START.EXE.
type Machine struct {
	e    *engine
	cfg  Config
	low  []byte // linear 0..lowSize
	prog []byte // linear progLin..progLin+progSize (DS offset 0..)
	desc [gdtSlots]descriptor

	pmVec  [256]farPtr
	exited bool
	code   int
	err    error // fatal error found in a hook

	exc    int // exception to deliver after the stop, -1 none
	excEIP uint32

	clk   clock
	pic   pic
	pit   pit
	vga   vga
	kbd   kbd
	dos   dos
	sb    sb
	opl   opl
	dma   dma
	mouse mouse
	alloc struct {
		next     uint32 // DS offset of the next 350C/350D block
		stackLow uint32
		dosNext  uint32 // linear, next AH=48h block
	}
	unknownPorts map[uint16]bool
	OnDraw       func() // called at each draw (0x6340), before the frame wait

	progress atomic.Uint64 // idles, for the watchdog
	sample   atomic.Bool   // watchdog asks for an EIP sample
	samples  map[uint32]int
}

// Initial DS-offset layout: image, then a 64 KB start stack, then the
// extender's blocks.
const (
	imageEnd   = 0x1cd50
	startStack = 0x30000
	heapStart  = 0x100000
)

// New loads START.EXE and sets up the machine as X-32 hands it over.
func New(cfg Config) (*Machine, error) {
	img, err := os.ReadFile(cfg.Image)
	if err != nil {
		return nil, err
	}
	if cfg.Log == nil {
		cfg.Log = io.Discard
	}
	e, err := newEngine()
	if err != nil {
		return nil, err
	}
	m := &Machine{e: e, cfg: cfg, exc: -1, unknownPorts: map[uint16]bool{}}
	if m.low, err = e.mapMem(0, lowSize); err != nil {
		e.close()
		return nil, err
	}
	if m.prog, err = e.mapMem(progLin, progSize); err != nil {
		e.close()
		return nil, err
	}
	copy(m.prog, img)

	m.setDesc(selCode, progLin, 0xffffffff, true)
	m.setDesc(selData, progLin, 0xffffffff, false)
	m.setDesc(selPSP, pspLin, 0xff, false)
	m.setDesc(selEnv, envLin, 0xfff, false)
	m.setDesc(selZero, 0, 0xffffffff, false)
	m.setDesc(selStub, 0, 0xffffffff, true)
	if err := e.setGDTR(gdtLin, gdtSlots*8-1); err != nil {
		e.close()
		return nil, err
	}
	m.low[stubLin] = 0xcf // iretd
	for i := range m.pmVec {
		m.pmVec[i] = farPtr{selStub, stubLin}
	}

	// PSP: int 20h, empty command tail, environment segment.
	psp := m.low[pspLin:]
	psp[0], psp[1] = 0xcd, 0x20
	binary.LittleEndian.PutUint16(psp[0x2c:], envLin>>4)
	psp[0x80], psp[0x81] = 0, 0x0d
	// Environment as DOSBox sets it, then the program path.
	env := "PATH=Z:\\\x00COMSPEC=Z:\\COMMAND.COM\x00BLASTER=A220 I7 D1 H5 T6\x00\x00\x01\x00D:\\START.EXE\x00"
	copy(m.low[envLin:], env)

	m.alloc.next = heapStart
	m.alloc.dosNext = dosLin
	m.dos.init(m)
	m.clk.init()
	m.pit.init(m)

	for _, r := range []struct {
		id  regID
		sel uint16
	}{{rCS, selCode}, {rDS, selData}, {rES, selData}, {rSS, selData}, {rFS, selPSP}, {rGS, selEnv}} {
		if err := e.setReg(r.id, uint32(r.sel)); err != nil {
			e.close()
			return nil, fmt.Errorf("load %#x: %w", r.sel, err)
		}
	}
	e.setReg(rESP, startStack)
	e.setReg(rEFLAGS, 0x202)
	e.setReg(rEIP, 0xd)

	e.onIntr = m.intr
	e.onIn = m.in
	e.onOut = m.out
	e.onInvalid = func(addr uint64, size int, write bool) {
		m.fail(fmt.Errorf("invalid %s of %d bytes at linear %#x, eip %#x", map[bool]string{false: "read", true: "write"}[write], size, addr, e.reg(rEIP)))
	}
	if err := m.installHooks(); err != nil {
		e.close()
		return nil, err
	}
	return m, nil
}

func (m *Machine) Close() { m.e.close() }

func (m *Machine) logf(format string, args ...any) {
	fmt.Fprintf(m.cfg.Log, "[%9.3f] ", m.clk.ms())
	fmt.Fprintf(m.cfg.Log, format, args...)
	if !strings.HasSuffix(format, "\n") {
		fmt.Fprintln(m.cfg.Log)
	}
}

// fail stops emulation with err (from inside a hook).
func (m *Machine) fail(err error) {
	if m.err == nil {
		m.err = err
	}
	m.e.stop()
}

// Run runs until the program exits, a hook fails or limit (virtual PIT
// clocks after the game timer started, 0 = none) passes.
func (m *Machine) Run(limit uint64) error {
	m.clk.runFor = limit
	if m.clk.started && limit != 0 {
		m.clk.limit = m.kbd.base + limit
	}
	done := make(chan struct{})
	defer close(done)
	go m.watchdog(done)
	eip := m.e.reg(rEIP)
	lastProgress := m.progress.Load()
	for !m.exited && m.err == nil {
		if err := m.e.start(eip); err != nil {
			return fmt.Errorf("eip %#x: %w; %s", m.e.reg(rEIP), err, m.Regs())
		}
		if p := m.progress.Load(); p != lastProgress {
			lastProgress = p
			m.samples = nil
		}
		if m.sample.Load() {
			m.takeSample()
		}
		if m.err != nil {
			break
		}
		eip = m.e.reg(rEIP)
		if m.clk.limit != 0 && m.clk.t >= m.clk.limit {
			return nil
		}
		if m.exc >= 0 {
			n := m.exc
			m.exc = -1
			if m.pmVec[n].sel == selStub {
				return fmt.Errorf("unhandled exception %d at eip %#x", n, eip)
			}
			eip = m.deliver(n, eip)
			continue
		}
		eip = m.deliverIRQ(eip)
	}
	if m.err != nil {
		return m.err
	}
	return nil
}

// deliverIRQ delivers the highest priority pending IRQ if interrupts are
// enabled and returns the EIP to continue at.
func (m *Machine) deliverIRQ(eip uint32) uint32 {
	for m.e.reg(rEFLAGS)&0x200 != 0 {
		irq, ok := m.pic.take()
		if !ok {
			break
		}
		n := 8 + irq
		if m.pmVec[n].sel == selStub {
			m.bios(irq)
			m.pic.eoi(irq)
			continue
		}
		return m.deliver(n, eip)
	}
	return eip
}

// bios does what the BIOS handler of an IRQ the program doesn't handle
// would do.
func (m *Machine) bios(irq int) {
	switch irq {
	case 0:
		binary.LittleEndian.PutUint32(m.low[0x46c:], binary.LittleEndian.Uint32(m.low[0x46c:])+1)
	case 1:
		sc := m.kbd.data
		if sc&0x80 == 0 {
			a := biosASCII[sc]
			if sc == 0x53 { // Del: extended
				a = 0
			}
			m.kbd.bios = append(m.kbd.bios, uint16(sc)<<8|uint16(a))
		}
	}
}

// deliver enters the PM handler of interrupt n as the CPU does through an
// interrupt gate: push EFLAGS, CS, EIP, clear IF and TF.
func (m *Machine) deliver(n int, eip uint32) uint32 {
	fl := m.e.reg(rEFLAGS)
	cs := m.e.reg(rCS)
	esp := m.e.reg(rESP) - 12
	ss := m.selBase(uint16(m.e.reg(rSS)))
	m.w32(ss+esp, eip)
	m.w32(ss+esp+4, cs&0xffff)
	m.w32(ss+esp+8, fl)
	m.e.setReg(rESP, esp)
	m.e.setReg(rEFLAGS, fl&^0x300)
	v := m.pmVec[n]
	if uint32(v.sel) != cs&0xffff {
		if err := m.e.setReg(rCS, uint32(v.sel)); err != nil {
			m.fail(fmt.Errorf("int %#x: load cs %#x: %w", n, v.sel, err))
		}
	}
	return v.off
}

// Memory access by linear address.

func (m *Machine) mem(lin uint32, n int) []byte {
	switch {
	case lin < lowSize && int(lin)+n <= lowSize:
		return m.low[lin : int(lin)+n]
	case lin >= progLin && int(lin-progLin)+n <= progSize:
		return m.prog[lin-progLin : int(lin-progLin)+n]
	}
	m.fail(fmt.Errorf("host access to unmapped linear %#x+%d, eip %#x", lin, n, m.e.reg(rEIP)))
	return make([]byte, n)
}

func (m *Machine) r16(lin uint32) uint16    { return binary.LittleEndian.Uint16(m.mem(lin, 2)) }
func (m *Machine) r32(lin uint32) uint32    { return binary.LittleEndian.Uint32(m.mem(lin, 4)) }
func (m *Machine) w16(lin uint32, v uint16) { binary.LittleEndian.PutUint16(m.mem(lin, 2), v) }
func (m *Machine) w32(lin uint32, v uint32) { binary.LittleEndian.PutUint32(m.mem(lin, 4), v) }

// ds converts a DS offset to a linear address.
func ds(off uint32) uint32 { return progLin + off }

// D8, D16 and D32 read the program's data by DS offset.
func (m *Machine) D8(off uint32) uint8   { return m.mem(ds(off), 1)[0] }
func (m *Machine) D16(off uint32) uint16 { return m.r16(ds(off)) }
func (m *Machine) D32(off uint32) uint32 { return m.r32(ds(off)) }

// Ms is the virtual time in ms.
func (m *Machine) Ms() float64 { return m.clk.ms() }

// Prog is the program's memory by DS offset.
func (m *Machine) Prog() []byte { return m.prog }

// VRAM is the VGA mode 13h framebuffer.
func (m *Machine) VRAM() []byte { return m.low[vgaLin : vgaLin+64000] }

// Screen is the VGA screen without the mouse cursor: the background the
// timer handler saved (at 0x16070: offset, width, height, pixels) is put
// back while the cursor is on.
func (m *Machine) Screen() []byte {
	scr := append([]byte(nil), m.VRAM()...)
	if int8(m.D8(0x16077)) < 0 {
		return scr
	}
	sv := m.D32(0x16070)
	off := m.D32(sv)
	w, h := uint32(m.D16(sv+4)), uint32(m.D16(sv+6))
	src := sv + 8
	for y := uint32(0); y < h; y++ {
		for x := uint32(0); x < w; x++ {
			if p := off + y*320 + x; p < 64000 {
				scr[p] = m.D8(src)
			}
			src++
		}
	}
	return scr
}

// DAC is the VGA palette (256 × r, g, b, 6 bit).
func (m *Machine) DAC() []byte { return m.vga.dac[:] }

func (m *Machine) cstr(lin uint32) string {
	var b []byte
	for i := uint32(0); i < 256; i++ {
		c := m.mem(lin+i, 1)[0]
		if c == 0 {
			break
		}
		b = append(b, c)
	}
	return string(b)
}

// Regs formats the registers for diagnostics.
func (m *Machine) Regs() string {
	e := m.e
	return fmt.Sprintf("eip=%#x eax=%#x ebx=%#x ecx=%#x edx=%#x esi=%#x edi=%#x ebp=%#x esp=%#x fl=%#x cs=%#x ds=%#x es=%#x fs=%#x gs=%#x ss=%#x",
		e.reg(rEIP), e.reg(rEAX), e.reg(rEBX), e.reg(rECX), e.reg(rEDX), e.reg(rESI), e.reg(rEDI), e.reg(rEBP), e.reg(rESP), e.reg(rEFLAGS),
		e.reg(rCS), e.reg(rDS), e.reg(rES), e.reg(rFS), e.reg(rGS), e.reg(rSS))
}

// UnknownPorts lists the ports accessed that the machine doesn't model.
func (m *Machine) UnknownPorts() []uint16 {
	var p []uint16
	for k := range m.unknownPorts {
		p = append(p, k)
	}
	sort.Slice(p, func(i, j int) bool { return p[i] < p[j] })
	return p
}
