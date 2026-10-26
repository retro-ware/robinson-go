package alis

import (
	"fmt"
	"os"
)

// Virtual time, in PIT input clocks as in START.EXE: the game timer (IRQ 0,
// PIT divisor 0x4da3, 60.035 Hz) runs itroutine; time only advances while
// the VM waits. Every 20 ms 882 audio frames are mixed and the display is
// sampled for the host. The host paces this against real time.

const (
	pitHz        = 1193182
	tickClocks   = 0x4da3 // game timer period (START.EXE 0x1e0d)
	audioUs      = 20000
	sampleUs     = 20000
	eventTicksUs = 1000000 / 120
	audioFrames  = 882
)

func clkToUs(c uint64) uint64  { return c * 1000000 / pitHz }
func usToClk(us uint64) uint64 { return (us*pitHz + 999999) / 1000000 }

type clock struct {
	now                uint64 // PIT clocks
	nextTick           uint64
	nAudio, nSmp       uint64 // 20 ms periods done
	nextAudio, nextSmp uint64
	argb               []uint32
	pcm                []int16
}

func (v *VM) initClock() {
	c := &v.clk
	c.nextTick = tickClocks
	c.nAudio, c.nSmp = 1, 1
	c.nextAudio, c.nextSmp = usToClk(audioUs), usToClk(sampleUs)
	c.argb = make([]uint32, gameWidth*gameHeight)
	c.pcm = make([]int16, audioFrames*2)
}

// EventHost is a Host with scripted input: time stops at each event so
// it applies at its exact virtual time.
type EventHost interface {
	NextEventUs() (uint64, bool)
}

// Now is the virtual time in µs.
func (v *VM) Now() uint64 { return clkToUs(v.clk.now) }

func (v *VM) pollInput() { v.host.PollInput(v.Now(), &v.in) }

// advanceTo advances virtual time to target (PIT clocks), running the
// timer, audio and display events that fall due.
func (v *VM) advanceTo(target uint64) {
	c := &v.clk
	for {
		nxt := min(target, c.nextTick, c.nextAudio, c.nextSmp)
		if ev, ok := v.host.(EventHost); ok {
			if at, ok := ev.NextEventUs(); ok && usToClk(at) < nxt {
				nxt = max(usToClk(at), c.now)
			}
		}
		c.now = nxt
		v.host.WaitUntil(v.Now())
		v.pollInput()
		if v.in.Quit || v.quit.Load() {
			v.state = stateStopped
		}
		if c.now >= c.nextTick {
			v.itroutine()
			c.nextTick += tickClocks
		}
		if c.now >= c.nextAudio {
			v.mixAudio(c.pcm)
			v.host.Audio(c.pcm)
			c.nAudio++
			c.nextAudio = usToClk(c.nAudio * audioUs)
		}
		if c.now >= c.nextSmp {
			v.compose(c.argb)
			v.host.Frame(v.Now(), c.argb, gameWidth, gameHeight)
			c.nSmp++
			c.nextSmp = usToClk(c.nSmp * sampleUs)
		}
		if c.now >= target {
			return
		}
	}
}

// startupTicks are the timer ticks START.EXE spends before the main
// script runs: the speed calibration waits for a first tick, clears the
// frame counter and counts for two more (0x1e81-0x1eb9), and the CD speed
// test reads for one more (0x32c7; the emulated reads take no time).
func (v *VM) startupTicks() {
	v.waitTick()
	v.img.vtiming = 0
	for range 3 {
		v.waitTick()
	}
}

// sleep advances virtual time by us.
func (v *VM) sleep(us uint64) { v.advanceTo(v.clk.now + usToClk(us)) }

// waitTick waits for the next game timer tick, as the original's wait
// loops spin until the timer handler has run.
func (v *VM) waitTick() { v.advanceTo(v.clk.nextTick) }

// delayFrame is the frame wait of draw (0x6340): until ctiming ticks have
// passed since the last draw.
func (v *VM) delayFrame() {
	for v.img.vtiming < v.ctiming && v.state != stateStopped {
		v.waitTick()
	}
}

