package alis

// The mouse cursor of START.EXE: a 16x16 sprite drawn straight into video
// memory over a saved background (gfx_blit.go). The draw puts it into the
// logic screen while it copies a rectangle; the timer handler moves it.

// initCursor is the cursor part of the mouse setup (START.EXE 0x2676).
func (v *VM) initCursor() {
	v.wu16(dsCurW, 0xf)
	v.wu16(dsCurH, 0xf)
	if v.mouse.present {
		v.wu32(dsCurSave, curSaveBuf)
	}
}

// cursorTick is the timer handler's part (START.EXE 0x99be): follow the
// mouse unless a draw holds the lock.
func (v *VM) cursorTick() {
	if int8(v.mem[dsCurShown]) < 0 {
		return
	}
	v.mem[dsCurLock]++
	if v.mem[dsCurLock] == 0 {
		if x, y, _ := v.mouseRead(); x != v.r16(dsCurX) || y != v.r16(dsCurY) {
			v.cursorRestore()
			v.w16(dsCurX, x)
			v.w16(dsCurY, y)
			v.cursorDraw()
		}
	}
	v.mem[dsCurLock]--
}
