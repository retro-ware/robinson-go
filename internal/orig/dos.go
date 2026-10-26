//go:build orig

package orig

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// dos is the DOS state: files, drives, DTA.
type dos struct {
	files map[uint16]*os.File
	drive int        // 0 = A:
	cwd   [26]string // without drive and leading backslash, upper case
	dta   uint32     // linear
	find  struct {
		names []string
		dirs  []string
		attr  uint16
	}
	dataStart, dataEnd uint32
}

func (d *dos) init(m *Machine) {
	d.files = map[uint16]*os.File{}
	d.drive = 3 // D:
	d.dta = pspLin + 0x80
}

// intr handles software interrupts and CPU exceptions.
func (m *Machine) intr(n uint32) {
	switch n {
	case 0x21:
		m.int21()
	case 0x10:
		m.int10()
	case 0x11:
		m.e.setReg(rEAX, m.e.reg(rEAX)&^0xffff|0x0026) // floppy-less, 80x25 colour, FPU
	case 0x33:
		m.int33()
	case 0, 1, 3, 4, 5, 6, 7, 0xd, 0xe:
		m.exc = int(n)
		m.excEIP = m.e.reg(rEIP)
		m.e.stop()
	default:
		m.fail(fmt.Errorf("int %#x at eip %#x: %s", n, m.e.reg(rEIP), m.Regs()))
	}
}

func (m *Machine) setCF(c bool) {
	fl := m.e.reg(rEFLAGS) &^ 1
	if c {
		fl |= 1
	}
	m.e.setReg(rEFLAGS, fl)
}

func (m *Machine) setAX(v uint16) { m.e.setReg(rEAX, m.e.reg(rEAX)&^0xffff|uint32(v)) }

// dosErr returns DOS error code e with CF set.
func (m *Machine) dosErr(e uint16) {
	m.setAX(e)
	m.setCF(true)
}

// segLin converts a selector:offset pair to linear.
func (m *Machine) segLin(sel uint32, off uint32) uint32 { return m.selBase(uint16(sel)) + off }

func (m *Machine) dsEDX() uint32 { return m.segLin(m.e.reg(rDS), m.e.reg(rEDX)) }

