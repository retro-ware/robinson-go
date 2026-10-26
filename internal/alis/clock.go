package alis

import (
	"fmt"
	"os"
)

// Virtual time (the C harness model): time only advances while the VM
// sleeps. Every 17 ms the timer ISR runs, every 20 ms 882 audio frames are
// mixed and the display is sampled. The host paces this against real time.

const (
	tickUs       = 17000
	audioUs      = 20000
	sampleUs     = 20000
	eventTicksUs = 1000000 / 120
	frameTicksUs = 1000000 / 50
	audioFrames  = 882
)

type clock struct {
	now                          uint64
	nextTick, nextAudio, nextSmp uint64
	frameTime, loopTime          uint64
	argb                         []uint32
	pcm                          []int16
}

func (v *VM) initClock() {
	c := &v.clk
	c.nextTick, c.nextAudio, c.nextSmp = tickUs, audioUs, sampleUs
	c.argb = make([]uint32, gameWidth*gameHeight)
	c.pcm = make([]int16, audioFrames*2)
}

func (v *VM) initTimers() {
	v.clk.frameTime = v.clk.now
	v.clk.loopTime = v.clk.now
}

// Now is the virtual time in µs.
func (v *VM) Now() uint64 { return v.clk.now }

func (v *VM) pollInput() { v.host.PollInput(v.clk.now, &v.in) }

// sleep advances virtual time by us, running the timer, audio and display
// events that fall due.
func (v *VM) sleep(us uint64) {
	c := &v.clk
	target := c.now + us
	for {
		nxt := min(target, c.nextTick, c.nextAudio, c.nextSmp)
		c.now = nxt
		v.host.WaitUntil(c.now)
		v.pollInput()
		if v.in.Quit || v.quit.Load() {
			v.state = stateStopped
		}
		if c.now >= c.nextTick {
			v.itroutine()
			c.nextTick += tickUs
		}
		if c.now >= c.nextAudio {
			v.mixAudio(c.pcm)
			v.host.Audio(c.pcm)
			c.nextAudio += audioUs
		}
		if c.now >= c.nextSmp {
			v.compose(c.argb)
			v.host.Frame(c.now, c.argb, gameWidth, gameHeight)
			c.nextSmp += sampleUs
		}
		if c.now >= target {
			return
		}
	}
}

// sleepUntil sleeps until *start+len and restarts *start (sys_sleep_until).
func (v *VM) sleepUntil(start *uint64, length uint64) {
	if v.clk.now < *start+length {
		v.sleep(*start + length - v.clk.now)
	}
	*start = v.clk.now
}

func (v *VM) delayLoop()  { v.sleepUntil(&v.clk.loopTime, 4000) }
func (v *VM) delayFrame() { v.sleepUntil(&v.clk.frameTime, frameTicksUs*uint64(v.ctiming)) }

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
}

func (v *VM) setphysic() {
	v.img.fphysic = 1
}

func (v *VM) draw() {
	img := &v.img
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
