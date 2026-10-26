//go:build difftest

package alis

// Differential tests against the C reference (go test -tags difftest
// ./internal/alis; needs ref/alis/build/libalisref.a). Each test puts the same
// state into both VMs, runs one C function and its port, and compares.

import (
	"bytes"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"testing"

	"github.com/retro-ware/robinson-go/internal/alis/cref"
)

type nullHost struct{}

func (nullHost) WallUs() uint64                   { return 0 }
func (nullHost) PollInput(uint64, *Input)         {}
func (nullHost) WaitUntil(uint64)                 {}
func (nullHost) Frame(uint64, []uint32, int, int) {}
func (nullHost) Audio([]int16)                    {}
func (nullHost) Random() uint16                   { return 0 }
func (nullHost) Time() (int, int, int)            { return 12, 0, 0 }

// pair returns a fresh Go VM and the (process-wide) C VM, both with the game
// loaded and their arenas equal.
func pair(t *testing.T) *VM {
	t.Helper()
	data, _ := filepath.Abs(filepath.Join("..", "..", "orig", "cd"))
	if _, err := os.Stat(data); err != nil {
		t.Skip("game data not present in orig/cd")
	}
	save := t.TempDir()
	os.Setenv("ALIS_SAVEDIR", save)
	if err := cref.Init(data); err != nil {
		t.Fatal(err)
	}
	v, err := New(nullHost{}, NewFileSys(data, save))
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func diffMem(t *testing.T, what string, g, c []byte) {
	t.Helper()
	if bytes.Equal(g, c) {
		return
	}
	n := 0
	for i := range min(len(g), len(c)) {
		if g[i] != c[i] {
			if n < 8 {
				t.Errorf("%s: mem[%#x] go %#02x c %#02x", what, i, g[i], c[i])
			}
			n++
		}
	}
	t.Fatalf("%s: %d bytes differ (sizes %d, %d)", what, n, len(g), len(c))
}

func TestInitArenaEqual(t *testing.T) {
	v := pair(t)
	diffMem(t, "after init", v.Mem(), cref.Mem())
}

// scratch fills the same random bytes into both arenas below finmem and
// returns the region's start.
func scratch(v *VM, r *rand.Rand, n int) uint32 {
	at := v.finmem - uint32(n) - 0x100
	c := cref.Mem()
	for i := range n {
		b := byte(r.IntN(256))
		v.mem[int(at)+i], c[int(at)+i] = b, b
	}
	return at
}

func TestMixSamples(t *testing.T) {
	v := pair(t)
	r := rand.New(rand.NewPCG(1, 2))
	const area = 1 << 16
	base := scratch(v, r, area)
	pg, pc := make([]int16, audioFrames*2), make([]int16, audioFrames*2)
	for tc := range 3000 {
		cref.AudioReset()
		v.aud = audioState{fsound: 1}
		for i := range 4 {
			if r.IntN(3) == 0 {
				continue
			}
			length := uint32(1 + r.IntN(8000))
			ch := cref.Channel{
				Type:   chanSample,
				Addr:   base + uint32(r.IntN(area-int(length)-16)),
				Played: uint32(r.IntN(int(length) * 2)),
				Volume: int16(r.IntN(128)),
				Length: length,
				Freq:   int16(1 + r.IntN(32767)),
				Loop:   uint8(r.IntN(4)),
				Curson: int8(r.IntN(256)),
			}
			if r.IntN(4) == 0 {
				ch.Played = 0
			}
			cref.SetChannel(i, ch)
			v.aud.channels[i] = channel{typ: ch.Type, data: v.mem[ch.Addr:], addr: ch.Addr, played: ch.Played,
				volume: ch.Volume, length: ch.Length, freq: ch.Freq, loop: ch.Loop, curson: ch.Curson}
		}
		if r.IntN(2) == 0 {
			// film speech on channel 3
			for range 1 + r.IntN(4) {
				n := uint32(2 + r.IntN(6000))
				at := base + uint32(r.IntN(area-int(n)))
				f := int16(4000 + r.IntN(28000))
				got := v.fliAudioPush(v.mem[at:at+n], f)
				if want := cref.FliPush(at, n, f); uint32(got) != want {
					t.Fatalf("case %d: fli push accepted %d, C %d", tc, got, want)
				}
			}
		}
		for buf := range 4 {
			var was [4]bool
			for i := range 4 {
				was[i] = v.aud.channels[i].typ != chanNone
			}
			v.mixAudio(pg)
			cref.Mix(pc)
			for k := range pg {
				if pg[k] != pc[k] {
					t.Fatalf("case %d buffer %d: pcm[%d] go %d c %d (go ch %+v)", tc, buf, k, pg[k], pc[k], v.aud.channels)
				}
			}
			for i := range 4 {
				g, c := &v.aud.channels[i], cref.GetChannel(i)
				if g.typ != c.Type || g.typ != chanNone && (g.played != c.Played || g.loop != c.Loop || g.length != c.Length || g.freq != c.Freq) {
					t.Fatalf("case %d buffer %d channel %d: go {typ %d played %d loop %d len %d freq %d} c %+v",
						tc, buf, i, g.typ, g.played, g.loop, g.length, g.freq, c)
				}
				if was[i] && g.typ == chanNone && g.curson != c.Curson {
					t.Fatalf("case %d buffer %d channel %d: curson go %d c %d", tc, buf, i, g.curson, c.Curson)
				}
			}
		}
	}
}

func (m *mv2State) mirror(a *audioState) *cref.MV2 {
	c := &cref.MV2{Muptr: m.muptr, Mumax: m.mumax, Mucnt: m.mucnt, Mubufa: m.mubufa, Mubufc: m.mubufc,
		Muspeed: m.muspeed, Muvolgen: m.muvolgen, Mubreak: m.mubreak, Mupnote: a.mupnote, Muvol: a.muvol,
		Mutempo: a.mutempo, Mutemp: a.mutemp, Muattac: a.muattac, Muchute: a.muchute, Muduree: a.muduree,
		Muflag: a.muflag, Left: m.left[:m.mutaloop], Right: m.right[:m.mutaloop]}
	for i, v := range m.voices {
		c.Voices[i] = cref.Voice{v.freqsam, v.startsam1, v.longsam1, v.volsam, v.startsam2, v.longsam2,
			v.value, v.typ, v.delta, v.loopsam, v.frqto, v.sample}
	}
	return c
}

func TestMV2Player(t *testing.T) {
	v := pair(t)
	r := rand.New(rand.NewPCG(3, 4))
	const area = 0x60000
	base := scratch(v, r, area)
	mem := cref.Mem()
	put := func(at uint32, x uint32) { // LE u32 in both arenas
		for k := range 4 {
			v.mem[at+uint32(k)], mem[at+uint32(k)] = byte(x>>(8*k)), byte(x>>(8*k))
		}
	}
	// instruments: sample data follows a 0x10-byte header (type 2 = sample)
	var insts []uint32
	for i := range 32 {
		at := base + 0x48000 + uint32(i)*0x800
		smp := at + 0x10
		if r.IntN(5) != 0 {
			v.mem[at], mem[at] = 2, 2
			put(at+2, uint32(0x30+r.IntN(0x7c0))) // length incl. header
			put(at+0xc, uint32(r.IntN(0x800)))    // loop end
			put(at+8, uint32(r.IntN(0x400)))      // loop length
		}
		insts = append(insts, smp)
		unk := uint32(0)
		if r.IntN(8) == 0 {
			unk = uint32(r.IntN(8))
		}
		v.aud.tabinst[i] = audioInstrument{smp, unk}
		cref.SetInst(i, smp, unk)
	}
	cref.MV2Prepare()
	v.mv2.prevmufreq = 0
	v.mv2.calculfrq()
	v.aud.rout = routMV2
	for tc := range 2000 {
		m, a := &v.mv2, &v.aud
		a.mupnote = base + uint32(r.IntN(0x100))
		m.mumax = uint16(1 + r.IntN(0x40))
		m.muptr = uint16(r.IntN(int(m.mumax)))
		m.mucnt = uint16(r.IntN(0x41))
		m.muspeed = uint16(r.IntN(8))
		a.mutempo = uint8(r.IntN(256))
		a.mutemp = uint8(r.IntN(8))
		a.muattac = uint16(r.IntN(4))
		a.muchute = uint16(r.IntN(40))
		a.muduree = int16(r.IntN(6) - 1)
		m.mubufa, m.mubufc = uint16(1+r.IntN(40)), uint16(1+r.IntN(40))
		m.mubreak = uint16(r.IntN(4) / 3)
		m.muvolgen = uint16(r.IntN(0x41))
		a.muvol = uint8(r.IntN(0x41))
		a.muflag = 1
		for i := range m.voices {
			s := insts[r.IntN(len(insts))]
			m.voices[i] = mv2Voice{freqsam: uint16(0x71 + r.IntN(0x2e7+36)), sample: s - 0x10,
				startsam1: s + uint32(r.IntN(0x800)), longsam1: uint32(r.IntN(0x800)),
				startsam2: s + uint32(r.IntN(0x800)), longsam2: uint32(r.IntN(0x400)),
				volsam: uint16(r.IntN(0x41)), value: int16(r.IntN(0x400)), typ: uint8(r.IntN(16)),
				delta: uint8(r.IntN(256)), frqto: uint16(r.IntN(0x10000))}
			if r.IntN(10) == 0 {
				m.voices[i] = mv2Voice{}
			}
		}
		clear(m.left[:])
		clear(m.right[:])
		cref.SetMV2(m.mirror(a))
	ticks:
		for tick := range 6 {
			v.mv2Soundrout()
			cref.MV2Soundrout()
			for _, vc := range m.voices {
				k := int(int16(vc.freqsam))
				if k < 0 {
					k = -k >> 2
				}
				if k >= tabfrqLen+36 {
					break ticks // C reads past mv2a: undefined
				}
			}
			g, c := m.mirror(a), cref.GetMV2()
			for k := range g.Left {
				if g.Left[k] != c.Left[k] || g.Right[k] != c.Right[k] {
					t.Fatalf("case %d tick %d: frame %d go %d,%d c %d,%d\nvoices %+v", tc, tick, k, g.Left[k], g.Right[k], c.Left[k], c.Right[k], g.Voices)
				}
			}
			if fmt.Sprint(*g) != fmt.Sprint(*c) {
				t.Fatalf("case %d tick %d:\ngo %+v\nc  %+v", tc, tick, *g, *c)
			}
		}
	}
}
