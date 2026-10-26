package alis

// The mouse cursor of START.EXE: a 16x16 sprite drawn straight into video
// memory over a saved background.

type cursorState struct {
	shown  int8   // 0x16077: < 0 hidden
	lock   uint8  // 0x16078: the timer handler moves the cursor when it is 0xff
	x, y   int16  // 0x1606c: where it is drawn
	sprite uint32 // 0x1607a: cdefmouse sprite
}

// cursorTick is the timer handler's part (START.EXE 0x99be): follow the
// mouse.
func (v *VM) cursorTick() {
	c := &v.cursor
	if c.shown < 0 {
		return
	}
	c.lock++
	if c.lock == 0 {
		if x, y, _ := v.mouseRead(); x != c.x || y != c.y {
			v.cursorRestore()
			c.x, c.y = x, y
			v.cursorDraw()
		}
	}
	c.lock--
}

// cursorDraw and cursorRestore draw and remove the cursor in video memory
// (START.EXE 0x139f6, 0x13a16). Until the screen is kept as the original
// keeps video memory, the cursor is drawn on the host's frames only
// (compose).
func (v *VM) cursorDraw()    {}
func (v *VM) cursorRestore() {}

// cursorBlit draws the cursor sprite at x, y into the 320x200 dst
// (START.EXE 0x13a36): 16x16 pixels clipped at the right and bottom
// edges; type 0 is 4-bit, 0x10 4-bit plus a colour base, anything else
// 8-bit; colour 0 is clear.
func (v *VM) cursorBlit(dst []byte, x, y int16) {
	h := int16(16)
	if y >= 0xb8 {
		h = 200 - y
	}
	w := int16(16)
	if x >= 0x130 {
		w = 320 - x
	}
	o := int(uint16(y)*320) + int(uint16(x))
	s := v.cursor.sprite
	switch typ := v.mem[s]; typ {
	case 0, 0x10:
		var base uint8
		s += 6
		if typ == 0x10 {
			base = v.mem[s]
			s += 2
		}
		for range h {
			p, d := s, o
			for n := w; ; {
				b := v.mem[p]
				p++
				if b>>4 != 0 {
					dst[d] = b>>4 + base
				}
				d++
				if n--; n == 0 {
					break
				}
				if b&0xf != 0 {
					dst[d] = b&0xf + base
				}
				d++
				if n--; n == 0 {
					break
				}
			}
			s += 8
			o += 320
		}
	default:
		s += 8
		for range h {
			for i := range w {
				if b := v.mem[s+uint32(i)]; b != 0 {
					dst[o+int(i)] = b
				}
			}
			s += 16
			o += 320
		}
	}
}
