package alis

// Keyboard and mouse as START.EXE reads them: the keyboard interrupt
// handler keeps a key-down table, the code of the last key and the shift
// bits; the timer handler samples the mouse driver on every tick.

type kbdState struct {
	tab  [128]uint8 // key code of scancode-1 (layout)
	down [128]bool  // 0x18b6c
	last uint8      // 0x18b68: key code of the last key pressed, 0 when released
	mods uint8      // 0x18b6a: 1 right shift, 2 left shift, 4 ctrl, 8 alt
}

// initInput reads the keyboard layout and whether to use the mouse from
// START.STP (START.EXE 0x9de: letter 13, digit 11); without one it is the
// 'Q' layout with a mouse.
func (v *VM) initInput() {
	layout, mouse := byte('Q'), true
	if stp, err := v.fs.ReadFile("START.STP"); err == nil && len(stp) >= 14 {
		layout, mouse = stp[13], stp[11] != '0'
	}
	v.initKeyboard(layout)
	v.cursor.shown = -1
	if mouse {
		v.initMouse()
		v.theconfig |= 4
	}
}

// initKeyboard sets the layout table from START.STP's keyboard letter and
// the country it implies (START.EXE 0x2268).
func (v *VM) initKeyboard(layout byte) {
	k := &v.kbd
	k.tab = keytab
	v.pays = 0
	switch layout {
	case 'A':
		v.pays = 1
		copy(k.tab[0x0e:], keytabA[:])
	case 'Z':
		v.pays = 2
		copy(k.tab[0x0e:], keytabZ[:])
	}
}

// kbdIRQ is the keyboard interrupt handler (START.EXE 0x2326) for one
// scancode. An 0xE0 prefix is the release of scancode 0x60.
func (v *VM) kbdIRQ(code uint8) {
	k := &v.kbd
	i := int(code&0x7f) - 1
	if i < 0 {
		return // the original touches the byte before its tables
	}
	if code&0x80 == 0 {
		k.down[i] = true
		if i >= 0x70 {
			return
		}
		switch c := k.tab[i]; c {
		case 0xe0, 0xe1, 0xe2, 0xe3:
			k.mods |= modBits[c-0xe0]
		case 0xfe:
		default:
			k.last = c
		}
		return
	}
	k.down[i] = false
	switch c := k.tab[i]; c {
	case 0xe0, 0xe1, 0xe2, 0xe3:
		k.mods &^= modBits[c-0xe0]
	case 0xfe:
	default:
		k.last = k.held()
	}
	// Ctrl+Alt with '.' or 'S' as the last key makes the original call
	// itself until the stack overflows (0x23d3); not reproduced.
}

var modBits = [4]uint8{2, 1, 4, 8}

// held is the code of the highest key still down (START.EXE 0x2440).
func (k *kbdState) held() uint8 {
	for i := 127; i >= 0; i-- {
		if k.down[i] {
			if c := k.tab[i]; c < 0xe0 || c > 0xe3 {
				return c
			}
		}
	}
	return 0
}

// getkey waits for a key press and its release and returns the key's
// code (START.EXE 0x2485).
func (v *VM) getkey() uint8 {
	for v.kbd.last == 0 && v.state != stateStopped {
		v.waitEvent()
	}
	r := v.kbd.last
	for v.kbd.last != 0 && v.state != stateStopped {
		v.waitEvent()
	}
	return r
}

// mouseState is the game's copy of the mouse (START.EXE 0x18e3b-0x18e43)
// and the offset csetmouse adds to it (0x18a98).
type mouseState struct {
	present    bool // driver found and enabled in START.STP (0x16068 == 0)
	x, y, b    int16
	offx, offy int16
}

const (
	mouseScreenW = gameWidth  // 0x178ef
	mouseScreenH = gameHeight // 0x178f1
)

func clampMouse(v, n int16) int16 {
	if v < 0 {
		return 0
	}
	if v >= n {
		return n - 1
	}
	return v
}

// initMouse is START.EXE 0x2676 with a driver present: the centre, then a
// first sample of the driver.
func (v *VM) initMouse() {
	m := &v.mouse
	m.x, m.y, m.b = mouseScreenW/2, mouseScreenH/2, 0
	m.present = true
	v.mouseSample()
}

// mouseSample copies the driver's state (START.EXE 0x27fa, run by the
// timer handler). The driver's x runs 0..639.
func (v *VM) mouseSample() {
	m := &v.mouse
	if !m.present {
		return
	}
	in := v.in.Mouse
	m.b = 0
	if in.LB {
		m.b |= 1
	}
	if in.RB {
		m.b |= 2
	}
	m.x = clampMouse(int16(in.X*2)>>1, mouseScreenW)
	m.y = clampMouse(int16(in.Y), mouseScreenH)
}

// mouseRead is the position with the csetmouse offset and the buttons
// (START.EXE 0x2876); -1, -1, 0 without a mouse.
func (v *VM) mouseRead() (x, y, b int16) {
	m := &v.mouse
	if !m.present {
		return -1, -1, 0
	}
	return clampMouse(m.x+m.offx, mouseScreenW), clampMouse(m.y+m.offy, mouseScreenH), m.b
}

// setMouse makes the mouse read as x, y from now on (START.EXE 0x28f1); a
// negative x drops the offset.
func (v *VM) setMouse(x, y int16) {
	m := &v.mouse
	if x < 0 {
		m.offx, m.offy = 0, 0
		return
	}
	m.offx, m.offy = x-m.x, y-m.y
}
