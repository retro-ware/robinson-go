//go:build difftest

// Package cref links the C reference (maestun/alis with tools/alis-ref.patch,
// built as ref/alis/build/libalisref.a) so tests can run its functions on the
// same state as the Go port.
package cref

/*
#cgo CFLAGS: -I${SRCDIR}/../../../ref/alis/src -I${SRCDIR}/../../../ref/alis/src/audio -I${SRCDIR}/../../../ref/alis/src/render -I${SRCDIR}/../../../ref/alis/src/video -I${SRCDIR}/../../../ref/alis/src/emu2149 -I${SRCDIR}/../../../ref/alis/src/emu8950 -DALIS_CONV_INPLACE=0 -DNDEBUG
#cgo pkg-config: sdl2
#cgo LDFLAGS: ${SRCDIR}/../../../ref/alis/build/libalisref.a -lz -lm
#include <stdlib.h>
#include <string.h>
#include "alis.h"
#include "platform.h"
#include "harness.h"
#include "sys/sys.h"

void sys_init(sPlatform *pl, int fullscreen, int mutesound);

static int cref_init(const char *path)
{
    sys_errors_init();
    sPlatform *pl = pl_guess(path, 0);
    if (!pl_supported(pl))
        return 1;
    harness_init();
    sys_init(pl, 0, 1);
    return alis_init(*pl);
}
int unpack_script(const char *packed_file_path, u8 *unpacked_buffer);
static int cref_unpack_script(const char *path, u8 *out, u32 magic)
{
    alis.typepack = magic >> 24;
    alis.typepack &= 0xfe;
    return unpack_script(path, out);
}
void unpack_old(u8* ptr_packed, const u32 packed_sz, u8* ptr_unpacked, const u32 unpacked_sz, s8 modpack);
u32 unpack_new(u8* ptr_packed, const u32 packed_sz, u8* ptr_unpacked, const u32 unpacked_sz, u8* dictionary);
void harness_set_maxms(u64 ms);
u8 sys_start(void);
#include "script.h"
typedef struct { s32 d6, d7; u16 d5; u32 acc, bsd7, bsd6, bsd7bis, sd7, sd6, oldsd7; u8 zero, neg; u32 pc, vram_org; s16 vacc_off; u8 running; u8 fseq; s16 saversp; u16 rnd; } cref_regs;
#define OFF(p) ((u32)((u8 *)(p) - alis.mem))
static void cref_get_regs(cref_regs *r)
{
    r->d6 = alis.varD6; r->d7 = alis.varD7; r->d5 = alis.varD5;
    r->acc = OFF(alis.acc);
    r->bsd7 = OFF(alis.bsd7); r->bsd6 = OFF(alis.bsd6); r->bsd7bis = OFF(alis.bsd7bis);
    r->sd7 = OFF(alis.sd7); r->sd6 = OFF(alis.sd6); r->oldsd7 = OFF(alis.oldsd7);
    r->zero = alis.sr.zero; r->neg = alis.sr.neg;
    r->pc = alis.script->pc; r->vram_org = alis.script->vram_org; r->vacc_off = alis.script->vacc_off;
    r->running = alis.script->running;
    r->fseq = alis.fseq; r->saversp = alis.saversp; r->rnd = alis.random_number;
}
static void cref_set_regs(const cref_regs *r)
{
    alis.varD6 = r->d6; alis.varD7 = r->d7; alis.varD5 = r->d5;
    alis.acc = (s32 *)(alis.mem + r->acc);
    alis.bsd7 = (char *)alis.mem + r->bsd7; alis.bsd6 = (char *)alis.mem + r->bsd6; alis.bsd7bis = (char *)alis.mem + r->bsd7bis;
    alis.sd7 = (char *)alis.mem + r->sd7; alis.sd6 = (char *)alis.mem + r->sd6; alis.oldsd7 = (char *)alis.mem + r->oldsd7;
    alis.sr.zero = r->zero; alis.sr.neg = r->neg;
    alis.script->pc = r->pc; alis.script->vacc_off = r->vacc_off; alis.script->running = r->running;
    alis.fseq = r->fseq; alis.saversp = r->saversp; alis.random_number = r->rnd;
    alis.state = eAlisStateRunning;
}
typedef struct { s16 wcx, wcy, wcz, wforme, matmask; s32 ptrent; s16 tablent[256], matent[256]; u8 witmov, fmitmov, fallent, tstinv; u16 goodmat; } cref_ent;
extern u8 tstinv;
static void cref_get_ent(cref_ent *e)
{
    e->wcx = alis.wcx; e->wcy = alis.wcy; e->wcz = alis.wcz; e->wforme = alis.wforme; e->matmask = alis.matmask;
    e->ptrent = (s32)(alis.ptrent - alis.tablent);
    memcpy(e->tablent, alis.tablent, sizeof e->tablent); memcpy(e->matent, alis.matent, sizeof e->matent);
    e->witmov = alis.witmov; e->fmitmov = alis.fmitmov; e->fallent = alis.fallent; e->goodmat = alis.goodmat; e->tstinv = tstinv;
}
static void cref_set_ent(const cref_ent *e)
{
    alis.wcx = e->wcx; alis.wcy = e->wcy; alis.wcz = e->wcz; alis.wforme = e->wforme; alis.matmask = e->matmask;
    alis.ptrent = alis.tablent + e->ptrent;
    memcpy(alis.tablent, e->tablent, sizeof e->tablent); memcpy(alis.matent, e->matent, sizeof e->matent);
    alis.witmov = e->witmov; alis.fmitmov = e->fmitmov; alis.fallent = e->fallent; alis.goodmat = e->goodmat; tstinv = e->tstinv;
}
static void cref_step(void) { tab_opcodes[*(alis.mem + alis.script->pc++)].fptr(); }
static u8 *cref_mem(void) { return alis.mem; }
static u32 cref_memsize(void) { return alis.platform.ram_sz; }

#include "channel.h"
#include "audio.h"
void sys_audio_callback_S16MSB(void *userdata, u8 *stream, s32 len);
u32 fli_audio_push(const s8 *src, u32 len, s16 freq);
extern volatile u8 fli_audio_q_head, fli_audio_q_tail;
extern u32 isr_counter;

static void cref_set_channel(int i, int type, u32 addr, u32 played, s16 volume, u32 length, s16 freq, u8 loop, s8 curson)
{
    sChannel *c = &audio.channels[i];
    c->type = type; c->address = (s8 *)(alis.mem + addr); c->played = played; c->volume = volume;
    c->length = length; c->freq = freq; c->loop = loop; c->curson = curson;
}
static void cref_get_channel(int i, int *type, u32 *addr, u32 *played, u32 *length, s16 *freq, u8 *loop, s8 *curson)
{
    sChannel *c = &audio.channels[i];
    *type = c->type; *addr = (u32)((u8 *)c->address - alis.mem); *played = c->played; *length = c->length;
    *freq = c->freq; *loop = c->loop; *curson = c->curson;
}
static void cref_mix(s16 *out, int frames) { sys_audio_callback_S16MSB(NULL, (u8 *)out, frames * 4); }
static void cref_audio_reset(void) { audio.muflag = 0; audio.fsound = 1; fli_audio_q_head = fli_audio_q_tail = 0; isr_counter = 0; for (int i = 0; i < 4; i++) audio.channels[i].type = 0; }
extern sMV2Audio mv2a;
extern u8 mustereo;
void mv2_calculfrq(void);
void mv2_soundrout(void);
static void cref_mv2_prepare(void) { mv2a.prevmufreq = 0; mv2_calculfrq(); mustereo = 1; audio.soundrout = mv2_soundrout; }
static u32 cref_fli_push(u32 addr, u32 len, s16 freq) { return fli_audio_push((const s8 *)(alis.mem + addr), len, freq); }
*/
import "C"

