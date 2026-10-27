package alis

import "strconv"

// Text output (alis image.c put_char, put_string, valtostr).

// putChar is START.EXE 0x3ecf: in charmode 2 a character is a font sprite
// of the main script put at the put offset, in the font colour if set.
func (v *VM) putChar(c uint8) {
	if v.charmode != 2 {
		return // 0 prints to the DOS console, 1 is unused on the PC
	}
	switch c {
	case '\n':
		v.w16(dsDepz, v.r16(dsDepz)-int16(v.fohaut))
		return
	case '\r':
		v.wu16(dsDepx, 0)
		return
	case ' ':
	default:
		if int16(v.fonum) < 0 {
			return
		}
		if idx := uint16(c) - v.foasc; int16(idx) >= 0 && idx < v.fomax {
			v.mem[dsFlagmain] = 1
			v.mem[dsInvx] = 0
			v.mem[dsPutAgain] = 1
			v.mem[dsMonoOn] = 1
			v.mem[dsPutSub] = 0
			v.putin(idx + v.fonum)
			v.mem[dsMonoOn] = 0
		}
	}
	v.w16(dsDepx, v.r16(dsDepx)+int16(v.folarg))
}

func (v *VM) putString() {
	for p := v.sd7; v.mem[p] != 0; p++ {
		v.putChar(v.mem[p])
	}
}

// valtostr writes value in decimal as a C string at addr.
func (v *VM) valtostr(addr uint32, value int16) {
	s := strconv.Itoa(int(value))
	copy(v.mem[addr:], s)
	v.mem[addr+uint32(len(s))] = 0
}
