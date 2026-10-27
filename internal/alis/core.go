package alis

import (
	"fmt"
	"strings"
)

// VM start-up, the scheduler and the helpers shared by the opcodes
// (alis alis.c).

// Error is a fatal VM error (alis_error).
type Error struct{ Msg string }

func (e *Error) Error() string { return e.Msg }

// alisErrors are START.EXE's messages for its error screen (table 0x1c626).
var alisErrors = [...]string{
	"...", "open : ", "Erreur fatale, formattage imprimante en cours", "Erreur  fatale,microprocesseur fondu",
	"Erreur fatale, l'ecran va exploser", "trop d'entites vivantes", "programme main attendu", "write",
	"create", "del", "trop de sprites", "debordement programmes",
	"debordement entites", "read", "close", "No de  dessin trop grand",
	"No de forme trop grand", "No de musique trop grand", "Interdit : Vecteur pour forme", "trop de programmes d'entites",
	"Interdit:vecteur multiple", "Live sans load", "débordement de TABFEN", "index tableau négatif",
	"index tableau trop grand", "near/view/approach interdit", "pointeur tableau non alloue", "him négatif - read/byte -",
	"him sur mort - read/byte -", "him négatif - read/word -", "him sur mort - read/word -", "him négatif - write/byte -",
	"him sur mort - write/byte -", "him négatif - write/word -", "him sur mort - write/word -",
}

// alisError is START.EXE's error screen (0x144a0): it shows the code, waits
// for Enter (continue) or Esc (quit). The port stops the VM.
func (v *VM) alisError(code int) {
	msg := ""
	if code < len(alisErrors) {
		msg = alisErrors[code]
	}
	v.fatal(fmt.Sprintf("%s: Error %d:%s", v.script.name, code, msg))
}

func (v *VM) fatal(msg string) {
	if v.err == nil {
		v.err = &Error{msg}
	}
	v.state = stateStopped
	if v.script != nil {
		v.script.running = false
	}
}

// New creates the machine for the game files in fs.
func New(host Host, fs *FileSys) (*VM, error) {
	v := &VM{host: host, fs: fs}
	v.init()
	if v.err != nil {
		return nil, v.err
	}
	return v, nil
}

// Mem is the VM arena (for state comparisons against the C reference).
func (v *VM) Mem() []byte { return v.mem }

// Atent is the address of the entity table in Mem.
func (v *VM) Atent() uint32 { return v.atent }

// Err is the error that stopped the VM, if any.
func (v *VM) Err() error {
	if v.err == nil {
		return nil
	}
	return v.err
}

func (v *VM) init() {
	v.initClock()
	v.cdspeed = 0x493e0
	v.pays = 0

	v.timeclock = 0
	v.aud.fsound = 1
	v.aud.fmusic = 1
	v.aud.musicID = 0xffff
	v.restartLoop = false
	v.automode = 0
	v.theconfig = 0x40 // fast CD (START.EXE 0x330b); initInput adds the mouse

	v.img = image{}
	v.dosPalReset()
	for i := range v.img.dkpalet {
		v.img.dkpalet[i] = 0x100
	}

	// START.EXE's flat memory: its data at their addresses, then the block
	// from the extender (0x1b97)
	v.mem = make([]byte, memTop)
	if !v.loadData() {
		return
	}
	v.setFinmem(memTop) // 0x15f26
	v.wu32(0x15f2a, memTop)
	v.wu32(0x15f22, progBase-gameWidth*gameHeight)
	v.initScreens()
	v.wu32(0x15f22, progBase)
	v.landTables() // 0x1bf7

	v.fswitch = 1 // dbl_buf
	v.fallent = 0
	v.fseq = 0
	v.fmuldes = 0
	v.fadddes = 0
	v.saversp = 0
	v.basevar = 0
	v.nbprog = 0
	v.maxprog = 0
	v.nbent = 0
	v.ptrent = 0
	v.ctiming = 0
	v.varD6, v.varD7 = 0, 0

	// string registers (0x377f) and the standard vectors (corient 0x6056)
	v.bsd7 = 0x15b8e
	v.bsd6 = 0x15c8e
	v.bsd7bis = 0x15d8e
	v.vstandard = 0x19686
	v.sd7 = v.bsd7
	v.sd6 = v.bsd6
	v.oldsd7 = v.bsd7bis
	v.fp = nil
	v.acc = kAccOrg
	v.accOrg = kAccOrg

	v.loadMain()
	if v.main == nil {
		if v.err == nil {
			v.fatal("could not load the main script")
		}
		return
	}
	v.script = v.main
}

