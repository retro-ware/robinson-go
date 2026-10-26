package alis

import (
	"fmt"
	"os"
)

var accDebug = os.Getenv("ACCDBG") != ""
var palDebug = os.Getenv("PALDBG") != ""

// DOS palette model of the 32-bit driver (alis image.c dpal_*, family
// DPAL_B32): a 6-bit target (tpal) and a live palette (mpal); fades chase
// mpal toward tpal on the vblank tick.

const (
	// timeclockHz is the timer rate the pacing assumes. The ISR runs every
	// 17 ms, but alis computes the rate before it knows the platform is
	// little-endian and so assumes 50 Hz.
	// TODO(START.EXE): check the original's fade and film rates.
	timeclockHz = 50
	paceHz      = 50        // frames and fades run at the ST's 50 Hz
)

type dosPal struct {
	tpal, mpal, sav1, sav2 [768]uint8
	step, reload, skip     uint8
	counter                int8
	ftopal, lock           uint8
	acc                    uint32
}

func dpalExpand(c uint8) uint8 { c &= 0x3f; return c<<2 | c>>4 }

// dpalUpload is the DAC write: the live 6-bit palette becomes mpalet.
func (v *VM) dpalUpload() {
	d := &v.img.dpal
	for i := range 256 {
		v.img.mpalet[i] = 0xff000000 | uint32(dpalExpand(d.mpal[i*3]))<<16 |
			uint32(dpalExpand(d.mpal[i*3+1]))<<8 | uint32(dpalExpand(d.mpal[i*3+2]))
	}
}

func (v *VM) dpalChase() {
	d := &v.img.dpal
	d.ftopal = 0
	for i := range 768 {
		t, m := d.tpal[i], d.mpal[i]
		if t == m {
			continue
		}
		d.ftopal = 1
		if t > m {
			if int(m)+int(d.step) < int(t) {
				d.mpal[i] = m + d.step
			} else {
				d.mpal[i] = t
			}
		} else {
			if int(m)-int(d.step) > int(t) {
				d.mpal[i] = m - d.step
			} else {
				d.mpal[i] = t
			}
		}
	}
}

// dpalSteps sets the fade speed from a duration; 0 means an immediate set.
func (v *VM) dpalSteps(dur int16, inc int) bool {
	d := &v.img.dpal
	d.step = 1
	if dur == 0 {
		return false
	}
	if dur < 63 {
		cl := uint8(dur)
		if cl == 0 {
			return false // the original divides by zero here
		}
		d.step = 63/cl + 1
		d.reload = 1
		d.counter = 1
	} else {
		q := int32(dur)/63 + int32(inc)
		if q > 127 {
			q = 127
		}
		d.reload = uint8(q)
		d.counter = int8(q)
	}
	return true
}

func (v *VM) dpalApply(fade bool) {
	d := &v.img.dpal
	d.lock = 1
	if fade {
		v.dpalChase()
		d.ftopal = 1
	} else {
		d.counter, d.reload = 0, 0
		d.mpal = d.tpal
		d.ftopal = 0xff
	}
	d.lock = 0
}

func (v *VM) dpalLoad16(words uint32, first int) {
	d := &v.img.dpal
	for i := 0; i < 16 && first+i < 256; i++ {
		hi, lo := v.mem[words+uint32(i*2)], v.mem[words+uint32(i*2+1)]
		c := [3]uint8{(hi & 0x0f) << 3, (lo & 0xf0) >> 1, (lo & 0x0f) << 3}
		for k := range 3 {
			if c[k] != 0 {
				c[k] = ((c[k]>>3)<<5 | 0x1f) >> 2
			}
			d.tpal[(first+i)*3+k] = c[k]
		}
	}
}

func dpalDark(c8 uint8, dk uint16) uint8 {
	p := uint32(c8) * uint32(dk)
	if p > 0xffff {
		p = 0xffff
	}
	return uint8(p >> 10)
}

// dpalLoadDark applies RRQ's dark palette while loading 8-bit colours.
func (v *VM) dpalLoadDark(rgb8 []uint8, first, n int) {
	d := &v.img.dpal
	dk := &v.img.dkpalet
	for i := range n {
		e := first + i
		r, g, b := rgb8[i*3], rgb8[i*3+1], rgb8[i*3+2]
		if r == 0 && g == 0 && b == 0 && (uint16(dk[e*3]) > 0x100 || uint16(dk[e*3+1]) > 0x100 || uint16(dk[e*3+2]) > 0x100) {
			r, g, b = 2, 2, 2
		}
		d.tpal[e*3] = dpalDark(r, uint16(dk[e*3]))
		d.tpal[e*3+1] = dpalDark(g, uint16(dk[e*3+1]))
		d.tpal[e*3+2] = dpalDark(b, uint16(dk[e*3+2]))
	}
}