func (m *Machine) int21() {
	e := m.e
	ax := e.reg(rEAX) & 0xffff
	ah := ax >> 8
	if m.cfg.Calls {
		m.logf("int21 %04x ebx=%#x ecx=%#x edx=%#x eip=%#x", ax, e.reg(rEBX), e.reg(rECX), e.reg(rEDX), e.reg(rEIP))
	}
	if ah == 0x25 && ax >= 0x2500 && ax < 0x2540 || ah == 0x35 {
		m.extender(ax)
		return
	}
	m.setCF(false)
	switch ah {
	case 0x09:
		var b []byte
		for lin := m.dsEDX(); ; lin++ {
			c := m.mem(lin, 1)[0]
			if c == '$' || len(b) > 1000 {
				break
			}
			b = append(b, c)
		}
		m.logf("print %q", b)
	case 0x06: // direct console I/O; DL=FFh reads without waiting
		if e.reg(rEDX)&0xff != 0xff {
			return
		}
		if len(m.kbd.bios) == 0 && m.kbd.ext == 0 {
			m.setAX(uint16(ax & 0xff00))
			e.setReg(rEFLAGS, e.reg(rEFLAGS)|0x40)
			m.idle() // the setup prompt polls here
			return
		}
		var c uint16
		if m.kbd.ext != 0 { // second byte of an extended key: the scancode
			c, m.kbd.ext = m.kbd.ext, 0
		} else {
			k := m.kbd.bios[0]
			m.kbd.bios = m.kbd.bios[1:]
			if c = k & 0xff; c == 0 {
				m.kbd.ext = k >> 8
			}
		}
		m.setAX(uint16(ax&0xff00 | uint32(c)))
		e.setReg(rEFLAGS, e.reg(rEFLAGS)&^0x40)
	case 0x0e:
		m.dos.drive = int(e.reg(rEDX) & 0xff)
		m.setAX(uint16(ax&0xff00 | 26))
	case 0x19:
		m.setAX(uint16(ax&0xff00) | uint16(m.dos.drive))
	case 0x1a:
		m.dos.dta = m.dsEDX()
	case 0x2a: // date: Monday 1994-11-07
		e.setReg(rECX, e.reg(rECX)&^0xffff|1994)
		e.setReg(rEDX, e.reg(rEDX)&^0xffff|11<<8|7)
		m.setAX(uint16(ax&0xff00 | 1))
	case 0x2c: // time from the virtual clock, from midnight
		cs := m.clk.t * 100 / pitHz
		h, mi, s, c := cs/360000%24, cs/6000%60, cs/100%60, cs%100
		e.setReg(rECX, e.reg(rECX)&^0xffff|uint32(h<<8|mi))
		e.setReg(rEDX, e.reg(rEDX)&^0xffff|uint32(s<<8|c))
	case 0x30:
		m.setAX(0x0005) // DOS 5.0
		e.setReg(rEBX, e.reg(rEBX)&^0xffff)
		e.setReg(rECX, e.reg(rECX)&^0xffff)
	case 0x36: // 2 GB free: 64 sectors of 512 bytes per cluster
		m.setAX(64)
		e.setReg(rEBX, e.reg(rEBX)&^0xffff|0xffff)
		e.setReg(rECX, e.reg(rECX)&^0xffff|512)
		e.setReg(rEDX, e.reg(rEDX)&^0xffff|0xffff)
	case 0x3b:
		m.chdir(m.cstr(m.dsEDX()))
	case 0x3c:
		m.create(m.cstr(m.dsEDX()))
	case 0x3d:
		m.open(m.cstr(m.dsEDX()), int(ax&3))
	case 0x3e:
		h := uint16(e.reg(rEBX))
		if f, ok := m.dos.files[h]; ok {
			f.Close()
			delete(m.dos.files, h)
		} else if h > 4 {
			m.dosErr(6)
		}
	case 0x3f:
		m.read(uint16(e.reg(rEBX)), m.dsEDX(), e.reg(rECX))
	case 0x40:
		m.write(uint16(e.reg(rEBX)), m.dsEDX(), e.reg(rECX))
	case 0x41:
		name := m.cstr(m.dsEDX())
		p, _, ok := m.writePath(name)
		if !ok || os.Remove(p) != nil {
			m.dosErr(2)
		}
		m.logf("delete %s", name)
	case 0x42:
		m.seek(uint16(e.reg(rEBX)), int(ax&0xff), int64(int32(e.reg(rECX)<<16|e.reg(rEDX)&0xffff)))
	case 0x43:
		name := m.cstr(m.dsEDX())
		if p, ok := m.readPath(name); !ok {
			m.dosErr(2)
		} else if fi, err := os.Stat(p); err != nil {
			m.dosErr(2)
		} else {
			attr := uint32(0x20)
			if fi.IsDir() {
				attr = 0x10
			}
			e.setReg(rECX, e.reg(rECX)&^0xffff|attr)
		}
	case 0x44:
		m.ioctl(ax)
	case 0x47:
		d := int(e.reg(rEDX) & 0xff)
		if d == 0 {
			d = m.dos.drive + 1
		}
		b := m.mem(m.segLin(e.reg(rDS), e.reg(rESI)), 64)
		n := copy(b, m.dos.cwd[d-1])
		b[n] = 0
	case 0x48:
		m.dosAlloc(e.reg(rEBX) & 0xffff)
	case 0x49:
	case 0x4c:
		m.exited = true
		m.code = int(ax & 0xff)
		m.logf("exit %d", m.code)
		e.stop()
	case 0x4e:
		m.findFirst(m.cstr(m.dsEDX()), uint16(e.reg(rECX)))
	case 0x4f:
		m.findNext()
	case 0x67:
	default:
		m.fail(fmt.Errorf("int 21h ax=%#04x at eip %#x: %s", ax, e.reg(rEIP), m.Regs()))
	}
}

