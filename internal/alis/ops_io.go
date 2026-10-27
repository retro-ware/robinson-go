package alis

import (
	"io"
)

// Palette, sound effect, file, pen and mouse opcodes (alis opcodes.c).

func cpalette(v *VM) {
	v.mem[dsFlagmain] = 0
	v.readexecOpername()
	palidx := int16(v.varD7)
	if palidx < 0 {
		v.dosCpalette(palidx, 0)
		return
	}
	addr := uint32(v.adresdes(int32(palidx)))
	v.dosCpalette(palidx, addr+uint32(v.r32(addr)))
}

func cdefcolor(v *VM) {
	v.readexecOpername()
	v.readexecOpernameSaveD6()
	v.dosCdefcolor(int16(v.varD7), uint16(v.varD6))
}

func ctiming(v *VM) {
	v.readexecOpername()
	v.ctiming = uint8(v.varD7)
}

// sound effect parameters common to czap/cexplode/cding/cnoise
func (v *VM) sfxParams() (pereson, priorson uint8, volson int16, freqson, longson uint16) {
	pereson = uint8(v.ru16(v.script.vramOrg - 0x0e))
	v.readexecOpername()
	priorson = uint8(v.varD7)
	v.readexecOpername()
	volson = int16(v.varD7 << 8)
	v.readexecOpername()
	freqson = uint16(v.varD7)
	v.readexecOpername()
	longson = uint16(v.varD7)
	return
}

func sfxDvol(volson int16, longson uint16) int16 {
	d := uint32(int32(volson)) / uint32(longson)
	if d == 0 {
		d = 1
	}
	return -int16(d)
}

func czap(v *VM) {
	pereson, priorson, volson, freqson, longson := v.sfxParams()
	if longson == 0 {
		return
	}
	v.readexecOpername()
	dfreqson := uint16(v.varD7)
	v.runson(chanDingZap, pereson, priorson, volson, freqson, longson, 0, int16(dfreqson))
}

func cexplode(v *VM) {
	pereson, priorson, volson, freqson, longson := v.sfxParams()
	if longson == 0 {
		return
	}
	v.runson(chanExplode, pereson, priorson, volson, freqson, longson, sfxDvol(volson, longson), 0)
}

func cding(v *VM) {
	pereson := uint8(v.ru16(v.script.vramOrg - 0x0e))
	v.readexecOpername()
	priorson := uint8(v.varD7)
	v.readexecOpername()
	volson := int16(v.varD7 << 8)
	v.readexecOpername()
	freqson := uint16(v.varD7 >> 1)
	v.readexecOpername()
	longson := uint16(v.varD7)
	if longson == 0 {
		return
	}
	v.runson(chanDingZap, pereson, priorson, volson, freqson, longson, sfxDvol(volson, longson), 0)
}

func cnoise(v *VM) {
	pereson, priorson, volson, freqson, longson := v.sfxParams()
	if longson == 0 {
		return
	}
	v.runson(chanNoise, pereson, priorson, volson, freqson, longson, sfxDvol(volson, longson), 0)
}

// cinitab copies its inline bytes (count-1 of them) to a variable of the
// running script (START.EXE 0x87b3).
func cinitab(v *VM) {
	dst := v.vramAt(int32(v.read16()))
	n := uint32(int32(v.read16()) - 1)
	for i := uint32(0); i < n; i++ {
		v.mem[dst+i] = v.mem[v.script.pc+i]
	}
	v.script.pc += n
}

// cfcreat creates (truncates) a file; the new handle replaces the current
// file's and is closed at once (START.EXE 0x96ad). The word operand is unused.
func cfcreat(v *VM) {
	name := v.readZeroString()
	v.read16()
	if v.fp != nil {
		v.fp.Close()
		v.fp = nil
	}
	f, err := v.fs.Open(name, 0x200)
	if err != nil {
		v.alisError(8)
		return
	}
	f.Close()
}

// cfdel deletes a file (START.EXE 0x96c1).
func cfdel(v *VM) {
	if v.fs.Remove(v.readZeroString()) != nil {
		v.alisError(9)
	}
}

// cfwritei writes its inline string, terminator included (START.EXE 0x970b).
func cfwritei(v *VM) {
	start := v.script.pc
	v.readZeroString()
	v.fwrite(v.mem[start:v.script.pc])
}

// cpicture evaluates its operand and does nothing else (START.EXE 0x9884).
func cpicture(v *VM) { v.readexecOpernameSaveD7() }

