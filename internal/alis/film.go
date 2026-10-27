package alis

import (
	"fmt"
	"io"
	"os"
)

var fliDebug = os.Getenv("FLIDBG") != ""

// FLI films (START.EXE 0x13c2f..0x144a0, opcodes 0x8e5b copenfilm, 0x8ed0
// crunfilm, 0x8ef9 cdelfilm; the state is the struct 0x17944). A film plays
// from buf: the whole film file (START.EXE streams it through a buffer), or
// VM memory for cfilm. Frames are decoded straight into the 320x200 screen
// 0x178cb, at its origin.

// Film flags (the struct's first word).
const (
	filmOverlay = 1 << 0 // brun skips colour 0, no black chunk, palette through the dark table
	filmPal160  = 1 << 1 // a 256-colour palette packet sets 160 colours
	filmLogic   = 1 << 2 // decode into the logic screen and copy it to physic per frame
	filmClear   = 1 << 3 // the header clears the palette and the screen
)

type film struct {
	id          int16 // flags; bits 8..15 enable the audio chunks 0x1000+n
	playing     bool
	fh          File   // the streamed file (+0x4e)
	addr1       uint32 // the next chunk (+0x1c)
	addr2       uint32 // cfilm: the film's first chunk (+0x18)
	end         uint32 // the end of the last chunk (+0x20)
	size        uint32 // the buffer's size (+0x24)
	base        uint32 // the buffer (+0x28)
	bend        uint32 // the buffer's end (+0x2c)
	wpos        uint32 // where the next read goes (+0x30)
	state       uint32 // the refill state (+0x34)
	budget      uint32 // bytes read per frame (+0x38)
	first       uint32 // bytes read at the start (+0x3c)
	cross       uint32 // the chunk across the buffer's end (+0x40)
	bufh        int32  // the buffer's memory handle (+0x50)
	frame       int16
	frames      int16
	result      int16
	batchframes int16
	waitclock   int16
	basemain    uint32
	palPending  bool   // 0x1c2de
	deadline    uint32 // timeclock of the next frame
	ch          uint32 // the speech's mixer channel (+0x58)
	speech      int    // the buffer of the last speech chunk (+0x5c)
}

func le16b(b []byte) uint16 { return uint16(b[0]) | uint16(b[1])<<8 }

// dsCDSpeed (0x18abc) is the CD's speed in bytes per second.
const dsCDSpeed = 0x18abc

func cdelfilm(v *VM) { v.delfilm() }

// delfilm is 0x13d53.
func (v *VM) delfilm() {
	f := &v.film
	f.addr1 = 0 // 0x13f08 with -1
	v.filmPalFlush()
	if f.bufh != 0 {
		v.ioMfree(f.bufh)
		f.bufh, f.base = 0, 0
	}
	if f.fh != nil {
		f.fh.Close()
		f.fh = nil
	}
}

// copenfilm is 0x8e5b: flags, file name, buffer size in thousands of
// bytes, frame time in ticks.
func copenfilm(v *VM) {
	if v.film.fh != nil {
		v.delfilm()
	}
	f := &v.film
	v.readexecOpername()
	f.id = int16(v.varD7)
	v.readexecOpernameSwap()
	fh, err := v.fs.Open(v.cstr(v.sd7), 0)
	if err != nil {
		v.fatal("film: can't open " + v.cstr(v.sd7))
		return
	}
	f.fh = fh
	v.readexecOpername()
	f.size = 1000 * uint32(uint16(v.varD7))
	v.readexecOpername()
	f.waitclock = int16(v.varD7)
	f.basemain = v.basemain
	v.inifilm()
	v.varD7 = int32(f.frames)
	v.cstoreContinue()
}

// filmRead is the DOS read 0x30ca into VM memory.
func (v *VM) filmRead(dst, n uint32) int {
	if int32(n) <= 0 {
		return 0
	}
	r, _ := io.ReadFull(v.film.fh, v.mem[dst:dst+n])
	return r
}

// filmTarget points 0x178cb at the screen the film draws into.
func (v *VM) filmTarget() {
	scr := uint32(dsPhysic)
	if v.film.id&filmLogic != 0 {
		scr = dsLogic
	}
	v.w32(dsWlogic, int32(v.ru32(scr)))
	v.w16(dsWloglarg, v.r16(dsLoglarg))
	v.w16(dsWlogx1, 0)
	v.w16(dsWlogy1, 0)
}