// startExe holds START.EXE's 32-bit image from this file offset; its data
// lie at dataStart..dataEnd, the zeroed rest up to 0x1cd50.
const (
	startExe     = "start.exe"
	startExeBase = 0x4857
	dataStart    = 0x14d80
	dataEnd      = 0x1cba8
)

func (v *VM) loadData() bool {
	raw, err := v.fs.ReadFile(startExe)
	if err == nil && len(raw) < startExeBase+dataEnd {
		err = fmt.Errorf("too short")
	}
	if err != nil {
		v.fatal("START.EXE: " + err.Error())
		return false
	}
	copy(v.mem[dataStart:dataEnd], raw[startExeBase+dataStart:])
	return true
}

const mainScript = "main.io"

// loadMain reads the VM specs from the main script header, lays out the
// memory and starts the main script (START.EXE 0xad33).
func (v *VM) loadMain() {
	v.nbprog = 0
	v.main = nil
	raw, err := v.fs.ReadFile(mainScript)
	if err != nil {
		v.fatal("main script: " + err.Error())
		return
	}
	v.vprotect = 0
	copy(v.mem[0x1802c:0x18042], raw)
	h := raw[6:]
	v.specs.scriptDataTabLen = le16(h[0:])
	v.specs.scriptVramTabLen = le16(h[2:])
	v.specs.memCap = le32(h[4:])
	v.specs.maxAllocatableVram = uint32(le16(h[8:]))
	v.specs.vramToDataOffset = uint32(le16(h[12:]))

	v.atprog = v.ru32(0x15f22)
	v.wu32(0x15f5e, v.atprog)
	v.wu32(0x15f6e, v.atprog)
	v.atent = v.atprog + (uint32(v.specs.scriptDataTabLen)>>2+1)<<4
	v.wu32(0x15f72, v.atent)
	v.maxent = uint32(v.specs.scriptVramTabLen)
	v.debent = v.atent + v.maxent*6
	v.finent = v.debent
	v.wu32(0x15f76, v.debent)
	v.wu32(0x15f7a, v.finent)
	debsprit := v.debent + (v.specs.maxAllocatableVram | 0xf) + 1
	v.wu32(dsDebsprit, debsprit)
	finsprit := debsprit + ((v.specs.vramToDataOffset+3)*0x30 | 0xf) + 1
	v.wu32(dsFinsprit, finsprit)
	v.debprog = finsprit
	v.setFinprog(finsprit)
	v.dernprog = v.atprog
	v.maxprog = v.specs.scriptDataTabLen
	if top := finsprit + v.specs.memCap; top <= v.finmem() {
		v.setFinmem(top)
	}
	v.mem[0x16077] = 0xff
	v.wu16(0x15f38, 0xffff)

	// entity table (0xb7a8): vram and the next entry
	for i := uint32(1); i < v.maxent; i++ {
		v.wu16(v.atent+i*6-2, uint16(i*6))
		v.wu32(v.atent+i*6-6, 0)
	}
	v.wu16(v.atent+4, 0)
	v.nbent = 1
	v.dernent = 6

	v.inisprit()

	prog := v.scriptLoad(mainScript)
	if prog == nil {
		return
	}
	v.main = v.scriptCreate(prog, 0)
	v.wu16(v.main.vramOrg-0x16, 0)
	v.basemain = v.main.vramOrg
	v.wu32(dsBasemain, v.basemain)
}

// ---------------------------------------------------------------------------
// script stream

func (v *VM) read8() uint8 {
	b := v.mem[v.script.pc]
	v.script.pc++
	return b
}

func (v *VM) read16() int16 {
	r := v.r16(v.script.pc)
	v.script.pc += 2
	return r
}

