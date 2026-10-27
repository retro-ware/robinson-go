package alis

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
		for _, code := range v.in.kb {
			v.kbdIRQ(code)
		}
		v.in.kb = v.in.kb[:0]
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

// waitEvent waits for the next timer tick or input event, as the
// original's loops that spin until an interrupt handler changes something.
func (v *VM) waitEvent() {
	t := v.clk.nextTick
	if ev, ok := v.host.(EventHost); ok {
		if at, ok := ev.NextEventUs(); ok && usToClk(at) < t {
			t = max(usToClk(at), v.clk.now)
		}
	}
	v.advanceTo(t)
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
	v.mouseSample()
	v.cursorTick()
	img.fitroutine = 0
}

// compose converts video memory to ARGB.
func (v *VM) compose(out []uint32) {
	for i, c := range v.Screen() {
		out[i] = v.img.mpalet[c]
	}
}

// Timeclock is the timer tick count (the original's 0x16058).
func (v *VM) Timeclock() uint32 { return v.timeclock }

// Ctiming is the frame length in ticks (cstiming).
func (v *VM) Ctiming() uint8 { return v.ctiming }

// Screen is video memory (8-bit pixels).
func (v *VM) Screen() []byte { return v.mem[vgaBase : vgaBase+vgaSize] }

// DAC is the palette as last uploaded to the VGA (6-bit r, g, b).
func (v *VM) DAC() []byte {
	b := make([]byte, 768)
	for i, c := range v.img.mpalet {
		b[i*3], b[i*3+1], b[i*3+2] = uint8(c>>16)>>2, uint8(c>>8)>>2, uint8(c)>>2
	}
	return b
}