// inifilm is 0x13c2f.
func (v *VM) inifilm() {
	f := &v.film
	f.playing = false
	f.bufh, f.base = 0, 0
	if f.fh != nil {
		f.size = max(f.size, 0x23a50)
		f.budget = uint32(uint64(v.ru32(dsCDSpeed)) * uint64(uint16(f.waitclock)) / tickClocks)
		f.budget -= f.budget >> 3
		f.first = max(f.size>>2, 0x11d28)
		n := f.size
		if uint16(f.id)&0xff00 != 0 {
			n += 0x3e80
		}
		f.bufh = v.ioMalloc(int32(n))
		if f.bufh == 0 {
			v.fatal("film: no memory for the buffer")
			return
		}
		f.base = v.ru32(uint32(f.bufh))
		f.addr1 = f.base
		if uint16(f.id)&0xff00 != 0 { // 0x13cd9
			f.ch = v.sbFilmChan()
		}
	}
	v.filmTarget()
	f.palPending = false
	if f.fh != nil {
		v.filmOpen()
	}
	v.filmStep()
	f.deadline = v.timeclock
}

// filmOpen is 0x13f08 with the file's handle: the first read fills the
// start of the buffer.
func (v *VM) filmOpen() {
	f := &v.film
	n := v.filmRead(f.addr1, f.first)
	f.base = f.addr1
	f.wpos = f.base + uint32(n)
	f.bend = f.base + f.size
	f.state = 0
}

// runfilm is 0x13da8.
func (v *VM) runfilm() {
	f := &v.film
	img := &v.img
	bm := f.basemain
	x, w := v.r16(bm+0xe), v.r16(bm+0x12)
	y, h := v.r16(bm+0x10), v.r16(bm+0x14)
	v.w16(dsBlocx1, x)
	v.w16(dsFenx1, x)
	v.w16(dsFenl, w+1)
	v.w16(dsBlocx2, x+w)
	v.w16(dsFenx2, x+w)
	v.w16(dsBlocy1, y)
	v.w16(dsFeny1, y)
	v.w16(dsFenh, h+1)
	v.w16(dsBlocy2, y+h)
	v.w16(dsFeny2, y+h)
	img.fenx1, img.fenx2, img.clipl = x, x+w, w+1
	img.feny1, img.feny2, img.cliph = y, y+h, h+1
	img.wloglarg = img.loglarg
	img.wlogx1, img.wlogy1 = 0, 0
	img.clipx1, img.clipy1 = img.fenx1, img.feny1
	img.clipx2, img.clipy2 = img.fenx2, img.feny2
	v.filmTarget()
	f.playing = true
	for {
		// wait for the frame's tick (0x13e4b)
		if f.waitclock > 0 {
			for f.deadline > v.timeclock && v.state != stateStopped {
				v.waitTick()
			}
		}
		f.deadline = v.timeclock + uint32(uint16(f.waitclock))
		ok := v.filmStep()
		if f.id&filmLogic != 0 {
			copy(v.mem[v.ru32(dsPhysic):][:gameWidth*gameHeight], v.mem[v.ru32(dsLogic):])
		}
		if fliDebug {
			fmt.Fprintf(os.Stderr, "F %d %d tc=%d\n", v.Now()/1000, f.frame, v.timeclock)
		}
		if !ok {
			break
		}
		f.batchframes--
		if f.batchframes == 0 {
			f.result = 0
			return
		}
		if f.batchframes < 0 {
			f.batchframes++
		}
	}
	if f.fh != nil {
		f.fh.Seek(0, io.SeekStart) // 0x3095; the buffer stays as it is
	} else {
		f.addr1 = f.addr2
	}
	f.result = -0x7ffc
}

// filmStep is 0x13f08 (eax 0): the header or the next frame; false at the
// end of the film.
func (v *VM) filmStep() bool {
	f := &v.film
	ok := false
	if p := f.addr1; p != 0 {
		switch v.ru16(p + 4) {
		case 0xaf11: // 0x1412c
			if f.id&filmClear != 0 {
				clear(v.img.dpal.mpal[:])
				v.dpalUpload()
				clear(v.filmScreen())
			}
			f.end = p + v.ru32(p)
			f.addr1 = p + 0x80
			f.frames = int16(v.ru16(p + 6))
			f.frame = 0
			v.filmRefill()
			ok = true
		case 0xf1fa:
			v.filmFrame()
			v.filmRefill()
			ok = true
		}
	}
	v.filmPalFlush()
	return ok
}