import (
	"fmt"
	"os"
	"unsafe"
)

var inited bool

// Init loads the game in dataDir into the C VM (once per process). It runs
// in virtual-time harness mode, without a window or audio device.
var dir string

// Run runs the game from its current state until virtual time ms.
func Run(ms uint64) {
	wd, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(wd)
	C.harness_set_maxms(C.u64(ms))
	C.sys_start()
}

func Init(dataDir string) error {
	if inited {
		return nil
	}
	os.Setenv("ALIS_DET", "1")
	wd, _ := os.Getwd()
	if err := os.Chdir(dataDir); err != nil {
		return err
	}
	defer os.Chdir(wd)
	p := C.CString(".")
	defer C.free(unsafe.Pointer(p))
	if C.cref_init(p) != 0 {
		return fmt.Errorf("cref: alis_init failed")
	}
	inited = true
	dir = dataDir
	return nil
}

// Mem is the C VM arena (shared, not a copy).
func Mem() []byte {
	return unsafe.Slice((*byte)(unsafe.Pointer(C.cref_mem())), int(C.cref_memsize()))
}

// Channel mirrors the fields of a C sChannel the mixer reads and writes.
// Addr is an arena offset (film chunks live outside the arena).
type Channel struct {
	Type         int
	Addr, Played uint32
	Volume       int16
	Length       uint32
	Freq         int16
	Loop         uint8
	Curson       int8
}