func (v *VM) read32() int32 {
	r := v.r32(v.script.pc)
	v.script.pc += 4
	return r
}

// readUntilZero copies a zero-terminated string from the script to dst.
func (v *VM) readUntilZero(dst uint32) {
	for {
		c := v.read8()
		v.mem[dst] = c
		dst++
		if c == 0 {
			return
		}
	}
}

// readZeroString reads a zero-terminated string from the script.
func (v *VM) readZeroString() string {
	var sb strings.Builder
	for {
		c := v.read8()
		if c == 0 {
			return sb.String()
		}
		sb.WriteByte(c)
	}
}

// ---------------------------------------------------------------------------
// C strings in VM memory

func (v *VM) strlen(a uint32) uint32 {
	n := uint32(0)
	for v.mem[a+n] != 0 {
		n++
	}
	return n
}

func (v *VM) cstr(a uint32) string { return string(v.mem[a : a+v.strlen(a)]) }

func (v *VM) strcpy(dst, src uint32) {
	n := v.strlen(src)
	copy(v.mem[dst:dst+n+1], v.mem[src:src+n+1])
}

func (v *VM) strcat(dst, src uint32) { v.strcpy(dst+v.strlen(dst), src) }

func (v *VM) setCstr(dst uint32, s string) {
	copy(v.mem[dst:], s)
	v.mem[dst+uint32(len(s))] = 0
}

// strncmp(a, b, strlen(a)) as the string comparisons use it.
func (v *VM) strncmpA(a, b uint32) int {
	n := v.strlen(a)
	for i := uint32(0); i < n; i++ {
		x, y := v.mem[a+i], v.mem[b+i]
		if x != y {
			return int(x) - int(y)
		}
		if x == 0 {
			return 0
		}
	}
	return 0
}

// ---------------------------------------------------------------------------
// accumulator stack

func (v *VM) push(x int32) {
	v.acc -= 4
	v.w32(v.acc, x)
}

func (v *VM) pop() int32 {
	x := v.r32(v.acc)
	v.acc += 4
	return x
}

// ---------------------------------------------------------------------------
// dispatch

type opfn func(*VM)

var (
	opcodes     [256]opfn
	codesc1     [256]opfn
	opernames   [256]opfn
	storenames  [256]opfn
	addnames    [256]opfn
	opcodeNames [256]string
)

func (v *VM) badOp(table string, code uint8) {
	v.fatal(fmt.Sprintf("%s: opcode 0x%02x missing in %s table (pc 0x%x)", v.script.name, code, table, v.script.pc-1))
}

func (v *VM) readexecOpcode() {
	code := v.read8()
	if f := opcodes[code]; f != nil {
		f(v)
		return
	}
	v.badOp("opcode", code)
}

func (v *VM) readexecCodesc1() {
	code := v.read8()
	if f := codesc1[code]; f != nil {
		f(v)
		return
	}
	v.badOp("codesc1", code)
}

func (v *VM) readexecCodesc2() {
	code := v.read8()
	if code == 0 {
		cnul(v)
		return
	}
	v.badOp("codesc2", code)
}

func (v *VM) readexecCodesc3() {
	code := v.read8()
	if code == 0 {
		cnul(v)
		return
	}
	v.badOp("codesc3", code)
}

func (v *VM) readexecOpername() {
	code := v.read8()
	v.operCode, v.operPC = code, v.script.pc
	if f := opernames[code]; f != nil {
		f(v)
	} else {
		v.badOp("opername", code)
	}
	v.varD7 = int32(int16(v.varD7))
}

func (v *VM) readexecStorename() {
	code := v.read8()
	if f := storenames[code]; f != nil {
		f(v)
		return
	}
	v.badOp("storename", code)
}

func (v *VM) readexecAddname() {
	code := v.read8()
	if f := addnames[code]; f != nil {
		f(v)
		return
	}
	v.badOp("addname", code)
}

func (v *VM) readexecAddnameSwap() {
	v.sd7, v.oldsd7 = v.oldsd7, v.sd7
	v.readexecAddname()
}

// gparam
func (v *VM) readexecOpernameSaveD7() {
	v.varD6 = v.varD7
	v.readexecOpername()
}

