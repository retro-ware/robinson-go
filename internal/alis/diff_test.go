//go:build difftest

package alis

// Differential tests against the C reference (go test -tags difftest
// ./internal/alis; needs ref/alis/build/libalisref.a). Each test puts the same
// state into both VMs, runs one C function and its port, and compares.

import (
	"bytes"
	"math/rand/v2"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/retro-ware/robinson-go/internal/alis/cref"
)

type nullHost struct{}

func (nullHost) WallUs() uint64                   { return 0 }
func (nullHost) PollInput(uint64, *Input)         {}
func (nullHost) WaitUntil(uint64)                 {}
func (nullHost) Frame(uint64, []uint32, int, int) {}
func (nullHost) Audio([]int16)                    {}
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
	at := v.finmem() - uint32(n) - 0x100
	c := cref.Mem()
	for i := range n {
		b := byte(r.IntN(256))
		v.mem[int(at)+i], c[int(at)+i] = b, b
	}
	return at
}

func TestUnpackScripts(t *testing.T) {
	v := pair(t)
	data, _ := filepath.Abs(filepath.Join("..", "..", "orig", "cd"))
	names, _ := filepath.Glob(filepath.Join(data, "*.IO"))
	n := 0
	for _, path := range names {
		raw, _ := os.ReadFile(path)
		magic := le32(raw)
		if int8(magic>>24) >= 0 {
			continue
		}
		got, err := v.readScriptFile(filepath.Base(path))
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		want := make([]byte, int(magic&0xffffff)+0x10000)
		sz := cref.UnpackScript(path, magic, want)
		if sz != len(got) || !bytes.Equal(got, want[:sz]) {
			i := 0
			for i < min(len(got), sz) && got[i] == want[i] {
				i++
			}
			t.Errorf("%s: go %d bytes, C %d, first diff at %#x", filepath.Base(path), len(got), sz, i)
		}
		n++
	}
	t.Logf("%d scripts", n)
}

// TestUnpackRandom feeds random packed streams to both packers (the data
// files unpack_script_fp reads); the streams are padded with zeros because
// the packers may read past the packed size.
func TestUnpackRandom(t *testing.T) {
	r := rand.New(rand.NewPCG(7, 7))
	skipped := 0
	defer func() { t.Logf("%d of 1500 new-packer cases undefined", skipped) }()
	for c := range 3000 {
		n := 1 + r.IntN(600)
		size := 1 + r.IntN(2000)
		packed := make([]byte, n+0x10000)
		for i := range n {
			packed[i] = byte(r.IntN(256))
		}
		got := make([]byte, size+0x10000)
		want := make([]byte, size+0x10000)
		if c%2 == 0 {
			mp := []int{1, 2, 8}[r.IntN(3)]
			size = max(size, mp) // interleaved output needs one byte per plane
			unpackOld(packed[:n], got[:size], mp)
			cref.UnpackOld(packed[:n], want[:size], mp)
		} else {
			dict := make([]byte, packedDictionarySize)
			for i := range dict {
				dict[i] = byte(r.IntN(256))
			}
			// Random streams can run the new packer far past its output;
			// those are undefined in C, so only defined runs are compared.
			if !func() (ok bool) {
				defer func() { recover() }()
				unpackNew(packed[:n], got, size, dict)
				return true
			}() {
				skipped++
				continue
			}
			cref.UnpackNew(packed[:n], want, size, dict)
		}
		if !bytes.Equal(got, want) {
			i := 0
			for got[i] == want[i] {
				i++
			}
			t.Fatalf("case %d (size %d): first diff at %d", c, size, i)
		}
	}
}