// filmRefill is 0x13fd0: reads at most budget bytes into the buffer. When
// the file reaches the buffer's end, the chunk across it moves to the
// start, the reads fill up to the next chunk, and the next chunk goes to
// the start once played up to there.
func (v *VM) filmRefill() {
	f := &v.film
	if f.fh == nil || int32(f.end) <= int32(f.base) {
		return
	}
	switch f.state {
	case 0:
		n := min(int32(f.bend-f.wpos), int32(f.budget))
		if n != 0 {
			r := v.filmRead(f.wpos, uint32(n))
			if r <= 0 {
				return
			}
			f.wpos += uint32(r)
			if int32(f.wpos) < int32(f.bend) {
				break
			}
		}
		p := f.addr1 // 0x1402d
		var q uint32
		for {
			q = p
			p += v.ru32(p)
			if int32(p) >= int32(f.bend) {
				break
			}
		}
		if p == f.bend {
			break
		}
		f.state, f.cross = 1, q
		fallthrough
	case 1:
		n := f.bend - f.cross
		if int32(n) > int32(f.addr1-f.base) {
			break
		}
		copy(v.mem[f.base:f.base+n], v.mem[f.cross:])
		f.wpos = f.base + n
		f.state = 2
	case 2:
		n := min(int32(f.addr1-f.wpos), int32(f.budget))
		r := v.filmRead(f.wpos, uint32(n))
		if r <= 0 {
			return
		}
		f.wpos += uint32(r)
	default:
		return
	}
	// 0x140ae
	if !(int32(f.addr1) < int32(f.bend) && int32(f.addr1+v.ru32(f.addr1)) <= int32(f.bend)) {
		d := f.addr1 - f.base
		f.addr1 = f.base
		f.end -= d
		f.state = 0
	}
	if int32(f.addr1) >= int32(f.wpos) {
		return
	}
	e := f.addr1 + v.ru32(f.addr1)
	if int32(e) <= int32(f.wpos) {
		return
	}
	if int32(e) < int32(f.bend) {
		n := f.wpos - f.addr1
		for i := range n { // rep movsb
			v.mem[f.base+i] = v.mem[f.addr1+i]
		}
		f.wpos = f.base + n
		f.addr1 = f.base
		e = f.base + v.ru32(f.base)
	}
	if r := v.filmRead(f.wpos, e-f.wpos); r > 0 {
		f.wpos += uint32(r)
	}
}

// filmPalFlush is 0x1433b: the palette goes out at the next retrace.
func (v *VM) filmPalFlush() {
	if v.film.palPending {
		v.waitRetrace()
		v.dpalUpload()
		v.film.palPending = false
	}
}

func (v *VM) filmScreen() []byte {
	return v.mem[v.ru32(dsWlogic):][:gameWidth*gameHeight]
}

// filmFrame is 0x14164 (it clears 0x64, see filmAudio). A chunk at or
// past the frame's end ends the loop without advancing, as in the original.
func (v *VM) filmFrame() {
	f := &v.film
	end := f.addr1 + v.ru32(f.addr1)
	n := int(v.ru16(f.addr1 + 6))
	p := f.addr1 + 16
	for range n {
		if p >= end {
			continue
		}
		typ := v.ru16(p + 4)
		next := p + v.ru32(p)
		b := v.mem[p+6 : max(next, p+6)]
		switch {
		case typ == 0xb:
			v.filmPalette(b)
		case typ == 0xc:
			v.filmDelta(b)
		case typ == 0xd:
			if f.id&filmOverlay == 0 {
				clear(v.filmScreen())
			}
		case typ == 0xf:
			v.filmBrun(b)
		case typ == 0x10: // 0x14490 copies 64000 bytes whatever the chunk's size
			copy(v.filmScreen(), v.mem[p+6:])
		case typ >= 0x1000 && typ < 0x1008:
			v.filmAudio(b, typ)
		}
		p = next
	}
	f.addr1 = end
	f.frame++
}

