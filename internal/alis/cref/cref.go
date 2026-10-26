//go:build difftest

// Package cref links the C reference (maestun/alis with tools/alis-ref.patch,
// built as ref/alis/build/libalisref.a) so tests can run its functions on the
// same state as the Go port.
package cref

/*
#cgo CFLAGS: -I${SRCDIR}/../../../ref/alis/src -I${SRCDIR}/../../../ref/alis/src/audio -I${SRCDIR}/../../../ref/alis/src/render -I${SRCDIR}/../../../ref/alis/src/video -I${SRCDIR}/../../../ref/alis/src/emu2149 -I${SRCDIR}/../../../ref/alis/src/emu8950 -DALIS_CONV_INPLACE=0
#cgo pkg-config: sdl2
#cgo LDFLAGS: ${SRCDIR}/../../../ref/alis/build/libalisref.a -lz -lm
#include <stdlib.h>
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
