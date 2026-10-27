// Command rrqharness runs the VM in deterministic virtual time and logs
// a CRC of every changed display sample, in the format of the C reference
// harness (ref/alis/src/harness.c).
package main

import (
	"bufio"
	"cmp"
	"encoding/binary"
	"flag"
	"fmt"
	"hash/crc32"
	"image"
	"image/color"
	"image/png"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/retro-ware/robinson-go/internal/alis"
	"github.com/retro-ware/robinson-go/internal/script"
	"github.com/retro-ware/robinson-go/internal/vmstate"
)

type host struct {
	events    []script.Event
	evpos     int
	stop      bool
	vm        *alis.VM
	maxUs     uint64
	logf      *bufio.Writer
	memf      *bufio.Writer
	memEvery  uint32
	memDump   string
	memDumpAt int64
	dumpDir   string
	dumpAt    []uint64
	sample    uint32
	lastCRC   uint32
	buf       []byte
	wav       *os.File
	wavN      uint32
}

func (h *host) WallUs() uint64      { return 0 }
func (h *host) WaitUntil(us uint64) {}
func (h *host) NextEventUs() (uint64, bool) {
	if h.evpos < len(h.events) {
		return h.events[h.evpos].Ms * 1000, true
	}
	return 0, false
}

func (h *host) PollInput(us uint64, in *alis.Input) {
	for h.evpos < len(h.events) && h.events[h.evpos].Ms*1000 <= us {
		h.apply(h.events[h.evpos], in)
		h.evpos++
	}
	if h.stop || h.maxUs != 0 && us >= h.maxUs {
		in.Quit = true
	}
}

func (h *host) apply(e script.Event, in *alis.Input) {
	switch e.Op {
	case "key":
		sc := alis.ScancodeByName(e.A)
		if sc == 0 {
			log.Printf("unknown key %s", e.A)
			return
		}
		if e.B == "down" {
			in.KeyDown(sc)
		} else {
			in.KeyUp(sc)
		}
	case "mouse":
		in.Mouse.X, _ = strconv.Atoi(e.A)
		in.Mouse.Y, _ = strconv.Atoi(e.B)
	case "lb", "rb":
		down := e.A == "down"
		if e.Op == "lb" {
			in.Mouse.LB = down
		} else {
			in.Mouse.RB = down
		}
	case "quit":
		h.stop = true
	}
}

func (h *host) Audio(s []int16) {
	if h.wav == nil {
		return
	}
	b := make([]byte, len(s)*2)
	for i, x := range s {
		binary.LittleEndian.PutUint16(b[i*2:], uint16(x))
	}
	h.wav.Write(b)
	h.wavN += uint32(len(b))
}

