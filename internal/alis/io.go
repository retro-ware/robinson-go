package alis

// Keyboard and joystick reads (alis sys.c io_*), SDL key semantics.

func (v *VM) ioInkey() uint8 {
	in := &v.in
	if in.ButtonSc <= 0 || !in.Keys[in.ButtonSc] {
		in.ButtonSc, in.ButtonSym = 0, 0
	}
	switch in.ButtonSc {
	case ScEscape:
		return 0x1b
	case ScUp:
		return 0xc8
	case ScDown:
		return 0xd0
	case ScLeft:
		return 0xcb
	case ScRight:
		return 0xcd
	case ScKP0:
		return '0'
	case ScKPPlus:
		return '+'
	case ScKPMinus:
		return '-'
	case ScLShift, ScRShift, ScLCtrl, ScRCtrl, ScLAlt, ScRAlt, ScCapsLock:
		return 0
	}
	if in.ButtonSc >= ScKP1 && in.ButtonSc <= ScKP9 {
		return uint8('1' + in.ButtonSc - ScKP1)
	}
	if in.ButtonSc >= ScF1 && in.ButtonSc <= ScF10 {
		return uint8(0xbb + in.ButtonSc - ScF1)
	}
	if in.Mods&kmodShift != 0 || in.Keys[ScLShift] || in.Keys[ScRShift] {
		if in.ButtonSc >= Sc1 && in.ButtonSc <= Sc0 {
			return "!@#$%^&*()"[in.ButtonSc-Sc1]
		}
		switch in.ButtonSc {
		case ScMinus:
			return '_'
		case ScEquals:
			return '+'
		}
	}
	return uint8(in.ButtonSym)
}

func (v *VM) ioShiftkey() uint8 {
	in := &v.in
	k := &in.Keys
	s := v.shift
	set := func(on bool, bit uint8) {
		if on {
			s |= bit
		} else {
			s &^= bit
		}
	}
	set(k[ScRShift], 1)
	set(k[ScLShift], 2)
	set(k[ScRCtrl] || k[ScLCtrl], 4)
	set(k[ScRAlt] || k[ScLAlt], 8)
	set(in.Mods&kmodCaps != 0, 16)
	v.shift = s
	return s
}

// ioGetkey waits for a key press and its release.
func (v *VM) ioGetkey() uint8 {
	var r uint8
	for v.state != stateStopped {
		if r = v.ioInkey(); r != 0 {
			break
		}
		v.sleep(eventTicksUs)
	}
	for v.state != stateStopped && v.ioInkey() != 0 {
		v.sleep(eventTicksUs)
	}
	return r
}

func (v *VM) joyCompose(mouseFire bool) uint8 {
	k := &v.in.Keys
	var b uint8
	if k[ScUp] || k[ScKP1+7] || k[ScKP1+6] || k[ScKP9] {
		b |= 1
	}
	if k[ScDown] || k[ScKP1+1] || k[ScKP1] || k[ScKP1+2] {
		b |= 2
	}
	if k[ScLeft] || k[ScKP1+3] || k[ScKP1+6] || k[ScKP1] {
		b |= 4
	}
	if k[ScRight] || k[ScKP1+5] || k[ScKP9] || k[ScKP1+2] {
		b |= 8
	}
	if mouseFire && v.in.Mouse.LB {
		b |= 0x80
	}
	return b
}

func (v *VM) ioJoy(port uint8) uint8 {
	if port != 0 {
		return v.joyCompose(false)
	}
	return v.joyCompose(true)
}

func (v *VM) ioJoykey(test uint8) uint8 {
	var r uint8
	if test == 0 {
		if v.joyCompose(true)&0x80 != 0 {
			r = 1
		}
		if v.shift&4 != 0 {
			r |= 2
		}
		if v.shift&8 != 0 {
			r |= 4
		}
	}
	return r
}