func (v *VM) itroutine() {
	img := &v.img
	prev := img.vtiming
	v.timeclock++
	img.fitroutine = 1
	img.vtiming++
	if img.vtiming == 0 {
		img.vtiming = prev
	}
	v.dosPalTick()
	img.fitroutine = 0
}

// compose converts the shown screen to ARGB.
func (v *VM) compose(out []uint32) {
	for i, c := range v.img.physic {
		out[i] = v.img.mpalet[c]
	}
	if v.ShowCursor {
		v.drawCursor(out)
	}
}

// drawCursor overlays the cdefmouse sprite at the mouse position, on the
// output only (the DOS game draws it over the screen; the harnesses leave
// it out). Types 0x00 (4-bit, colour 0 clear), 0x10 (4-bit with palette
// base and clear colour) and 0x14 (8-bit, colour 0 clear).
func (v *VM) drawCursor(out []uint32) {
	m := v.in.Mouse
	d := v.desmouse
	if !m.Enabled || d == 0 {
		return
	}
	typ := v.mem[d]
	w := int(v.ru16(d+2)) + 1
	h := int(v.ru16(d+4)) + 1
	for y := range h {
		for x := range w {
			var c uint8
			var clear bool
			switch typ {
			case 0x00:
				b := v.mem[d+6+uint32(y*(w/2)+x/2)]
				c = b >> 4
				if x%2 == 1 {
					c = b & 0xf
				}
				clear = c == 0
			case 0x10:
				b := v.mem[d+8+uint32(y*(w/2)+x/2)]
				c = b >> 4
				if x%2 == 1 {
					c = b & 0xf
				}
				clear = c == v.mem[d+7]
				c += v.mem[d+6]
			case 0x14:
				c = v.mem[d+8+uint32(y*w+x)]
				clear = c == 0
			default:
				return
			}
			px, py := m.X+x, m.Y+y
			if clear || px >= gameWidth || py >= gameHeight {
				continue
			}
			out[py*gameWidth+px] = v.img.mpalet[c]
		}
	}
}

func (v *VM) setphysic() {
	v.img.fphysic = 1
}

func (v *VM) draw() {
	img := &v.img
	if v.OnDraw != nil {
		v.OnDraw()
	}
	v.delayFrame()
	if drawDebug {
		fmt.Fprintf(os.Stderr, "D %d %d %d\n", v.clk.now/1000, v.timeclock, v.ctiming)
	}
	if v.fswitch != 0 && img.fphytolog != 0 {
		img.fphytolog = 0
		v.phytolog()
	}
	img.vtiming = 0
	if v.fswitch != 0 {
		v.oldfen()
		img.ptabfen = 0
	}
	img.switchgo = 0
	v.setWlogic(false)
	oldphys := img.physic
	for sc := v.scr.ptscreen; sc != 0; sc = v.scrToNext(sc) {
		if v.sc8(sc, 0)&0x40 == 0 {
			v.affiscr(sc, v.scrScreenID(sc))
		}
	}
	img.fremap = 0
	if v.fswitch != 0 {
		img.physic = img.logic
		img.logic = oldphys
		v.setphysic()
	}
}

var drawDebug = os.Getenv("DRAWDBG") != ""

// Timeclock is the timer tick count (the original's 0x16058).
func (v *VM) Timeclock() uint32 { return v.timeclock }

// Ctiming is the frame length in ticks (cstiming).
func (v *VM) Ctiming() uint8 { return v.ctiming }

// Screen is the displayed screen (8-bit pixels).
func (v *VM) Screen() []byte { return v.img.physic }

// DAC is the palette as last uploaded to the VGA (6-bit r, g, b).
func (v *VM) DAC() []byte {
	b := make([]byte, 768)
	for i, c := range v.img.mpalet {
		b[i*3], b[i*3+1], b[i*3+2] = uint8(c>>16)>>2, uint8(c>>8)>>2, uint8(c)>>2
	}
	return b
}
