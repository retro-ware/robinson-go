//go:build orig

// Command origrun runs the original START.EXE in the emulated PC of
// internal/orig (go run -tags orig ./cmd/origrun).
package main

import (
	"bufio"
	"flag"
	"fmt"
	"hash/crc32"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/retro-ware/robinson-go/internal/alis"
	"github.com/retro-ware/robinson-go/internal/orig"
	"github.com/retro-ware/robinson-go/internal/script"
	"github.com/retro-ware/robinson-go/internal/vmstate"
)

func main() {
	ms := flag.Float64("ms", 10000, "virtual time limit in ms after the game timer starts (0 = none)")
	calls := flag.Bool("calls", false, "log every DOS and extender call")
	save := flag.String("save", "", "drive C: directory (default: a new temporary one with START.STP)")
	setup := flag.String("stp", "RBVVSBP1J0M1KQ", "START.STP contents for a new drive C:")
	pngDir := flag.String("png", "", "write the screen at every -every'th draw to this directory")
	every := flag.Int("every", 50, "draws between -png frames")
	frames := flag.String("frames", "", "log tick, screen and palette CRC at every draw (as rrqharness -frames)")
	input := flag.String("input", "", "input script (as rrqharness -input)")
	trace := flag.String("trace", "", "comma separated code addresses: log the registers there")
	peek := flag.String("peek", "", "comma separated data addresses: log their dwords with -trace")
	watch := flag.String("watch", "", "from,to: log writes to these data addresses")
	raw := flag.String("raw", "", "write the screen and palette at draw -rawdraw to this file")
	rawDraw := flag.Int("rawdraw", 0, "draw for -raw")
	vars := flag.String("vars", "", "write the scripts' state at draw -varsdraw to this file (as rrqharness -vars)")
	varsDraw := flag.Int("varsdraw", 0, "draw for -vars")
	flag.Parse()

	if *save == "" {
		d, err := os.MkdirTemp("", "origrun")
		check(err)
		defer os.RemoveAll(d)
		check(os.WriteFile(filepath.Join(d, "START.STP"), []byte(*setup), 0o644))
		*save = d
	}
	m, err := orig.New(orig.Config{
		Image: "orig/start32.bin",
		CD:    "orig/cd",
		Save:  *save,
		Log:   os.Stderr,
		Calls: *calls,
	})
	check(err)
	defer m.Close()
	var peeks []uint32
	for _, a := range strings.Split(*peek, ",") {
		if a != "" {
			addr, err := strconv.ParseUint(a, 0, 32)
			check(err)
			peeks = append(peeks, uint32(addr))
		}
	}
	for _, a := range strings.Split(*trace, ",") {
		if a == "" {
			continue
		}
		addr, err := strconv.ParseUint(a, 0, 32)
		check(err)
		check(m.Hook(uint32(addr), func() {
			fmt.Fprintf(os.Stderr, "trace tick %d %s", m.D32(0x16058), m.Regs())
			for _, p := range peeks {
				fmt.Fprintf(os.Stderr, " [%#x]=%#x", p, m.D32(p))
			}
			fmt.Fprintln(os.Stderr)
		}))
	}
	if *watch != "" {
		var from, to uint32
		_, err := fmt.Sscanf(*watch, "%v,%v", &from, &to)
		check(err)
		check(m.Watch(from, to, func(eip, addr uint32, size int, val uint32) {
			fmt.Fprintf(os.Stderr, "write tick %d eip %#x [%#x]=%#x/%d\n", m.D32(0x16058), eip, addr, val, size)
		}))
	}
	if *input != "" {
		evs, err := script.Read(*input)
		check(err)
		queue(m, evs)
	}
	draws := 0
	var fw *bufio.Writer
	if *frames != "" {
		f, err := os.Create(*frames)
		check(err)
		defer f.Close()
		fw = bufio.NewWriter(f)
		defer fw.Flush()
	}
	m.OnDraw = func() {
		scr := m.Screen()
		if *pngDir != "" && draws%*every == 0 {
			check(writePNG(filepath.Join(*pngDir, fmt.Sprintf("d%05d_t%07.0f.png", draws, m.Ms())), scr, m.DAC()))
		}
		if *raw != "" && draws == *rawDraw {
			check(os.WriteFile(*raw, append(append([]byte{}, scr...), m.DAC()...), 0o644))
		}
		if *vars != "" && draws == *varsDraw {
			f, err := os.Create(*vars)
			check(err)
			vmstate.Scripts(f, m.D8, m.D32(0x15f72))
			f.Close()
		}
		if fw != nil {
			fmt.Fprintf(fw, "draw %d tick %d ct %d scr %08x pal %08x\n", draws, m.D32(0x16058), m.D8(0x1605e),
				crc32.ChecksumIEEE(scr), crc32.ChecksumIEEE(m.DAC()))
		}
		draws++
	}
	err = m.Run(uint64(*ms * 1193182 / 1000))
	fmt.Fprintf(os.Stderr, "stopped: err=%v draws=%d config=%#x cdspeed=%d unknown ports=%#x\n",
		err, draws, m.D16(0x16062), m.D32(0x18abc), m.UnknownPorts())
}

// queue gives the machine the events of an input script at the times the
// alis package's clock gives them: the mouse driver's x runs 0..639, keys
// are their scancode bytes.
func queue(m *orig.Machine, evs []script.Event) {
	var buttons, x, y uint16
	for _, e := range evs {
		t := (e.Ms*1000*1193182 + 999999) / 1000000
		switch e.Op {
		case "key":
			sc := alis.ScancodeByName(e.A)
			if sc == 0 {
				fmt.Fprintf(os.Stderr, "unknown key %s\n", e.A)
				continue
			}
			for _, b := range alis.KeyBytes(sc, e.B != "down") {
				m.Key(t, b)
			}
		case "mouse":
			nx, _ := strconv.Atoi(e.A)
			ny, _ := strconv.Atoi(e.B)
			x, y = uint16(nx*2), uint16(ny)
			m.Mouse(t, buttons, x, y)
		case "lb", "rb":
			bit := uint16(1)
			if e.Op == "rb" {
				bit = 2
			}
			if e.A == "down" {
				buttons |= bit
			} else {
				buttons &^= bit
			}
			m.Mouse(t, buttons, x, y)
		}
	}
}

func writePNG(name string, vram, dac []byte) error {
	img := image.NewPaletted(image.Rect(0, 0, 320, 200), nil)
	for i := 0; i < 256; i++ {
		r, g, b := dac[i*3], dac[i*3+1], dac[i*3+2]
		img.Palette = append(img.Palette, color.RGBA{r<<2 | r>>4, g<<2 | g>>4, b<<2 | b>>4, 255})
	}
	copy(img.Pix, vram)
	f, err := os.Create(name)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

func check(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
