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

	"github.com/retro-ware/robinson-go/internal/orig"
)

func main() {
	ms := flag.Float64("ms", 10000, "virtual time limit in ms (0 = none)")
	calls := flag.Bool("calls", false, "log every DOS and extender call")
	save := flag.String("save", "", "drive C: directory (default: a new temporary one with START.STP)")
	setup := flag.String("stp", "RBVVSBP1J0M1KQ", "START.STP contents for a new drive C:")
	pngDir := flag.String("png", "", "write the screen at every -every'th draw to this directory")
	every := flag.Int("every", 50, "draws between -png frames")
	frames := flag.String("frames", "", "log tick, screen and palette CRC at every draw (as rrqharness -frames)")
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
