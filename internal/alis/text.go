package alis

import "strconv"

// Text output (alis image.c put_char, put_string, valtostr).

func (v *VM) putChar(c int8) {
	img := &v.img
	if v.charmode != 2 {
		return // 0 prints to the DOS console, 1 is unused on the PC
	}
	switch c {
	case '\n':
		img.depz -= int16(v.fohaut)
	case '\r':
		img.depx = 0
	default:
		if c != ' ' {
			if int16(v.fonum) < 0 {
				return
			}
			idx := int16(c) - int16(v.foasc)
			if idx > -1 && idx-int16(v.fomax) < 0 {
				v.flagmain = 1
				img.invertX = 0
				v.fadddes = 1
				v.put(uint16(idx + int16(v.fonum)))
			}
		}
		img.depx += int16(v.folarg)
	}
}

func (v *VM) putString() {
	for p := v.sd7; v.mem[p] != 0; p++ {
		v.putChar(int8(v.mem[p]))
	}
}

// valtostr writes value in decimal as a C string at addr.
func (v *VM) valtostr(addr uint32, value int16) {
	s := strconv.Itoa(int(value))
	copy(v.mem[addr:], s)
	v.mem[addr+uint32(len(s))] = 0
}