// dpalTopalette loads a 0xFE resource at res (0: keep the target).
func (v *VM) dpalTopalette(res uint32, dur int16) {
	d := &v.img.dpal
	fade := v.dpalSteps(dur, 1)
	if res != 0 {
		n, first := int(v.mem[res+1])+1, int(v.mem[res+2])
		if n == 1 {
			v.dpalLoad16(res+2, first)
		} else {
			if first+n > 256 {
				n = 256 - first
			}
			if v.img.fdarkpal != 0 {
				v.dpalLoadDark(v.mem[res+4:res+4+uint32(n*3)], first, n)
			} else {
				for k := range n * 3 {
					d.tpal[first*3+k] = v.mem[res+4+uint32(k)] >> 2
				}
			}
		}
	}
	v.dpalApply(fade)
	d.skip = 0
}

func (v *VM) dpalCtopalsav(idx, dur int16) {
	d := &v.img.dpal
	src := &d.sav2
	if idx == -1 {
		src = &d.sav1
	}
	if v.img.fdarkpal != 0 {
		var rgb8 [768]uint8
		for k := range rgb8 {
			rgb8[k] = src[k] << 2
		}
		v.dpalLoadDark(rgb8[:], 0, 256)
	} else {
		d.tpal = *src
	}
	v.dpalTopalette(0, dur)
}

func (v *VM) dosPalReset() { v.img.dpal = dosPal{} }

func (v *VM) dosCpalette(idx int16, res uint32) {
	d := &v.img.dpal
	if idx < 0 {
		if idx >= -2 {
			d.counter, d.reload = 0, 0
			v.dpalCtopalsav(idx, 0)
		}
	} else if d.skip == 0 && v.mem[res] == 0xfe {
		d.counter, d.reload = 0, 0
		v.dpalTopalette(res, 0)
	}
	d.skip = 0
}

func (v *VM) dosCtopalet(idx, dur int16, res uint32) {
	d := &v.img.dpal
	if d.skip == 0 {
		if idx < 0 {
			if idx >= -2 {
				v.dpalCtopalsav(idx, dur)
			}
		} else if v.mem[res] == 0xfe {
			v.dpalTopalette(res, dur)
		}
	}
	d.skip = 0
}

func (v *VM) dosCtoblack(dur int16) {
	d := &v.img.dpal
	fade := v.dpalSteps(dur, 0)
	d.tpal = [768]uint8{}
	d.lock = 1
	if !fade {
		d.counter, d.reload = 0, 0
		d.mpal = [768]uint8{}
	}
	v.dpalChase()
	d.ftopal = 1
	d.lock = 0
	d.skip = 0
}

func (v *VM) dosCselpalet(val int16) { v.img.dpal.skip = uint8(val) }

// dosCdefcolor does nothing in the 32-bit driver.
func (v *VM) dosCdefcolor(idx int16, val uint16) {}

func (v *VM) dosCsavepal(idx int16) {
	d := &v.img.dpal
	switch idx {
	case -1:
		d.sav1 = d.tpal
	case -2:
		d.sav2 = d.tpal
	}
}

// dosPutinPalette sets a 0xFE resource drawn with put (the original crashes).
func (v *VM) dosPutinPalette(res uint32) {
	d := &v.img.dpal
	d.counter, d.reload = 0, 0
	v.dpalTopalette(res, 0)
}

// dosFilmPalette writes a film colour chunk straight into the live palette.
func (v *VM) dosFilmPalette(addr []byte) {
	d := &v.img.dpal
	packets := uint16(addr[0]) | uint16(addr[1])<<8
	p := 2
	index := 0
	d.lock = 1
	for {
		index += int(addr[p])
		n := int(addr[p+1])
		p += 2
		if n == 0 {
			n = 256
		}
		if index+n > 256 {
			n = 256 - index
			packets = 1
		}
		for k := range n * 3 {
			d.mpal[index*3+k] = addr[p+k] & 0x3f
		}
		p += n * 3
		index += n
		packets--
		if packets == 0 {
			break
		}
	}
	d.mpal[0], d.mpal[1], d.mpal[2] = 0, 0, 0
	d.lock = 0
	v.dpalUpload()
}

func (v *VM) dpalTick() {
	d := &v.img.dpal
	if palDebug {
		fmt.Fprintf(os.Stderr, "P %d %d %d %d %d\n", v.clk.now/1000, d.counter, d.reload, d.ftopal, d.step)
	}
	if d.lock != 0 {
		return
	}
	v.dpalUpload()
	if d.ftopal == 0xff {
		d.ftopal = 0
		return
	}
	d.counter--
	if d.counter >= 0 {
		return
	}
	d.counter = int8(d.reload)
	v.dpalChase()
}

// dosPalTick runs from itroutine; fade steps follow the 50 Hz pace.
func (v *VM) dosPalTick() {
	d := &v.img.dpal
	if accDebug {
		fmt.Fprintf(os.Stderr, "A %d %d %d\n", v.clk.now/1000, d.acc, d.ftopal)
	}
	for d.acc += paceHz; d.acc >= timeclockHz; d.acc -= timeclockHz {
		if d.ftopal != 0 {
			v.dpalTick()
		}
	}
}