// isolate reruns the calling test alone in a child process (the C VM is
// process-wide and other tests change it) and ends it with the child's result.
func isolate(t *testing.T) {
	t.Helper()
	if os.Getenv("CREF_CHILD") != "" {
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=^"+t.Name()+"$", "-test.v")
	cmd.Env = append(os.Environ(), "CREF_CHILD=1")
	out, err := cmd.CombinedOutput()
	for _, l := range strings.Split(string(out), "\n") {
		if strings.Contains(l, "diff_test.go") {
			t.Log(strings.TrimSpace(l))
		}
	}
	if err != nil {
		t.Fatalf("child: %v", err)
	}
	t.Skip("passed in a child process")
}

// runHost is the rrqharness host without logging: deterministic rand, noon,
// and a stop at maxUs.
type runHost struct {
	nullHost
	maxUs uint64
}

func (h *runHost) PollInput(us uint64, in *Input) {
	if us >= h.maxUs {
		in.Quit = true
	}
}

// runPair runs both VMs from start until ms of virtual time. It must be the
// last use of the C VM in the process, since the C state cannot be rewound.
func runPair(t *testing.T, ms uint64) *VM {
	t.Helper()
	isolate(t)
	data, _ := filepath.Abs(filepath.Join("..", "..", "orig", "cd"))
	if _, err := os.Stat(data); err != nil {
		t.Skip("game data not present in orig/cd")
	}
	save := t.TempDir()
	os.Setenv("ALIS_SAVEDIR", save)
	if err := cref.Init(data); err != nil {
		t.Fatal(err)
	}
	v, err := New(&runHost{maxUs: ms * 1000}, NewFileSys(data, save))
	if err != nil {
		t.Fatal(err)
	}
	v.Run()
	cref.Run(ms)
	return v
}

func TestRunArenaEqual(t *testing.T) {
	v := runPair(t, 8000)
	t.Logf("script %s at %d µs", v.script.name, v.Now())
	diffMem(t, "after 8 s", v.Mem(), cref.Mem())
}

func (v *VM) regs() cref.Regs {
	s := v.script
	return cref.Regs{D6: v.varD6, D7: v.varD7, D5: v.varD5, Acc: v.acc, Bsd7: v.bsd7, Bsd6: v.bsd6, Bsd7bis: v.bsd7bis,
		Sd7: v.sd7, Sd6: v.sd6, Oldsd7: v.oldsd7, Zero: v.srZero, Neg: v.srNeg,
		PC: s.pc, VramOrg: s.vramOrg, VaccOff: s.vaccOff, Running: s.running, Fseq: v.fseq, Saversp: v.saversp, Random: v.randomNumber}
}

func (v *VM) setRegs(r cref.Regs) {
	v.varD6, v.varD7, v.varD5, v.acc = r.D6, r.D7, r.D5, r.Acc
	v.bsd7, v.bsd6, v.bsd7bis, v.sd7, v.sd6, v.oldsd7 = r.Bsd7, r.Bsd6, r.Bsd7bis, r.Sd7, r.Sd6, r.Oldsd7
	v.srZero, v.srNeg = r.Zero, r.Neg
	v.script.pc, v.script.vaccOff, v.script.running = r.PC, r.VaccOff, r.Running
	v.fseq, v.saversp, v.randomNumber = r.Fseq, r.Saversp, r.Random
	v.state = stateRunning
}

func (v *VM) ent() cref.Ent {
	return cref.Ent{Wcx: v.wcx, Wcy: v.wcy, Wcz: v.wcz, Wforme: v.wforme, Matmask: v.matmask(), Ptrent: int32(v.ptrent),
		Tablent: v.tablent, Matent: v.matent, Witmov: v.witmov, Fmitmov: v.fmitmov, Fallent: v.fallent, Tstinv: v.tstinv, Goodmat: v.goodmat}
}

func (v *VM) setEnt(e cref.Ent) {
	v.wcx, v.wcy, v.wcz, v.wforme, v.ptrent = e.Wcx, e.Wcy, e.Wcz, e.Wforme, int(e.Ptrent)
	v.setMatmask(e.Matmask)
	v.tablent, v.matent, v.witmov, v.fmitmov, v.fallent, v.goodmat, v.tstinv = e.Tablent, e.Matent, e.Witmov, e.Fmitmov, e.Fallent, e.Goodmat, e.Tstinv
}

// fuzzOps are the opcodes of the expression engine and control flow:
// jumps, calls, branches, store/eval/arithmetic, loops and switches.
var fuzzOps = []byte{
	0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10, 0x11,
	0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18, 0x19, 0x1a, 0x1b, 0x1c, 0x1d,
	0x1e, 0x1f, 0x20, 0x21, 0x22, 0x23, 0x2b, 0x2c, 0x2d, 0x2e, 0x2f,
	// entity tests: ctstmov, ctstset, cftstmov, cftstset, csuccent, cpredent,
	// crstent, cvtstmov, cvftstmov, cwmov, cwtstmov, cwftstmov, ctstform, cavtstmov
	0x53, 0x54, 0x55, 0x56, 0x57, 0x58, 0x60, 0x8e, 0x8f, 0xa2, 0xa3, 0xa4, 0xa5, 0xf9,
	// cinitab, cpicture, cautomode, ccancel (file ops stay out: random names)
	0x00, 0x6f, 0x81, 0x88, 0x91, 0x92, 0xd9, 0xdb,
	// cnearent..cviewmat, cfindmat, cfindtyp, cnearcla, cviewcla
	0x59, 0x5a, 0x5b, 0x5c, 0x5d, 0x5e, 0x93, 0x94, 0xcb, 0xcc,
	// csczoom, cscsun, cscaset, cscamov
	0xe7, 0xed, 0xf2, 0xf3,
}

// TestOpcodeFuzz runs the game to a live state in both VMs, then from that
// same state executes one random instruction per case and compares the
// arena and registers. Cases the Go VM rejects (bad sub-opcode, out of
// range) are skipped, since C reads past its tables there.
func TestOpcodeFuzz(t *testing.T) {
	v := runPair(t, 8000)
	diffMem(t, "start", v.Mem(), cref.Mem())
	base := bytes.Clone(v.Mem())
	regs := v.regs()
	ent := v.ent()
	if c := cref.GetEnt(); c != ent {
		t.Fatalf("entity state differs at start:\ngo %+v\nc  %+v", ent, c)
	}
	if c := cref.GetRegs(); c != regs {
		t.Fatalf("registers differ at start:\ngo %+v\nc  %+v", regs, c)
	}
	cm := cref.Mem()
	t.Chdir(filepath.Join("..", "..", "orig", "cd")) // alis file paths are relative
	r := rand.New(rand.NewPCG(11, 11))
	code := make([]byte, 48)
	ran, skipped := 0, 0
	for c := range 20000 {
		copy(v.mem, base)
		op := fuzzOps[r.IntN(len(fuzzOps))]
		code[0] = op
		for i := 1; i < len(code); i++ {
			code[i] = byte(r.IntN(256))
		}
		copy(v.mem[regs.PC:], code)
		rr := regs
		rr.D6, rr.D7 = int32(r.Uint32()), int32(int16(r.Uint32()))
		v.setRegs(rr)
		ent.Tstinv = uint8(r.IntN(2))
		ent.Fallent = uint8(r.IntN(2))
		ent.Matmask = int16(r.Uint32())
		v.setEnt(ent)
		v.err = nil
		ok := func() (ok bool) {
			defer func() { recover() }()
			v.readexecOpcode()
			return v.err == nil
		}()
		if !ok {
			skipped++
			continue
		}
		copy(cm, base)
		copy(cm[regs.PC:], code)
		cref.SetRegs(rr)
		cref.SetEnt(ent)
		cref.Step()
		ran++
		if g, w := v.regs(), cref.GetRegs(); g != w {
			t.Fatalf("case %d op %#02x code % x (sd7 %q):\ngo %+v\nc  %+v", c, op, code, v.cstr(rr.Sd7), g, w)
		}
		if g, w := v.ent(), cref.GetEnt(); g != w {
			t.Fatalf("case %d op %#02x code % x: entity state\ngo %+v\nc  %+v", c, op, code, g, w)
		}
		if !bytes.Equal(v.mem, cm) {
			t.Logf("case %d op %#02x code % x", c, op, code)
			diffMem(t, "after op", v.mem, cm)
		}
	}
	t.Logf("%d cases ran, %d skipped", ran, skipped)
}
