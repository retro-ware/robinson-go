// Command rrqharness runs the VM in deterministic virtual time and logs
// a CRC of every changed display sample, in the format of the C reference
// harness (ref/alis/src/harness.c).
package main

import (
	"bufio"
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
)

type host struct {
	vm      *alis.VM
	maxUs   uint64
	logf    *bufio.Writer
	dumpDir string
	dumpAt  []uint64
	sample  uint32
	lastCRC uint32
	rnd     uint32
	buf     []byte
}

func (h *host) WallUs() uint64      { return 0 }
func (h *host) WaitUntil(us uint64) {}
func (h *host) PollInput(us uint64, in *alis.Input) {
	if h.maxUs != 0 && us >= h.maxUs {
		in.Quit = true
	}
}
func (h *host) Audio(s []int16) {}
func (h *host) Random() uint16 {
	h.rnd = h.rnd*1103515245 + 12345
	return uint16(h.rnd >> 16 & 0x7fff)
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
	dump := flag.String("dump", "", "PNG dump directory")
	dumpAt := flag.String("dumpat", "", "comma separated ms to dump")
	flag.Parse()

	h := &host{maxUs: *maxMs * 1000, lastCRC: 0xffffffff, rnd: 1, dumpDir: *dump}
	if *logPath != "" {
		f, err := os.Create(*logPath)
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()
		h.logf = bufio.NewWriter(f)
		defer h.logf.Flush()
	}
	for _, s := range strings.Split(*dumpAt, ",") {
		if n, err := strconv.ParseUint(s, 10, 64); err == nil {
			h.dumpAt = append(h.dumpAt, n)
		}
	}
	vm, err := alis.New(h, alis.NewFileSys(*data, *data))
	if err != nil {
		log.Fatal(err)
	}
	h.vm = vm
	if err := vm.Run(); err != nil {
		log.Print(err)
	}
}