// wavHeader writes the 44-byte header of a 44100 Hz s16 stereo WAV, as
// the C harness's ALIS_WAV.
func wavHeader(f *os.File, n uint32) {
	h := make([]byte, 44)
	copy(h, "RIFF")
	binary.LittleEndian.PutUint32(h[4:], 36+n)
	copy(h[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(h[16:], 16)
	binary.LittleEndian.PutUint16(h[20:], 1)
	binary.LittleEndian.PutUint16(h[22:], 2)
	binary.LittleEndian.PutUint32(h[24:], 44100)
	binary.LittleEndian.PutUint32(h[28:], 44100*4)
	binary.LittleEndian.PutUint16(h[32:], 4)
	binary.LittleEndian.PutUint16(h[34:], 16)
	copy(h[36:], "data")
	binary.LittleEndian.PutUint32(h[40:], n)
	f.WriteAt(h, 0)
}
func (h *host) Time() (int, int, int) { return 12, 0, 0 }

func (h *host) Frame(us uint64, argb []uint32, w, ht int) {
	if len(h.buf) != len(argb)*4 {
		h.buf = make([]byte, len(argb)*4)
	}
	for i, p := range argb {
		binary.LittleEndian.PutUint32(h.buf[i*4:], p)
	}
	crc := crc32.ChecksumIEEE(h.buf)
	ms := us / 1000
	if int64(h.sample) == h.memDumpAt {
		os.WriteFile(h.memDump, h.vm.Mem(), 0o644)
	}
	if h.memf != nil && h.sample%h.memEvery == 0 {
		fmt.Fprintf(h.memf, "%d %d %08x\n", h.sample, ms, crc32.ChecksumIEEE(h.vm.Mem()))
	}
	if crc != h.lastCRC {
		h.lastCRC = crc
		if h.logf != nil {
			fmt.Fprintf(h.logf, "%d %d %08x\n", h.sample, ms, crc)
		}
	}
	for _, at := range h.dumpAt {
		if at >= ms && at < ms+20 && h.dumpDir != "" {
			img := image.NewRGBA(image.Rect(0, 0, w, ht))
			for i, p := range argb {
				img.Set(i%w, i/w, color.RGBA{uint8(p >> 16), uint8(p >> 8), uint8(p), 0xff})
			}
			f, err := os.Create(filepath.Join(h.dumpDir, fmt.Sprintf("%06d.png", h.sample)))
			if err == nil {
				png.Encode(f, img)
				f.Close()
			}
		}
	}
	h.sample++
}

func main() {
	data := flag.String("data", "orig/cd", "game data directory")
	maxMs := flag.Uint64("maxms", 20000, "stop at this virtual time (ms)")
	logPath := flag.String("log", "", "CRC log file")
	memDump := flag.String("memdump", "", "write the VM memory at sample -memdumpat to this file")
	memDumpAt := flag.Int64("memdumpat", -1, "sample index for -memdump")
	memEvery := flag.Int("memevery", 1, "log the memory CRC every n-th sample")
	memPath := flag.String("memlog", "", "CRC of the VM memory every sample (C harness ALIS_MEMLOG)")
	dump := flag.String("dump", "", "PNG dump directory")
	dumpAt := flag.String("dumpat", "", "comma separated ms to dump")
	input := flag.String("input", "", "input script (C harness ALIS_INPUT format)")
	wav := flag.String("wav", "", "write the mixed audio as WAV (C harness ALIS_WAV)")
	save := flag.String("save", "", "directory for files the game writes (C harness ALIS_SAVEDIR; default: the data directory)")
	sbLog := flag.String("sblog", "", "log the end tick and CRC of every Sound Blaster block (as origrun -sblog)")
	frames := flag.String("frames", "", "log tick, screen and palette CRC at every draw (as origrun -frames)")
	raw := flag.String("raw", "", "write the screen and palette at draw -rawdraw to this file (as origrun -raw)")
	rawDraw := flag.Int("rawdraw", 0, "draw for -raw")
	vars := flag.String("vars", "", "write the scripts' state at draw -varsdraw to this file (as origrun -vars)")
	varsDraw := flag.Int("varsdraw", 0, "draw for -vars")
	dsDump := flag.String("dsdump", "", "write the data segment at draw -varsdraw to this file (as origrun -dsdump)")
	flag.Parse()

	h := &host{maxUs: *maxMs * 1000, lastCRC: 0xffffffff, dumpDir: *dump}
	if *logPath != "" {
		f, err := os.Create(*logPath)
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()
		h.logf = bufio.NewWriter(f)
		defer h.logf.Flush()
	}
	if *memPath != "" {
		f, err := os.Create(*memPath)
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()
		h.memf = bufio.NewWriter(f)
		h.memEvery = uint32(max(1, *memEvery))
		defer h.memf.Flush()
	}
	if *wav != "" {
		f, err := os.Create(*wav)
		if err != nil {
			log.Fatal(err)
		}
		wavHeader(f, 0)
		f.Seek(44, 0)
		h.wav = f
		defer func() {
			wavHeader(f, h.wavN)
			f.Close()
		}()
	}
	for _, s := range strings.Split(*dumpAt, ",") {
		if n, err := strconv.ParseUint(s, 10, 64); err == nil {
			h.dumpAt = append(h.dumpAt, n)
		}
	}
	if *input != "" {
		evs, err := script.Read(*input)
		if err != nil {
			log.Fatal(err)
		}
		h.events = evs
	}
	h.memDump, h.memDumpAt = *memDump, *memDumpAt
	vm, err := alis.New(h, alis.NewFileSys(*data, cmp.Or(*save, *data)))
	if err != nil {
		log.Fatal(err)
	}
	h.vm = vm
	var fw *bufio.Writer
	if *frames != "" {
		f, err := os.Create(*frames)
		if err != nil {
			log.Fatal(err)
		}
		fw = bufio.NewWriter(f)
		defer f.Close()
		defer fw.Flush()
	}
	if *sbLog != "" {
		f, err := os.Create(*sbLog)
		if err != nil {
			log.Fatal(err)
		}
		w := bufio.NewWriter(f)
		defer f.Close()
		defer w.Flush()
		i := 0
		vm.SBLog(func(end uint64, b []byte) {
			fmt.Fprintf(w, "block %d t %d rate %d n %d crc %08x\n", i, end, 15151, len(b), crc32.ChecksumIEEE(b))
			i++
		})
	}
	n := 0
	vm.OnDraw = func() {
		if *raw != "" && n == *rawDraw {
			if err := os.WriteFile(*raw, append(append([]byte{}, vm.Screen()...), vm.DAC()...), 0o644); err != nil {
				log.Fatal(err)
			}
		}
		if *dsDump != "" && n == *varsDraw {
			if err := os.WriteFile(*dsDump, vm.Mem(), 0o644); err != nil {
				log.Fatal(err)
			}
		}
		if *vars != "" && n == *varsDraw {
			f, err := os.Create(*vars)
			if err != nil {
				log.Fatal(err)
			}
			mem := vm.Mem()
			vmstate.Scripts(f, func(a uint32) byte { return mem[a] }, vm.Atent())
			f.Close()
		}
		if fw != nil {
			fmt.Fprintf(fw, "draw %d tick %d ct %d scr %08x pal %08x\n", n, vm.Timeclock(), vm.Ctiming(),
				crc32.ChecksumIEEE(vm.Screen()), crc32.ChecksumIEEE(vm.DAC()))
		}
		n++
	}
	if err := vm.Run(); err != nil {
		log.Print(err)
	}
}
