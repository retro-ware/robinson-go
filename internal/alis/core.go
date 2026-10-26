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
var alisErrors = map[int]string{
	0x14: "Interdit:vecteur multiple",
	0x1b: "him négatif - read/byte -",
	0x1d: "him négatif - read/word -",
	0x1f: "him négatif - write/byte -",
	0x21: "him négatif - write/word -",
}

// alisError is START.EXE's error screen (0x144a0): it shows the code, waits
// for Enter (continue) or Esc (quit). The port stops the VM.
func (v *VM) alisError(code int) {
	v.fatal(fmt.Sprintf("%s: Error %d:%s", v.script.name, code, alisErrors[code]))
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
	v.render3dInit()

	v.timeclock = 0
	v.aud.fsound = 1
	v.aud.fmusic = 1
	v.aud.musicID = 0xffff
	v.restartLoop = false
	v.automode = 0
	v.theconfig = 0x87 // sound + video + high memory

	v.img = image{}
	v.dosPalReset()
	for i := range v.img.dkpalet {
		v.img.dkpalet[i] = 0x100
	}
	v.img.flinepal = 0
	v.img.tlinepal[0] = 0
	v.img.tlinepal[1] = 0
	v.img.tlinepal[2] = 0xff
	v.img.tlinepal[3] = 0
	v.img.fmouse = -1
	v.img.fonum = -1
	v.img.loglarg = 0xa0

	v.mem = make([]byte, ramSize)
	v.img.spritemem = make([]byte, kSpriteMemSize)
	v.img.physic = make([]byte, gameWidth*gameHeight)
	v.img.logic = make([]byte, gameWidth*gameHeight)
	v.img.logx1 = 0
	v.img.logx2 = gameWidth - 1
	v.img.logy1 = 0
	v.img.logy2 = gameHeight - 1

	v.fswitch = 1 // dbl_buf
	v.flagmain = 0
	v.fallent = 0
	v.fseq = 0
	v.fmuldes = 0
	v.fadddes = 0
	v.ferase = 0
	v.saversp = 0
	v.basevar = 0
	v.finmem = basemem - 0x400
	v.nbprog = 0
	v.maxprog = 0
	v.atprog = basemem
	v.nbent = 0
	v.ptrent = 0
	v.ctiming = 0
	v.prevkey = 0
	v.varD6, v.varD7 = 0, 0

	v.bsd7 = 0x1a1e6
	v.bsd6 = 0x1a2e6
	v.bsd7bis = 0x1a3e6
	v.vstandard = 0x19d16
	v.tabptr = 0x19c00
	v.sd7 = v.bsd7
	v.sd6 = v.bsd6
	v.oldsd7 = v.bsd7bis
	v.fp = nil
	v.acc = kAccOrg
	v.accOrg = kAccOrg

	v.pretrlinetra = 0x1ef7c
	v.pretglinetra = 0x1f58e
	v.tlinetra = 0x1e602
	v.itlinetra = 0x1e802
	v.trlinetra = 0x1f07e
	v.tglinetra = 0x1f690
	v.fmouse = 0xff

	v.loadMain()
	if v.main == nil {
		if v.err == nil {
			v.fatal("could not load the main script")
		}
		return
	}
	v.script = v.main
	v.basemain = v.main.vramOrg
	v.desmouse = 0

	v.img.zoombid = 0x20de2
	v.img.landdata = 0x1ccc2
}

const mainScript = "main.io"