// gparam1
func (v *VM) readexecOpernameSaveD6() {
	tmp := int16(v.varD7)
	v.readexecOpernameSaveD7()
	v.varD6 = v.varD7
	v.varD7 = int32(tmp)
}

func (v *VM) readexecOpernameSwap() {
	v.sd7, v.sd6 = v.sd6, v.sd7
	v.readexecOpername()
}

// ---------------------------------------------------------------------------
// scheduler

func (v *VM) loop() {
	v.script.running = true
	for v.state != stateStopped && v.script.running {
		v.readexecOpcode()
	}
}

func (v *VM) entscr(ent uint16) *scriptLive { return v.liveScripts[ent/6] }

// Run executes the game until it quits or fails (alis_main_V3).
func (v *VM) Run() (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("alis: %v (script %s pc 0x%x)", r, v.scriptName(), v.scriptPC())
			panic(err)
		}
	}()
	v.state = stateRunning
	v.cstopret = 0
	v.varD5 = 0
	v.initInput()
	v.startupTicks()

	for v.state != stateStopped {
		v.restartLoop = false
		v.script = v.entscr(v.varD5)
		v.fallent = 0
		v.fseq = 0
		v.acc = v.accOrg

		vram := v.script.vramOrg
		org := uint32(v.r32(vram - 0x14))
		if si := int8(v.r8(vram - 0x24)); si < 0 && si&2 == 0 {
			if off := v.r32(org + 10); off != 0 {
				v.saversp = v.r16(vram - 0x0a)
				v.script.vaccOff = v.saversp
				v.savecoord(vram)
				v.script.pc = org + 10 + uint32(off)
				v.loop()
				v.updtcoord(v.script.vramOrg)
			}
		}

		vram = v.script.vramOrg
		if cs := v.r8(vram - 4); cs != 0 {
			if int8(cs) < 0 {
				v.w8(vram-4, 1)
			}
			if int32(v.timeclock) >= v.r32(vram-0x3e) { // signed, as START.EXE 0x387b
				v.wu32(vram-0x3e, uint32(v.ru16(vram-0x3a))+v.timeclock)
				v.savecoord(vram)
				v.script.pc = uint32(v.r32(vram - 0x08))
				v.script.vaccOff = v.r16(vram - 0x0a)
				v.fseq++
				v.loop()

				if !v.restartLoop {
					vram = v.script.vramOrg
					v.w16(vram-0x0a, v.script.vaccOff)
					v.w32(vram-0x08, int32(v.script.pc))
					org = uint32(v.r32(vram - 0x14))
					if off := v.r32(org + 6); off != 0 {
						v.fseq = 0
						v.saversp = v.r16(vram - 0x0a)
						v.script.vaccOff = v.saversp
						v.script.pc = org + 6 + uint32(off)
						v.loop()
					}
					if !v.restartLoop {
						v.updtcoord(v.script.vramOrg)
					}
				}
			}
		}

		v.varD5 = v.ru16(v.atent + 4 + uint32(v.varD5))
		if v.varD5 == 0 {
			v.script = v.entscr(0)
			v.draw()
		}
	}
	v.sleepUntilMusicStopsAtExit()
	return v.Err()
}

func (v *VM) scriptName() string {
	if v.script == nil {
		return "-"
	}
	return v.script.name
}

func (v *VM) scriptPC() uint32 {
	if v.script == nil {
		return 0
	}
	return v.script.pc
}

// Stop asks the VM to quit (from another goroutine).
func (v *VM) Stop() { v.quit.Store(true) }

// ---------------------------------------------------------------------------
// resources

func (v *VM) scriptOrg(vram uint32) uint32 { return uint32(v.r32(vram - 0x14)) }

func (v *VM) adresdes(idx int32) int32 {
	vram := v.script.vramOrg
	if v.mem[dsFlagmain] != 0 {
		vram = v.main.vramOrg
	}
	addr := v.scriptOrg(vram)
	addr += uint32(v.r32(addr + 0xe))
	if l := int32(v.r16(addr + 4)); l > idx {
		return int32(addr) + v.r32(addr) + idx*4
	}
	return 0xf
}

