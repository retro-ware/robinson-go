package alis

import (
	"fmt"
	"os"
	"strings"
)

var fliDebug = os.Getenv("FLIDBG") != ""

// FLI/FLC films on the PC (alis video.c, video/fli.c, escnames.c
// copenfilm/cdelfilm). A film plays from buf: the whole film file, or VM
// memory for films stored in a resource.

const vgaLogicSize = 128400 - 0x40

type film struct {
	id                 int16
	playing            bool
	buf                []byte
	addr1, addr2, endp int
	frame, frames      int16
	width, height      uint16
	result             int16
	batchframes        int16
	waitclock          int16
	basemain           uint32
	active             bool // eAlisVideoFLIC
	endframe           int

	vga      []byte
	deadline uint32 // timeclock of the next frame
}

func le16b(b []byte) uint16 { return uint16(b[0]) | uint16(b[1])<<8 }
func le32b(b []byte) uint32 { return uint32(le16b(b)) | uint32(le16b(b[2:]))<<16 }

func cdelfilm(v *VM) {
	v.endfilm()
	v.film = film{vga: v.film.vga}
}

func copenfilm(v *VM) {
	v.film = film{vga: v.film.vga}
	v.readexecOpername()
	v.film.id = int16(v.varD7)
	v.readexecOpernameSwap()
	v.fp = nil
	name := v.cstr(v.sd7)
	// the CD carries "150." (Falcon) and "300." (fullscreen) variants
	if i := strings.Index(name, "150."); i >= 0 {
		name = name[:i] + "300." + name[i+4:]
		copy(v.mem[v.sd7+uint32(i):], "300.")
	}
	if data, err := v.fs.ReadFile(name); err == nil {
		v.film.buf = data
	}
	v.readexecOpername()
	v.readexecOpername()
	v.film.waitclock = int16(v.varD7)
	v.film.basemain = v.basemain
	v.inifilm()
	v.varD7 = int32(v.film.frames)
	v.cstoreContinue()
}

func (v *VM) filmStreamClose() {}

func (v *VM) endfilm() { v.film.active = false }

func (v *VM) inifilm() {
	f := &v.film
	f.playing = false
	if f.vga == nil {
		f.vga = make([]byte, vgaLogicSize)
	}
	v.fliReset()
	f.active = true
	v.flitofen()
	f.deadline = v.timeclock
}

