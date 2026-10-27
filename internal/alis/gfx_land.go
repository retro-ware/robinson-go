package alis

// The land scenes (START.EXE 0xbb70..0xfd00) are being ported; until then
// a land scene stops the VM.

// openland is 0xbb70, for copensc.
func (v *VM) openland(id uint16) {
	v.fatal("land: START.EXE 0xbb70 is not ported")
}

// affiland is 0xbc6e: a changed land scene.
func (v *VM) affiland(a uint32) {
	v.fatal("land: START.EXE 0xbc6e is not ported")
}

// landRedraw is 0xbe63: a land scene within the redrawn rectangle.
func (v *VM) landRedraw(a uint32, id, di uint16) {
	v.fatal("land: START.EXE 0xbe63 is not ported")
}

// landPosition is 0xc0b3: where element di of land scene a is drawn.
func (v *VM) landPosition(a uint32, di uint16) {
	v.fatal("land: START.EXE 0xc0b3 is not ported")
}