func cfopen(v *VM) {
	var name string
	var mode uint16
	if v.mem[v.script.pc] == 0xff {
		v.script.pc++
		v.readexecOpernameSwap()
		v.readexecOpername()
		name = v.cstr(v.sd7)
		mode = uint16(v.varD7)
	} else {
		name = v.readZeroString()
		mode = uint16(v.read16())
	}
	v.afopen(name, mode)
	if v.fp == nil {
		v.fatal("can't open file " + name)
	}
}

func cfclose(v *VM) {
	if v.fp == nil || v.fp.Close() != nil {
		v.fatal("can't close file")
	}
	v.fp = nil
}

// fread reads up to len(dst) bytes like the C library.
func (v *VM) fread(dst []byte) int {
	if v.fp == nil {
		return 0
	}
	n, _ := io.ReadFull(v.fp, dst)
	return n
}

func (v *VM) fwrite(src []byte) {
	if v.fp != nil {
		v.fp.Write(src)
	}
}

// data files keep words big-endian
func cfreadv(v *VM) {
	v.fread(v.buffer[:2])
	v.varD7 = int32(int16(be16(v.buffer[:2])))
	v.cstoreContinue()
}

func cfwritev(v *VM) {
	v.readexecOpername()
	v.fwrite([]byte{byte(v.varD7 >> 8), byte(v.varD7)})
}

// unmixword undoes the word interleaving of PW60 data.
func (v *VM) unmixword(tgtAddr int32, tgtLen uint32) {
	var tmpOffsetLsr4 int32
	bufpackend := int32(tgtLen >> 5)
	tmp := make([]byte, uint16(tgtLen>>5)+4)
	half := int32(tgtLen >> 1)
	off2 := int32(1)
	tmp[0] = 1
	rd := func(o int32) uint8 { return v.r8(uint32(tgtAddr + o)) }
	wr := func(o int32, b uint8) { v.w8(uint32(tgtAddr+o), b) }
	for {
		tmpByte := rd(off2)
		off := off2
		for {
			xchg := tmpByte
			if half <= off {
				for {
					off = (off-half)*2 + 1
					xchg = rd(off)
					wr(off, tmpByte)
					tmpByte = xchg
					if half > off {
						break
					}
				}
				tmpOffsetLsr4 = off >> 4
				tmp[tmpOffsetLsr4] |= 1 << (uint16(off>>1) & 7)
				if off == off2 {
					break
				}
			}
			off *= 2
			tmpByte = rd(off)
			wr(off, xchg)
		}
		for tmp[tmpOffsetLsr4] == 0xff {
			tmpOffsetLsr4++
			if bufpackend <= tmpOffsetLsr4 {
				return
			}
		}
		tgt := tmp[tmpOffsetLsr4]
		tune := int32(-1)
		for {
			tune++
			bit := tgt & 1
			tgt >>= 1
			if bit == 0 {
				break
			}
		}
		off2 = (tune+tmpOffsetLsr4*8)*2 + 1
		if off2 >= half {
			return
		}
	}
}

func (v *VM) readVaddr() uint32 {
	base := v.script.vramOrg
	offset := v.read16()
	if offset == 0 {
		offset = v.read16()
		base = v.basemain
	}
	return uint32(int32(base) + int32(offset))
}

// fic_cellsize: word arrays are stored big-endian in data files.
func (v *VM) ficCellsize(cell uint32) uint8 {
	if v.r8(cell-2) == 2 {
		return 2
	}
	return 0
}

func ficSwap(p []byte, size uint8) {
	if size != 2 && size != 4 {
		return
	}
	s := int(size)
	for i := 0; i+s <= len(p); i += s {
		for l, r := 0, s-1; l < r; l, r = l+1, r-1 {
			p[i+l], p[i+r] = p[i+r], p[i+l]
		}
	}
}

func cfreadb(v *VM) {
	cell := v.readVaddr()
	addr := int32(cell)
	if int8(v.r8(uint32(addr)-1)) < 0 {
		// START.EXE 0x9796: the pointer cell must be positive (error 26)
		addr = v.r32(uint32(addr))
		if addr <= 0 {
			v.alisError(0x1a)
			return
		}
		addr = v.r32(uint32(addr))
	}
	length := uint32(v.read32())
	if v.typepack == 0 {
		v.fread(v.mem[addr : uint32(addr)+length])
	} else {
		v.unpackFile(v.mem[addr:], length)
		if v.wordpack != 0 {
			v.unmixword(addr, length)
		}
	}
	ficSwap(v.mem[addr:uint32(addr)+length], v.ficCellsize(cell))
}