func (v *VM) runfilm() {
	f := &v.film
	img := &v.img
	bm := f.basemain
	img.fenx1 = v.r16(bm + 0xe)
	img.fenx2 = v.r16(bm+0x12) + img.fenx1
	img.clipl = v.r16(bm+0x12) + 1
	img.feny1 = v.r16(bm + 0x10)
	img.feny2 = v.r16(bm+0x14) + img.feny1
	img.cliph = v.r16(bm+0x14) + 1
	img.wloglarg = img.loglarg
	img.wlogx1, img.wlogy1 = 0, 0
	img.clipx1, img.clipy1 = img.fenx1, img.feny1
	img.clipx2, img.clipy2 = img.fenx2, img.feny2
	f.playing = true
	for {
		// wait for the frame's tick (START.EXE 0x13e4b)
		if f.waitclock > 0 {
			for f.deadline > v.timeclock && v.state != stateStopped {
				v.waitTick()
			}
		}
		f.deadline = v.timeclock + uint32(uint16(f.waitclock))
		result := v.flitofen()
		if fliDebug {
			fmt.Fprintf(os.Stderr, "F %d %d tc=%d\n", v.Now()/1000, f.frame, v.timeclock)
		}
		if !result {
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
	f.addr1 = f.addr2
	f.result = -0x7ffc
}

func (v *VM) flitofen() bool {
	f := &v.film
	for f.buf != nil && f.addr1+6 <= len(f.buf) {
		b := f.buf[f.addr1:]
		switch typ := le16b(b[4:]); {
		case typ == 0xaf11 || typ == 0xaf12:
			f.endp = f.addr1 + int(le32b(b))
			f.frames = int16(le16b(b[6:]))
			f.width = le16b(b[8:])
			f.height = le16b(b[10:])
			f.addr1 += 0x80
			f.frame = 0
			if need := int(f.width) * int(f.height); need > len(f.vga) {
				nv := make([]byte, need)
				copy(nv, f.vga)
				f.vga = nv
			}
			return true
		case typ == 0xf100 && f.endp > f.addr1:
			n := int(le32b(b))
			if n == 0 {
				n = 16
			}
			f.addr1 += n
			f.frame++
		case typ == 0xf1fa && f.endp > f.addr1:
			v.flinext(f.addr1)
			return true
		default:
			return false
		}
	}
	return false
}

func (v *VM) flinext(addr int) {
	f := &v.film
	f.endframe = addr + int(le32b(f.buf[addr:]))
	v.fliElements(addr + 16)
	dx, dy := int(v.img.fenx1), int(v.img.feny1)
	fw := int(f.width)
	for y := range int(f.height) {
		row := dy + y
		if row < 0 || row >= gameHeight || dx < 0 || dx+fw > gameWidth {
			continue
		}
		copy(v.img.physic[row*gameWidth+dx:row*gameWidth+dx+fw], f.vga[y*fw:])
	}
	copy(v.img.logic, v.img.physic)
	f.addr1 = f.endframe
	f.frame++
}

func (v *VM) fliElements(addr int) {
	f := &v.film
	for addr < f.endframe {
		off := int(le32b(f.buf[addr:]))
		typ := le16b(f.buf[addr+4:])
		addr += 6
		next := addr + off - 6
		b := f.buf[addr:]
		switch typ {
		case 0x4:
			v.fliPalette256(b)
		case 0x7:
			v.fliSS2(b)
		case 0xb:
			v.dosFilmPalette(b)
		case 0xc:
			v.fliDecomp(b, true)
		case 0xd:
			clear(f.vga[:int(f.width)*int(f.height)])
		case 0xf:
			v.fliDecomp(b, false)
		case 0x10:
			w := int(f.width)
			for y := range int(f.height) {
				copy(f.vga[y*w:y*w+w], b[y*w:])
			}
		default:
			if typ >= 0x1000 && typ <= 0x1007 || typ == 0x1300 {
				v.fliAudio(f.buf[addr:next], typ)
			}
		}
		addr = next
	}
}

func (v *VM) fliPalette256(b []byte) {
	packets := le16b(b)
	p, index := 2, 0
	for {
		index += int(b[p])
		n := int(b[p+1])
		p += 2
		if n == 0 {
			n = 256
		}
		if index+n > 256 {
			n = 256 - index
			packets = 1
		}
		for i := range n {
			c := b[p+i*3:]
			v.img.mpalet[index+i] = 0xff000000 | uint32(c[0])<<16 | uint32(c[1])<<8 | uint32(c[2])
		}
		p += n * 3
		index += n
		packets--
		if packets == 0 {
			break
		}
	}
}

func (v *VM) fliDecomp(b []byte, partial bool) {
	f := &v.film
	w := int(f.width)
	index, n := 0, int(f.height)
	p := 0
	if partial {
		index = int(le16b(b)) * w
		n = int(le16b(b[2:]))
		p = 4
	}
	vga := f.vga
	for ; n > 0; n-- {
		if index+w > len(vga) {
			break
		}
		packets := int(b[p])
		p++
		col := 0
		for ; packets > 0; packets-- {
			if partial {
				col += int(b[p])
				p++
			}
			count := int(int8(b[p]))
			p++
			if partial {
				count = -count
			}
			if count >= 0 {
				if count == 0 {
					count = 256
				}
				if col+count > w {
					count = w - col
					n, packets = 1, 1
				}
				c := b[p]
				p++
				for i := range count {
					vga[index+col+i] = c
				}
			} else {
				count = -count
				if col+count > w {
					count = w - col
					n, packets = 1, 1
				}
				copy(vga[index+col:index+col+count], b[p:])
				p += count
			}
			col += count
		}
		index += w
	}
}

func (v *VM) fliSS2(b []byte) {
	f := &v.film
	w := int(f.width)
	vga := f.vga
	lines := le16b(b)
	p, y := 2, 0
	for lines > 0 {
		op := le16b(b[p:])
		p += 2
		switch op & 0xc000 {
		case 0xc000:
			y += int(uint16(-int16(op)))
			continue
		case 0x8000:
			if i := y*w + w - 1; i < len(vga) {
				vga[i] = uint8(op)
			}
			continue
		}
		x := 0
		line := y * w
		for packets := op; packets > 0; packets-- {
			x += int(b[p])
			count := int8(b[p+1])
			p += 2
			if count >= 0 {
				n := int(count) * 2
				if x+n > w {
					n = max(w-x, 0)
				}
				if line+x+n <= len(vga) {
					copy(vga[line+x:line+x+n], b[p:])
				}
				p += int(count) * 2
				x += n
			} else {
				b0, b1 := b[p], b[p+1]
				p += 2
				for k := 0; k < int(-count) && x+2 <= w; k++ {
					if line+x+1 < len(vga) {
						vga[line+x], vga[line+x+1] = b0, b1
					}
					x += 2
				}
			}
		}
		y++
		lines--
	}
}

func (v *VM) fliAudio(b []byte, typ uint16) {
	n := len(b) &^ 1
	if n == 0 {
		return
	}
	var smp []byte
	if typ == 0x1300 {
		n /= 2
		smp = make([]byte, n)
		for i := range n {
			smp[i] = b[i*2+1]
		}
	} else {
		if uint16(1<<(typ-0x1000))<<8&uint16(v.film.id) == 0 {
			return
		}
		smp = b[:n]
	}
	f := uint32(22050)
	if v.film.waitclock > 0 {
		f = 60 * uint32(n) / uint32(v.film.waitclock)
	}
	v.fliAudioPush(smp, int16(min(f, 32767)))
}