// extender handles the X-32 (35xxh) and Phar Lap compatible (25xxh) calls.
func (m *Machine) extender(ax uint32) {
	e := m.e
	m.setCF(false)
	switch ax {
	case 0x2502: // get PM vector: ES:EBX
		v := m.pmVec[e.reg(rECX)&0xff]
		e.setReg(rES, uint32(v.sel))
		e.setReg(rEBX, v.off)
	case 0x2503: // get RM vector: EBX = seg:off
		e.setReg(rEBX, 0xf000ff53)
	case 0x2504, 0x2506, 0x2507: // set PM vector DS:EDX (2507 also sets RM vector EBX)
		n := e.reg(rECX) & 0xff
		m.pmVec[n] = farPtr{uint16(e.reg(rDS)), e.reg(rEDX)}
		m.logf("vector %#x = %#x:%#x (%04x)", n, e.reg(rDS)&0xffff, e.reg(rEDX), ax)
	case 0x2508: // selector base
		e.setReg(rECX, m.selBase(uint16(e.reg(rEBX))))
	case 0x2509: // real-mode code segment of the extender's helpers
		e.setReg(rEBX, rmHelperSeg)
	case 0x250d: // real-mode link info
		e.setReg(rEAX, rmHelperSeg<<16|0x60)
		e.setReg(rECX, 0x1000)
		e.setReg(rEDX, 0)
	case 0x250e: // call real-mode procedure EBX
		m.callRM(e.reg(rEBX))
	case 0x252b: // lock pages
	case 0x3501:
		sel, ok := m.allocSel()
		if !ok {
			m.setCF(true)
			return
		}
		e.setReg(rEBX, e.reg(rEBX)&^0xffff|uint32(sel))
	case 0x3502:
		sel := uint16(e.reg(rEBX))
		if sel >= selDyn {
			m.desc[sel>>3].used = false
			m.writeDesc(sel)
		}
	case 0x3503, 0x3505:
		sel := uint16(e.reg(rEBX))
		if sel>>3 >= gdtSlots || !m.desc[sel>>3].used {
			m.setCF(true)
			return
		}
		d := m.desc[sel>>3]
		d.base = m.selBase(sel)
		if ax == 0x3503 {
			d.base = e.reg(rECX)
		} else if sel >= selDyn {
			d.limit = e.reg(rECX)
		}
		m.setDesc(sel, d.base, d.limit, d.code)
	case 0x3504:
		e.setReg(rECX, m.selBase(uint16(e.reg(rEBX))))
	case 0x350b: // free memory
		e.setReg(rEAX, progSize-m.alloc.next)
	case 0x350c: // allocate ECX bytes: EAX start, ECX size, EDX stack limit
		n := (e.reg(rECX) + 0xfff) &^ 0xfff
		if m.alloc.next+n > progSize {
			m.setCF(true)
			return
		}
		e.setReg(rEAX, m.alloc.next)
		e.setReg(rEDX, m.alloc.stackLow)
		m.logf("350c: %#x bytes at %#x", n, m.alloc.next)
		m.alloc.next += n
	case 0x350d: // new stack of ECX bytes: EBX top
		n := (e.reg(rECX) + 0xfff) &^ 0xfff
		m.alloc.stackLow = m.alloc.next
		m.alloc.next += n
		e.setReg(rEBX, m.alloc.next)
		m.logf("350d: stack %#x..%#x", m.alloc.stackLow, m.alloc.next)
	default:
		m.fail(fmt.Errorf("extender call ax=%#04x at eip %#x: %s", ax, e.reg(rEIP), m.Regs()))
	}
}

// The extender's real-mode helper segment. Offset 0x50 returns the DOS
// country code in AL. Offset 0x64 is the mouse procedure: it installs a mouse event handler (int 33h function 0Ch)
// that keeps buttons, x and y in three words and returns their address in
// AX (offset) and BX (segment).
const rmHelperSeg = 0x70

func (m *Machine) callRM(addr uint32) {
	switch addr {
	case rmHelperSeg<<16 | 0x50: // DOS country code (AH=38h): DOSBox's default, USA
		m.setAX(1)
	case rmHelperSeg<<16 | 0x64:
		m.setAX(mouseLin & 0xf)
		m.e.setReg(rEBX, m.e.reg(rEBX)&^0xffff|mouseLin>>4)
		m.mouse.rm = true
	default:
		m.fail(fmt.Errorf("real-mode call %04x:%04x at eip %#x", addr>>16, addr&0xffff, m.e.reg(rEIP)))
	}
}

func (m *Machine) dosAlloc(paras uint32) {
	n := paras << 4
	if m.alloc.dosNext+n > dosEnd {
		m.e.setReg(rEBX, (dosEnd-m.alloc.dosNext)>>4)
		m.dosErr(8)
		return
	}
	lin := m.alloc.dosNext
	m.alloc.dosNext += (n + 0xf) &^ 0xf
	m.setAX(uint16(lin >> 4))
	m.e.setReg(rEBX, lin-progLin)
	m.logf("dos alloc %#x paragraphs at %#x", paras, lin)
}