// filmPalette is 0x14270: packets of (first colour, count) with 6-bit
// colours. With filmOverlay and the dark table on (0x178fb), the colours
// go through the table (0x8adf) after a shift to 8 bits done in the film
// data, and every packet reads the same bytes (the original restores esi).
func (v *VM) filmPalette(b []byte) {
	f := &v.film
	d := &v.img.dpal
	packets := le16b(b)
	p := 2
	if f.id&filmOverlay != 0 && v.img.fdarkpal != 0 {
		for ; packets != 0; packets-- {
			first, n := int(b[p]), int(b[p+1])
			if n == 0 {
				n = 256
				if f.id&filmPal160 != 0 {
					n = 160
				}
			}
			n = min(n, 256-first)
			rgb := b[p+2 : p+2+n*3]
			for i := range rgb {
				rgb[i] <<= 2
			}
			v.dpalDarkInto(d.mpal[:], rgb, first, n)
		}
	} else {
		for ; packets != 0; packets-- {
			first, n := int(b[p]), int(b[p+1])
			p += 2
			if n == 0 {
				n = 256
				if f.id&filmPal160 != 0 {
					n = 160
				}
			}
			copy(d.mpal[first*3:], b[p:p+n*3])
			p += n * 3
		}
	}
	d.mpal[0], d.mpal[1], d.mpal[2] = 0, 0, 0
	f.palPending = true
	if f.id&filmOverlay == 0 {
		v.filmPalFlush()
	}
}

// filmDelta is 0x14358: from line (word), a byte count of lines (0: 256)
// of packets (skip, signed count: > 0 copies, else fills -count).
func (v *VM) filmDelta(b []byte) {
	scr := v.mem[v.ru32(dsWlogic):]
	line := int(le16b(b)) * gameWidth
	lines := b[2]
	p := 4
	for {
		di := line
		packets := b[p]
		p++
		for ; packets != 0; packets-- {
			di += int(b[p])
			c := int(int8(b[p+1]))
			p += 2
			if c > 0 {
				copy(scr[di:di+c], b[p:p+c])
				p += c
				di += c
			} else {
				col := b[p]
				p++
				for range -c {
					scr[di] = col
					di++
				}
			}
		}
		line += gameWidth
		if lines--; lines == 0 {
			return
		}
	}
}

// filmBrun is 0x14418: 200 rows of packets (signed count: >= 0 fills,
// < 0 copies -count), written on without realigning to the rows; with
// filmOverlay colour 0 is transparent.
func (v *VM) filmBrun(b []byte) {
	scr := v.mem[v.ru32(dsWlogic):]
	transp := v.film.id&filmOverlay != 0
	di, p := 0, 0
	for range 200 {
		packets := b[p]
		p++
		for ; packets != 0; packets-- {
			c := int(int8(b[p]))
			p++
			if c >= 0 {
				col := b[p]
				p++
				if !transp || col != 0 {
					for i := range c {
						scr[di+i] = col
					}
				}
				di += c
			} else {
				for range -c {
					if col := b[p]; !transp || col != 0 {
						scr[di] = col
					}
					p++
					di++
				}
			}
		}
	}
}

// filmAudio is 0x14211: the chunks of the channels enabled in the flags'
// high byte, copied into one of two speech buffers and queued on the
// film's mixer channel at 0x1bfca*n/waitclock Hz.
func (v *VM) filmAudio(b []byte, typ uint16) {
	f := &v.film
	if uint16(1<<(typ-0x1000))<<8&uint16(f.id) == 0 {
		return
	}
	f.speech ^= 1
	copy(v.sb.film[f.speech][:], b)
	n := uint32(len(b))
	if f.waitclock == 0 { // div bx at 0x14254 by zero
		v.divHang()
		return
	}
	q := 60 * uint32(uint16(n)) / uint32(uint16(f.waitclock))
	if q > 0xffff {
		// the handler's retries shift in edx's upper half, which comes from
		// before the film opcode and isn't modelled
		v.fatal("film: the speech rate overflows div at 0x14254")
		return
	}
	freq := uint16(q)
	if v.sbFilmPush(f.ch, sbFilmBase+uint32(f.speech)*0x10000, n, freq) {
		// meant to set the film's 0x64 (skip the frame wait), but edi
		// points at the mixer channel by then
		v.mem[f.ch+0x64] = 1
	}
}