func AudioReset() { C.cref_audio_reset() }

func SetChannel(i int, c Channel) {
	C.cref_set_channel(C.int(i), C.int(c.Type), C.u32(c.Addr), C.u32(c.Played), C.s16(c.Volume), C.u32(c.Length), C.s16(c.Freq), C.u8(c.Loop), C.s8(c.Curson))
}

func GetChannel(i int) Channel {
	var c Channel
	var typ C.int
	var addr, played, length C.u32
	var freq C.s16
	var loop C.u8
	var curson C.s8
	C.cref_get_channel(C.int(i), &typ, &addr, &played, &length, &freq, &loop, &curson)
	c.Type, c.Addr, c.Played, c.Length, c.Freq, c.Loop, c.Curson = int(typ), uint32(addr), uint32(played), uint32(length), int16(freq), uint8(loop), int8(curson)
	return c
}

// Mix runs the audio callback for len(out)/2 stereo frames.
func Mix(out []int16) { C.cref_mix((*C.s16)(unsafe.Pointer(&out[0])), C.int(len(out)/2)) }

// FliPush queues len bytes at arena offset addr as film speech.
func FliPush(addr, length uint32, freq int16) uint32 {
	return uint32(C.cref_fli_push(C.u32(addr), C.u32(length), C.s16(freq)))
}

// Voice mirrors sAudioVoice.
type Voice struct {
	Freqsam             uint16
	Startsam1, Longsam1 uint32
	Volsam              uint16
	Startsam2, Longsam2 uint32
	Value               int16
	Type, Delta         uint8
	Loopsam, Frqto      uint16
	Sample              uint32
}

// MV2 mirrors the mv2a and audio fields the sample player uses.
type MV2 struct {
	Voices                              [4]Voice
	Muptr, Mumax, Mucnt, Mubufa, Mubufc uint16
	Muspeed, Muvolgen, Mubreak          uint16
	Mupnote                             uint32
	Muvol, Mutempo, Mutemp              uint8
	Muattac, Muchute                    uint16
	Muduree                             int16
	Muflag                              int32
	Left, Right                         []int16
}