// loadMain reads the VM specs from the main script header, lays out the
// arena and starts the main script (alis_load_main).
func (v *VM) loadMain() {
	v.nbprog = 0
	v.main = nil
	raw, err := v.fs.ReadFile(mainScript)
	if err != nil {
		v.fatal("main script: " + err.Error())
		return
	}
	v.vprotect = 0
	h := raw[6:]
	v.specs.scriptDataTabLen = le16(h[0:])
	v.specs.scriptVramTabLen = le16(h[2:])
	v.specs.memCap = le32(h[4:])
	v.specs.maxAllocatableVram = le32(h[8:])
	v.specs.vramToDataOffset = (le32(h[12:]) + 3) * 0x30

	v.atprog = basemem
	v.atent = v.atprog + uint32(v.specs.scriptDataTabLen)*4
	v.maxent = uint32(v.specs.scriptVramTabLen)
	v.debent = v.atent + v.maxent*6
	v.finent = v.debent

	v.img.debsprit = int32(((v.debent + v.specs.maxAllocatableVram) | 0xf) + 1)
	v.img.finsprit += v.img.debsprit + int32(v.specs.vramToDataOffset)

	v.debprog = uint32(v.img.finsprit)
	v.finprog = v.debprog
	v.dernprog = v.atprog
	v.maxprog = v.specs.scriptDataTabLen

	v.finmem = ramSize - 0x9168
	if v.specs.memCap != 0 && v.debprog+v.specs.memCap < v.finmem {
		v.finmem = v.debprog + v.specs.memCap
	}

	v.inisprit()

	// entity table: vram (s32) + next link (u16) per entry
	for idx := uint32(0); idx < uint32(v.specs.scriptVramTabLen); idx++ {
		v.w32(v.atent+idx*6, 0)
		v.wu16(v.atent+idx*6+4, uint16((1+idx)*6))
	}
	v.specs.scriptVramMaxAddr = ((v.debent + v.specs.maxAllocatableVram) | 0xf) + 1

	prog := v.scriptLoad(mainScript)
	if prog == nil {
		return
	}
	v.main = v.scriptLive(prog)
	v.basemain = v.main.vramOrg

	v.dernent = v.r16(v.atent + 4)
	v.wu16(v.atent+4, 0)
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

func (v *VM) savecoord(addr uint32) {
	v.img.oldcx = v.r16(addr + scrWCX)
	v.img.oldcy = v.r16(addr + scrWCY)
	v.img.oldcz = v.r16(addr + scrWCZ)
	v.img.oldacx = v.r16(addr + scrWCAX)
	v.img.oldacy = v.r16(addr + scrWCAY)
	v.img.oldacz = v.r16(addr + scrWCAZ)
}

func normAngle(angle int16) int16 {
	if 0x168 < angle {
		angle -= 0x168
		if 0x168 < angle {
			angle %= 0x168
		}
	}
	if angle < -0x168 {
		angle += 0x168
		if angle < -0x168 {
			angle %= 0x168
		}
	}
	return angle
}

func (v *VM) updtcoord(addr uint32) {
	angle := v.r16(addr + scrWCAX)
	if angle != v.img.oldacx {
		v.w16(addr+scrWCAX, normAngle(angle))
	}
	angle = v.r16(addr + scrWCAY)
	if angle != v.img.oldacy {
		v.w16(addr+scrWCAY, normAngle(angle))
	}
	angle = v.r16(addr + scrWCAZ)
	if angle != v.img.oldacz {
		angle = normAngle(angle)
		v.w16(addr+scrWCAZ, angle)
	}

	addx := v.r16(addr+scrWCX) - v.img.oldcx
	addy := v.r16(addr+scrWCY) - v.img.oldcy
	addz := v.r16(addr+scrWCZ) - v.img.oldcz
	wcaz := v.ru16(addr + scrWCAZ)
	if angle != v.img.oldacz || addz != 0 || addx != 0 || addy != 0 {
		for s := v.ru16(v.script.vramOrg - 0x18); s != 0; s = v.sprTonext(s) {
			sp := v.spr(s)
			if sp.state() == 0 {
				sp.setState(2)
			}
			sp.setDepx(sp.depx() + addx)
			sp.setDepy(sp.depy() + addy)
			sp.setDepz(sp.depz() + addz)
			sp.setSprite0x28(sp.sprite0x28()&0xffff | uint32(wcaz)<<16)
		}
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
	v.initTimers()

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
			if v.ru32(vram-0x3e) <= v.timeclock {
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
	if v.flagmain != 0 {
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
	if v.flagmain != 0 {
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