func (m *Machine) ioctl(ax uint32) {
	h := uint16(m.e.reg(rEBX))
	switch ax {
	case 0x4400:
		var dx uint32 = 0x80d3 // character device (console)
		if _, ok := m.dos.files[h]; ok {
			dx = uint32(m.dos.drive)
		} else if h > 4 {
			m.dosErr(6)
			return
		}
		m.e.setReg(rEDX, m.e.reg(rEDX)&^0xffff|dx)
	default:
		m.fail(fmt.Errorf("ioctl %#04x", ax))
	}
}

// File system: D: is the CD, C: the save directory; a write anywhere goes
// to the save directory (as DOSBox's overlay would not, but the game only
// writes to C:).

// dosPath makes name absolute: drive index and components.
func (m *Machine) dosPath(name string) (int, []string) {
	name = strings.ToUpper(strings.ReplaceAll(name, "/", "\\"))
	drive := m.dos.drive
	if len(name) >= 2 && name[1] == ':' {
		drive = int(name[0] - 'A')
		name = name[2:]
	}
	var parts []string
	if !strings.HasPrefix(name, "\\") && m.dos.cwd[drive] != "" {
		parts = strings.Split(m.dos.cwd[drive], "\\")
	}
	for _, p := range strings.Split(name, "\\") {
		switch p {
		case "", ".":
		case "..":
			if len(parts) > 0 {
				parts = parts[:len(parts)-1]
			}
		default:
			parts = append(parts, p)
		}
	}
	return drive, parts
}

func (m *Machine) driveRoot(drive int) string {
	switch drive {
	case 2:
		return m.cfg.Save
	case 3:
		return m.cfg.CD
	}
	return ""
}

// lookup finds the host path of parts under root, case-insensitively.
func lookup(root string, parts []string) (string, bool) {
	p := root
	for _, c := range parts {
		ents, err := os.ReadDir(p)
		if err != nil {
			return "", false
		}
		found := false
		for _, e := range ents {
			if strings.EqualFold(e.Name(), c) {
				p = filepath.Join(p, e.Name())
				found = true
				break
			}
		}
		if !found {
			return "", false
		}
	}
	return p, true
}

func (m *Machine) readPath(name string) (string, bool) {
	drive, parts := m.dosPath(name)
	root := m.driveRoot(drive)
	if root == "" {
		return "", false
	}
	return lookup(root, parts)
}

// writePath is the host path for a new or existing file on C:.
func (m *Machine) writePath(name string) (string, int, bool) {
	drive, parts := m.dosPath(name)
	if drive != 2 || len(parts) == 0 {
		return "", drive, false
	}
	if p, ok := lookup(m.cfg.Save, parts); ok {
		return p, drive, true
	}
	dir, ok := lookup(m.cfg.Save, parts[:len(parts)-1])
	if !ok {
		return "", drive, false
	}
	return filepath.Join(dir, parts[len(parts)-1]), drive, true
}

func (m *Machine) newHandle(f *os.File) uint16 {
	for h := uint16(5); ; h++ {
		if _, ok := m.dos.files[h]; !ok {
			m.dos.files[h] = f
			return h
		}
	}
}

func (m *Machine) open(name string, mode int) {
	var f *os.File
	var err error
	if drive, _ := m.dosPath(name); mode == 0 || mode == 2 && drive != 2 {
		// DOSBox's CD drive opens read/write requests read-only
		p, ok := m.readPath(name)
		if !ok {
			m.logf("open %s: not found", name)
			m.dosErr(2)
			return
		}
		f, err = os.Open(p)
	} else {
		p, _, ok := m.writePath(name)
		if !ok {
			m.logf("open %s mode %d: denied", name, mode)
			m.dosErr(5)
			return
		}
		f, err = os.OpenFile(p, os.O_RDWR, 0)
	}
	if err != nil {
		m.logf("open %s: %v", name, err)
		m.dosErr(2)
		return
	}
	h := m.newHandle(f)
	m.logf("open %s mode %d = %d", name, mode, h)
	m.setAX(h)
}

func (m *Machine) create(name string) {
	p, _, ok := m.writePath(name)
	if !ok {
		m.logf("create %s: denied", name)
		m.dosErr(5)
		return
	}
	f, err := os.Create(p)
	if err != nil {
		m.dosErr(5)
		return
	}
	h := m.newHandle(f)
	m.logf("create %s = %d", name, h)
	m.setAX(h)
}