func SetMV2(m *MV2) {
	for i, v := range m.Voices {
		c := &C.mv2a.voices[i]
		c.freqsam, c.startsam1, c.longsam1, c.volsam = C.u16(v.Freqsam), C.u32(v.Startsam1), C.u32(v.Longsam1), C.u16(v.Volsam)
		c.startsam2, c.longsam2, c.value, c._type, c.delta = C.u32(v.Startsam2), C.u32(v.Longsam2), C.s16(v.Value), C.u8(v.Type), C.u8(v.Delta)
		c.loopsam, c.frqto, c.sample = C.u16(v.Loopsam), C.u16(v.Frqto), C.u32(v.Sample)
	}
	a := &C.mv2a
	a.muptr, a.mumax, a.mucnt, a.mubufa, a.mubufc = C.u16(m.Muptr), C.u16(m.Mumax), C.u16(m.Mucnt), C.u16(m.Mubufa), C.u16(m.Mubufc)
	a.muspeed, a.muvolgen, a.mubreak = C.u16(m.Muspeed), C.u16(m.Muvolgen), C.u16(m.Mubreak)
	C.audio.mupnote, C.audio.muvol, C.audio.mutempo, C.audio.mutemp = C.u32(m.Mupnote), C.u8(m.Muvol), C.u8(m.Mutempo), C.u8(m.Mutemp)
	C.audio.muattac, C.audio.muchute, C.audio.muduree, C.audio.muflag = C.u16(m.Muattac), C.u16(m.Muchute), C.s16(m.Muduree), C.a32(m.Muflag)
	copy(unsafe.Slice((*int16)(unsafe.Pointer(&C.audio.muadresse[0])), len(m.Left)), m.Left)
	copy(unsafe.Slice((*int16)(unsafe.Pointer(&C.muright[0])), len(m.Right)), m.Right)
}

func GetMV2() *MV2 {
	m := &MV2{}
	for i := range m.Voices {
		c := &C.mv2a.voices[i]
		m.Voices[i] = Voice{uint16(c.freqsam), uint32(c.startsam1), uint32(c.longsam1), uint16(c.volsam), uint32(c.startsam2),
			uint32(c.longsam2), int16(c.value), uint8(c._type), uint8(c.delta), uint16(c.loopsam), uint16(c.frqto), uint32(c.sample)}
	}
	a := &C.mv2a
	m.Muptr, m.Mumax, m.Mucnt, m.Mubufa, m.Mubufc = uint16(a.muptr), uint16(a.mumax), uint16(a.mucnt), uint16(a.mubufa), uint16(a.mubufc)
	m.Muspeed, m.Muvolgen, m.Mubreak = uint16(a.muspeed), uint16(a.muvolgen), uint16(a.mubreak)
	m.Mupnote, m.Muvol, m.Mutempo, m.Mutemp = uint32(C.audio.mupnote), uint8(C.audio.muvol), uint8(C.audio.mutempo), uint8(C.audio.mutemp)
	m.Muattac, m.Muchute, m.Muduree, m.Muflag = uint16(C.audio.muattac), uint16(C.audio.muchute), int16(C.audio.muduree), int32(C.audio.muflag)
	n := int(C.audio.mutaloop)
	m.Left = append([]int16(nil), unsafe.Slice((*int16)(unsafe.Pointer(&C.audio.muadresse[0])), n)...)
	m.Right = append([]int16(nil), unsafe.Slice((*int16)(unsafe.Pointer(&C.muright[0])), n)...)
	return m
}

// MV2Prepare sets the player up as mv2_gomusic does for a sample song
// (frequency tables, mutaloop, stereo) without touching the song state.
func MV2Prepare() {
	C.cref_mv2_prepare()
}

func MV2Soundrout() { C.mv2_soundrout() }

// SetInst sets audio.tabinst[i].
func SetInst(i int, addr, unknown uint32) {
	C.audio.tabinst[i].address = C.u32(addr)
	C.audio.tabinst[i].unknown = C.u32(unknown)
}

// UnpackScript runs alis unpack_script on a packed script file; magic is its
// first header word, which selects the packer as in script_load.
func UnpackScript(path string, magic uint32, out []byte) int {
	p := C.CString(path)
	defer C.free(unsafe.Pointer(p))
	return int(C.cref_unpack_script(p, (*C.u8)(unsafe.Pointer(&out[0])), C.u32(magic)))
}

// UnpackOld runs alis unpack_old.
func UnpackOld(packed, out []byte, modpack int) {
	C.unpack_old((*C.u8)(unsafe.Pointer(&packed[0])), C.u32(len(packed)), (*C.u8)(unsafe.Pointer(&out[0])), C.u32(len(out)), C.s8(modpack))
}