func (v *VM) adresform(idx int16) int32 {
	addr := v.scriptOrg(v.script.vramOrg)
	addr += uint32(v.r32(addr + 0xe))
	if l := int32(v.r16(addr + 0xa)); l > int32(idx) {
		addr += uint32(v.r32(addr + 6))
		return int32(addr) + int32(v.r16(addr+uint32(int32(idx)*2)))
	}
	return 0
}

func (v *VM) adresmus(idx int32) int32 {
	vram := v.script.vramOrg
	if v.mem[dsFlagmain] != 0 {
		vram = v.main.vramOrg
	}
	mem := v.scriptOrg(vram)
	off := v.r32(mem + 0xe)
	addr := mem + uint32(off)
	if l := int32(v.r16(addr + 0x10)); l > idx {
		at := v.r32(addr+0xc) + off + idx*4
		return int32(mem) + v.r32(mem+uint32(at)) + at
	}
	return int32(mem) + 0x11
}

// tabint/tabchar/tabstring resolve an array cell; the extra dimension
// indices are popped from the accumulator.
func (v *VM) tabint(addr uint32) int32 {
	result := int32(addr) + v.varD7*2
	addr--
	length := int8(v.r8(addr))
	addr--
	if length < 0 {
		na := uint32(v.r32(addr + 2))
		na = uint32(v.r32(na))
		result = int32(na) + v.varD7*2
		length &= 0xf
		for i := int8(0); i < length; i++ {
			addr -= 4
			result += v.r32(addr) * v.pop()
		}
	} else {
		length &= 0xf
		for i := int8(0); i < length; i++ {
			addr -= 2
			result += int32(v.ru16(addr)) * v.pop()
		}
	}
	return result
}

func (v *VM) tabchar(addr uint32) int32 {
	result := int32(addr) + v.varD7
	addr--
	length := int8(v.r8(addr))
	addr--
	if length < 0 {
		na := uint32(v.r32(addr + 2))
		na = uint32(v.r32(na))
		result = int32(na) + v.varD7
		length &= 0xf
		for i := int8(0); i < length; i++ {
			addr -= 4
			result += v.r32(addr) * v.pop()
		}
	} else {
		length &= 0xf
		for i := int8(0); i < length; i++ {
			addr -= 2
			result += int32(v.ru16(addr)) * v.pop()
		}
	}
	return result
}

func (v *VM) tabstring(addr uint32) int32 {
	result := int32(addr)
	addr--
	length := int8(v.r8(addr))
	addr--
	result += v.varD7 * int32(v.r8(addr))
	if length < 0 {
		na := uint32(v.r32(addr + 2))
		na = uint32(v.r32(na))
		result = int32(na) + v.varD7*int32(v.r8(addr+1))
		length &= 0xf
		for i := int8(0); i < length; i++ {
			addr -= 4
			result += v.r32(addr) * v.pop()
		}
	} else {
		length &= 0xf
		for i := int8(0); i < length; i++ {
			addr -= 2
			result += int32(v.ru16(addr)) * v.pop()
		}
	}
	return result
}

// afopen opens a game file and detects the PB60/PW60 packed formats
// (open mode 0x800).
func (v *VM) afopen(name string, openmode uint16) File {
	exists := v.fs.Exists(name)
	fp, err := v.fs.Open(name, openmode)
	if err != nil {
		fp = nil
	}
	v.fp = fp
	v.typepack = 0
	v.openmode = openmode
	if (openmode&0x100 == 0 || exists) && openmode&0x200 == 0 && fp != nil && openmode&0x800 != 0 {
		var hdr [12]byte
		fp.Read(hdr[:])
		typ := be32(hdr[0:])
		switch typ {
		case 0x50423630:
			v.typepack = -0x60 // 0xa0
			v.wordpack = 0
			v.longpack = be32(hdr[4:])
		case 0x50573630:
			v.typepack = -0x60
			v.wordpack = 1
			v.longpack = be32(hdr[4:])
		}
	}
	return fp
}

func be32(b []byte) uint32 {
	return uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])
}
func be16(b []byte) uint16 { return uint16(b[0])<<8 | uint16(b[1]) }