func (m *Machine) read(h uint16, lin, n uint32) {
	f, ok := m.dos.files[h]
	if !ok {
		if h == 0 {
			m.e.setReg(rEAX, 0)
			return
		}
		m.dosErr(6)
		return
	}
	k, err := io.ReadFull(f, m.mem(lin, int(n)))
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		m.dosErr(5)
		return
	}
	m.e.setReg(rEAX, uint32(k))
}

func (m *Machine) write(h uint16, lin, n uint32) {
	b := m.mem(lin, int(n))
	if h == 1 || h == 2 {
		m.logf("stdout %q", b)
		m.e.setReg(rEAX, n)
		return
	}
	f, ok := m.dos.files[h]
	if !ok {
		m.dosErr(6)
		return
	}
	k, err := f.Write(b)
	if err != nil {
		m.dosErr(5)
		return
	}
	m.e.setReg(rEAX, uint32(k))
}

func (m *Machine) seek(h uint16, whence int, off int64) {
	f, ok := m.dos.files[h]
	if !ok {
		m.dosErr(6)
		return
	}
	pos, err := f.Seek(off, whence)
	if err != nil {
		m.dosErr(25)
		return
	}
	m.setAX(uint16(pos))
	m.e.setReg(rEDX, m.e.reg(rEDX)&^0xffff|uint32(pos>>16)&0xffff)
}

func (m *Machine) chdir(name string) {
	drive, parts := m.dosPath(name)
	root := m.driveRoot(drive)
	if root == "" {
		m.dosErr(3)
		return
	}
	if p, ok := lookup(root, parts); !ok {
		m.dosErr(3)
		return
	} else if fi, err := os.Stat(p); err != nil || !fi.IsDir() {
		m.dosErr(3)
		return
	}
	m.dos.cwd[drive] = strings.Join(parts, "\\")
	m.logf("chdir %s", name)
}

func (m *Machine) findFirst(pat string, attr uint16) {
	drive, parts := m.dosPath(pat)
	root := m.driveRoot(drive)
	m.dos.find.names = nil
	if root != "" && len(parts) > 0 {
		if dir, ok := lookup(root, parts[:len(parts)-1]); ok {
			ents, _ := os.ReadDir(dir)
			for _, e := range ents {
				if (e.IsDir() && attr&0x10 == 0) || !dosMatch(parts[len(parts)-1], strings.ToUpper(e.Name())) {
					continue
				}
				m.dos.find.names = append(m.dos.find.names, filepath.Join(dir, e.Name()))
			}
		}
	}
	m.logf("findfirst %s: %d", pat, len(m.dos.find.names))
	m.findNext()
}

func (m *Machine) findNext() {
	if len(m.dos.find.names) == 0 {
		m.dosErr(18)
		return
	}
	p := m.dos.find.names[0]
	m.dos.find.names = m.dos.find.names[1:]
	fi, err := os.Stat(p)
	if err != nil {
		m.dosErr(18)
		return
	}
	b := m.mem(m.dos.dta, 43)
	clear(b)
	b[0x15] = 0x20
	if fi.IsDir() {
		b[0x15] = 0x10
	}
	sz := uint32(fi.Size())
	b[0x1a], b[0x1b], b[0x1c], b[0x1d] = byte(sz), byte(sz>>8), byte(sz>>16), byte(sz>>24)
	copy(b[0x1e:0x2a], strings.ToUpper(filepath.Base(p)))
}

// dosMatch matches an 8.3 wildcard pattern.
func dosMatch(pat, name string) bool {
	split := func(s string) (string, string) {
		if i := strings.LastIndexByte(s, '.'); i >= 0 {
			return s[:i], s[i+1:]
		}
		return s, ""
	}
	match := func(p, s string, n int) bool {
		for i := 0; i < n; i++ {
			var pc, sc byte = ' ', ' '
			if i < len(p) {
				pc = p[i]
			}
			if i < len(s) {
				sc = s[i]
			}
			if pc == '*' {
				return true
			}
			if pc != '?' && pc != sc {
				return false
			}
		}
		return true
	}
	pb, pe := split(pat)
	nb, ne := split(name)
	return match(pb, nb, 8) && match(pe, ne, 3)
}

var _ fs.FileInfo