// UnpackNew runs alis unpack_new into out[:size].
func UnpackNew(packed, out []byte, size int, dict []byte) {
	C.unpack_new((*C.u8)(unsafe.Pointer(&packed[0])), C.u32(len(packed)), (*C.u8)(unsafe.Pointer(&out[0])), C.u32(size), (*C.u8)(unsafe.Pointer(&dict[0])))
}

// Regs are the VM registers an opcode can change, with pointers as arena
// offsets.
type Regs struct {
	D6, D7                                     int32
	D5                                         uint16
	Acc, Bsd7, Bsd6, Bsd7bis, Sd7, Sd6, Oldsd7 uint32
	Zero, Neg                                  bool
	PC, VramOrg                                uint32
	VaccOff                                    int16
	Running                                    bool
	Fseq                                       uint8
	Saversp                                    int16
	Random                                     uint16
}

func GetRegs() Regs {
	var c C.cref_regs
	C.cref_get_regs(&c)
	return Regs{int32(c.d6), int32(c.d7), uint16(c.d5), uint32(c.acc), uint32(c.bsd7), uint32(c.bsd6), uint32(c.bsd7bis),
		uint32(c.sd7), uint32(c.sd6), uint32(c.oldsd7), c.zero != 0, c.neg != 0, uint32(c.pc), uint32(c.vram_org), int16(c.vacc_off), c.running != 0, uint8(c.fseq), int16(c.saversp), uint16(c.rnd)}
}

func SetRegs(r Regs) {
	b := func(x bool) C.u8 {
		if x {
			return 1
		}
		return 0
	}
	c := C.cref_regs{d6: C.s32(r.D6), d7: C.s32(r.D7), d5: C.u16(r.D5), acc: C.u32(r.Acc), bsd7: C.u32(r.Bsd7), bsd6: C.u32(r.Bsd6),
		bsd7bis: C.u32(r.Bsd7bis), sd7: C.u32(r.Sd7), sd6: C.u32(r.Sd6), oldsd7: C.u32(r.Oldsd7), zero: b(r.Zero), neg: b(r.Neg),
		pc: C.u32(r.PC), vram_org: C.u32(r.VramOrg), vacc_off: C.s16(r.VaccOff), running: b(r.Running), fseq: C.u8(r.Fseq), saversp: C.s16(r.Saversp), rnd: C.u16(r.Random)}
	C.cref_set_regs(&c)
}

// Step executes one opcode of the current script.
func Step() { C.cref_step() }

// Ent is the entity-test state (tstmov family, near/view lists).
type Ent struct {
	Wcx, Wcy, Wcz, Wforme, Matmask   int16
	Ptrent                           int32
	Tablent, Matent                  [256]int16
	Witmov, Fmitmov, Fallent, Tstinv uint8
	Goodmat                          uint16
}

func GetEnt() (e Ent) {
	var c C.cref_ent
	C.cref_get_ent(&c)
	e = Ent{Wcx: int16(c.wcx), Wcy: int16(c.wcy), Wcz: int16(c.wcz), Wforme: int16(c.wforme), Matmask: int16(c.matmask),
		Ptrent: int32(c.ptrent), Witmov: uint8(c.witmov), Fmitmov: uint8(c.fmitmov), Fallent: uint8(c.fallent), Tstinv: uint8(c.tstinv), Goodmat: uint16(c.goodmat)}
	for i := range 256 {
		e.Tablent[i], e.Matent[i] = int16(c.tablent[i]), int16(c.matent[i])
	}
	return
}

func SetEnt(e Ent) {
	c := C.cref_ent{wcx: C.s16(e.Wcx), wcy: C.s16(e.Wcy), wcz: C.s16(e.Wcz), wforme: C.s16(e.Wforme), matmask: C.s16(e.Matmask),
		ptrent: C.s32(e.Ptrent), witmov: C.u8(e.Witmov), fmitmov: C.u8(e.Fmitmov), fallent: C.u8(e.Fallent), tstinv: C.u8(e.Tstinv), goodmat: C.u16(e.Goodmat)}
	for i := range 256 {
		c.tablent[i], c.matent[i] = C.s16(e.Tablent[i]), C.s16(e.Matent[i])
	}
	C.cref_set_ent(&c)
}
