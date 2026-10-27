// Package alis is the ALIS virtual machine of Robinson's Requiem (Silmarils,
// 1994, IBM PC CD release, ALIS 3.1), ported from the alis reimplementation
// (github.com/maestun/alis, MIT) restricted to the paths this game uses, and
// checked against the original START.EXE.
package alis

import "sync/atomic"

const (
	maxScripts = 512

	// Robinson's Requiem PC (main script uid 0x02160ec0)
	gameUID    = 0x02160ec0
	gameWidth  = 320
	gameHeight = 200
	basemem    = 0x2b400
	ramSize    = 4 << 20 // textured RRQ: 4 MB arena
	contextSz  = 0x3e    // script context bytes below vram_org

	kAccOrg        = 0x19c00 // virtual accumulator stack (s32 cells)
	kSpriteMemSize = 0x20000
	sprSz          = 0x36 // alis sprite record size

	// screen element field offsets in a script's vram (v3)
	scrWCX   = 0
	scrWCY   = 8
	scrWCZ   = 16
	scrWCAX  = 0x18
	scrWCAY  = 0x20
	scrWCAZ  = 0x28
	scrGroup = 0x30
	scrAddr  = 0x32
	scrWCX2  = 0x34
	scrWCY2  = 0x38
	scrWCZ2  = 0x3c
)

type vmState int

const (
	stateStopped vmState = iota
	stateRunning
)

type scriptHeader struct {
	id            uint16
	unknown01     uint8
	unknown02     uint8
	codeLocOffset uint16
	retOffset     uint32
	dwUnknown3    uint32
	dwUnknown4    uint32
	wUnknown5     uint16
	vramAllocSz   uint16
	wUnknown7     uint16
}

// scriptData is a loaded program (alis sAlisScriptData).
type scriptData struct {
	name    string
	sz      uint32
	header  scriptHeader
	typ     uint8
	dataOrg uint32
}

// scriptLive is a running entity (alis sAlisScriptLive).
type scriptLive struct {
	name    string
	data    *scriptData
	vramOrg uint32
	vaccOff int16
	running bool
	pc      uint32
	pcOrg   uint32
}

// specs are read from the packed main script header.
type specs struct {
	scriptDataTabLen   uint16
	scriptVramTabLen   uint16
	memCap             uint32
	maxAllocatableVram uint32
	vramToDataOffset   uint32
	scriptVramMaxAddr  uint32
}

// VM is the whole machine state (alis sAlisVM plus the image, audio and
// screen globals).
type VM struct {
	// OnDraw, if set, runs at each draw before the frame wait.
	OnDraw func()

	host  Host
	fs    *FileSys
	in    Input
	kbd   kbdState
	mouse mouseState
	err   *Error
	quit  atomic.Bool

	clk clock // virtual time

	mem   []byte
	state vmState

	specs specs

	atprog   uint32
	debprog  uint32
	finprog  uint32
	dernprog uint32
	maxprog  uint16
	nbprog   uint16
	saversp  int16

	atent   uint32
	debent  uint32
	finent  uint32
	maxent  uint32
	nbent   int16
	dernent int16

	finmem   uint32
	basevar  int32
	basemain uint32
	debsprit uint32 // sprite segment (fs base, 0x15f8c)

	automode uint8
	fallent  uint8
	fseq     uint8
	fmuldes  uint8
	fadddes  uint8
	ferase   uint8

	fview    uint16
	valnorme uint32
	valchamp int16
	ptrent   int // index into tablent
	tablent  [256]int16
	matent   [256]int16

	// mouse
	mousflag  uint8
	fmouse    uint8
	fremouse  uint8
	fremouse2 uint8
	oldmouse  uint16

	theconfig int16

	wcx, wcy, wcz int16
	wforme        int16
	tstinv        uint8 // own form mirrored, latched by the test opcodes (0x1602e)
	matmask       int16
	poldy, poldx  int16

	acc    uint32 // virtual accumulator stack pointer (offset in mem)
	accOrg uint32

	// Robinson's Requiem line tables (offsets in mem)
	pretrlinetra uint32
	pretglinetra uint32
	tlinetra     uint32
	itlinetra    uint32
	trlinetra    uint32
	tglinetra    uint32

	liveScripts   [maxScripts]*scriptLive
	loadedScripts [maxScripts]*scriptData
	script        *scriptLive
	main          *scriptLive

	varD6 int32
	varD7 int32
	varD5 uint16

	// the last operand routine and where its operands start (operAX)
	operCode uint8
	operPC   uint32

	// string registers (offsets in mem)
	bsd7, bsd6, bsd7bis uint32
	sd7, sd6, oldsd7    uint32

	vstandard uint32
	autoname  [13]byte
	tabptr    uint32

	buffer [1024]byte

	charmode uint8
	xlocate  uint8
	ylocate  uint8
	pays     uint8

	// font
	foasc  uint16
	fonum  uint16
	folarg uint8
	fohaut uint8
	fomax  uint16

	witmov   uint8
	fmitmov  uint8
	goodmat  uint16
	baseform uint32

	typepack int8
	wordpack uint8
	longpack uint32

	restartLoop bool

	// virtual status register
	srZero bool
	srNeg  bool

	oevalLoop bool

	fp       File
	openmode uint16

	vquality uint16

	fswitch      uint8
	ctiming      uint8
	cstopret     uint8
	randomNumber uint16
	vprotect     int8

	timeclock uint32

	basedark uint16
	ptrdark  uint32

	cdspeed uint32

	img      image
	aud      audioState
	mv2      mv2State
	film     film
	filmnode uint16

	walk walkState

	px1, py1, pz1, px2, py2, pz2 int16

	timeOffset int
}
