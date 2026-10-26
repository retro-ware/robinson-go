package alis

// Not yet ported: the RRQ 3D landscape (render/rrq3d.c, rrq3d_dos.c).
// These stand-ins draw nothing.

type land struct{}

// TODO(land): port rrq3d.c / rrq3d_dos.c.
func (v *VM) render3dInit()                          {}
func (v *VM) openland(id uint16)                     {}
func (v *VM) landtopix(scene uint32, elem uint16)    {}
func (v *VM) landtofi(sprite, scene uint16)          {}
func (v *VM) affiland(scene uint16)                  {}
func (v *VM) pointpix(x, y int16)                    {}
func (v *VM) calctop(scene uint32, x, y int16) int32 { return 0 }