// unpackFile reads a packed block of the open data file (unpack_script_fp):
// it reads as many packed bytes as the unpacked size.
func (v *VM) unpackFile(out []byte, size uint32) {
	packed := make([]byte, size)
	if uint8(v.typepack)&0xf0 == 0xa0 {
		dict := make([]byte, packedDictionarySize)
		v.fread(dict)
		v.fread(packed)
		unpackNew(packed, out, int(size), dict)
		return
	}
	v.fread(packed)
	modpack := 8
	switch uint8(v.typepack) {
	case 0x80:
		modpack = 1
	case 0xa0:
		modpack = 2
	}
	unpackOld(packed, out[:size], modpack)
}

func (v *VM) writeswap(addr, length, cell uint32) {
	size := v.ficCellsize(cell)
	if size == 2 || size == 4 {
		data := make([]byte, length)
		copy(data, v.mem[addr:addr+length])
		ficSwap(data, size)
		v.fwrite(data)
		return
	}
	v.fwrite(v.mem[addr : addr+length])
}

func cfwriteb(v *VM) {
	cell := v.readVaddr()
	length := uint32(v.read32())
	if int8(v.r8(cell-1)) > -1 {
		v.writeswap(cell, length, cell)
		return
	}
	addr1 := uint32(v.r32(cell))
	addr2 := uint32(v.r32(addr1))
	if addr2 > 0 {
		v.writeswap(addr2, length, cell)
	}
}

// pen drawing does nothing in the 3.x PC interpreter
func cplot(v *VM) {
	v.readexecOpername()
	v.readexecOpername()
}

func cdraw(v *VM) {
	v.readexecOpername()
	v.readexecOpernameSaveD6()
}

func cbox(v *VM)  { cdraw(v) }
func cboxf(v *VM) { cdraw(v) }

func cink(v *VM) {
	v.readexecOpername()
	v.img.inkcolor = uint16(v.varD7)
}

func cpset(v *VM) {
	v.readexecOpername()
	v.poldx = int16(v.varD7)
	v.readexecOpername()
	v.poldy = int16(v.varD7)
}

func cpmove(v *VM) {
	v.readexecOpername()
	v.poldx += int16(v.varD7)
	v.readexecOpername()
	v.poldy += int16(v.varD7)
}

func cpmode(v *VM) {
	v.readexecOpername()
	v.img.lineAMode = uint16(v.varD7)
}

func cxyscroll(v *VM) {
	v.readexecOpername()
	v.readexecOpername()
}

func clinking(v *VM) {
	v.readexecOpername()
	v.w16(v.script.vramOrg-0x2a, int16(v.varD7))
}

// cmouson shows the cursor (START.EXE 0x988a).
func cmouson(v *VM) {
	if !v.mouse.present || int8(v.mem[dsCurShown]) >= 0 {
		return
	}
	x, y, _ := v.mouseRead()
	v.w16(dsCurX, x)
	v.w16(dsCurY, y)
	v.cursorDraw()
	v.mem[dsCurShown] = 1
	v.mem[dsCurLock] = 0xff
}

// cmousoff hides the cursor (START.EXE 0x98b1).
func cmousoff(v *VM) {
	if !v.mouse.present || int8(v.mem[dsCurShown]) < 0 {
		return
	}
	v.mem[dsCurShown] = 0xff
	v.cursorRestore()
}

// cmouse stores x, y and the buttons (START.EXE 0x98d1).
func cmouse(v *VM) {
	x, y, b := v.mouseRead()
	v.varD7 = int32(x)
	v.cstoreContinue()
	v.varD7 = int32(y)
	v.cstoreContinue()
	v.varD7 = int32(b)
	v.cstoreContinue()
}

// cdefmouse sets the cursor sprite, and the mouse sprite's resource
// (START.EXE 0x98f1).
func cdefmouse(v *VM) {
	v.mem[dsFlagmain] = 0
	v.readexecOpername()
	entry, ok := v.resentry(uint16(v.varD7))
	if !ok {
		return
	}
	v.wu32(0x15eb6, entry)
	v.wu32(dsCurSprite, entry+v.ru32(entry))
	v.setFsu32(v.ru16(dsMouseSpr), spNewad, entry)
}

// csetmouse (START.EXE 0x9a22): x by getval, y by 0x9d17, which leaves it
// in D6 and restores D7; alis took y from the stale D6.
func csetmouse(v *VM) {
	v.readexecOpername()
	x := uint16(v.varD7)
	d7 := v.varD7
	v.readexecOpername()
	y := uint16(v.varD7)
	v.varD6, v.varD7 = v.varD7, d7
	v.setMouse(int16(x), int16(y))
}
