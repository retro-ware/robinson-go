package alis

import "encoding/binary"

// sprite is one 0x36-byte record in the sprite pool (alis sSprite). Records
// are addressed by a u16 index relative to img.basesprite; index 0 is the
// C NULL.
type sprite []byte

func (v *VM) spr(i uint16) sprite {
	o := uint32(v.img.basesprite) + uint32(i)
	return sprite(v.img.spritemem[o : o+sprSz])
}

func (v *VM) sprTonext(i uint16) uint16 { return v.spr(i).toNext() }

func (s sprite) u16(o int) uint16     { return binary.LittleEndian.Uint16(s[o:]) }
func (s sprite) s16(o int) int16      { return int16(binary.LittleEndian.Uint16(s[o:])) }
func (s sprite) set16(o int, x int16) { binary.LittleEndian.PutUint16(s[o:], uint16(x)) }
func (s sprite) setu16(o int, x uint16) {
	binary.LittleEndian.PutUint16(s[o:], x)
}
func (s sprite) u24(o int) uint32 { return uint32(s[o]) | uint32(s[o+1])<<8 | uint32(s[o+2])<<16 }
func (s sprite) set24(o int, x uint32) {
	s[o] = byte(x)
	s[o+1] = byte(x >> 8)
	s[o+2] = byte(x >> 16)
}

func (s sprite) state() int8         { return int8(s[0x00]) }
func (s sprite) setState(x int8)     { s[0x00] = byte(x) }
func (s sprite) numelem() int8       { return int8(s[0x01]) }
func (s sprite) setNumelem(x int8)   { s[0x01] = byte(x) }
func (s sprite) screenID() int16     { return s.s16(0x02) }
func (s sprite) setScreenID(x int16) { s.set16(0x02, x) }
func (s sprite) toNext() uint16      { return s.u16(0x04) }
func (s sprite) setToNext(x uint16)  { s.setu16(0x04, x) }
func (s sprite) link() uint16        { return s.u16(0x06) }
func (s sprite) setLink(x uint16)    { s.setu16(0x06, x) }
func (s sprite) newad() uint32       { return s.u24(0x08) }
func (s sprite) setNewad(x uint32)   { s.set24(0x08, x) }
func (s sprite) newf() int8          { return int8(s[0x0b]) }
func (s sprite) setNewf(x int8)      { s[0x0b] = byte(x) }
func (s sprite) newx() int16         { return s.s16(0x0c) }
func (s sprite) setNewx(x int16)     { s.set16(0x0c, x) }
func (s sprite) newy() int16         { return s.s16(0x0e) }
func (s sprite) setNewy(x int16)     { s.set16(0x0e, x) }
func (s sprite) newd() int16         { return s.s16(0x10) }
func (s sprite) setNewd(x int16)     { s.set16(0x10, x) }
func (s sprite) data() uint32        { return s.u24(0x12) }
func (s sprite) setData(x uint32)    { s.set24(0x12, x) }
func (s sprite) flaginvx() uint8     { return s[0x15] }
func (s sprite) setFlaginvx(x uint8) { s[0x15] = x }
func (s sprite) depx() int16         { return s.s16(0x16) }
func (s sprite) setDepx(x int16)     { s.set16(0x16, x) }
func (s sprite) depy() int16         { return s.s16(0x18) }
func (s sprite) setDepy(x int16)     { s.set16(0x18, x) }
func (s sprite) depz() int16         { return s.s16(0x1a) }
func (s sprite) setDepz(x int16)     { s.set16(0x1a, x) }
func (s sprite) credonOff() int8     { return int8(s[0x1c]) }
func (s sprite) setCredonOff(x int8) { s[0x1c] = byte(x) }
func (s sprite) creducing() uint8    { return s[0x1d] }
func (s sprite) setCreducing(x uint8) {
	s[0x1d] = x
}
func (s sprite) clinking() int16       { return s.s16(0x1e) }
func (s sprite) setClinking(x int16)   { s.set16(0x1e, x) }
func (s sprite) cordspr() uint8        { return s[0x20] }
func (s sprite) setCordspr(x uint8)    { s[0x20] = x }
func (s sprite) chsprite() uint8       { return s[0x21] }
func (s sprite) setChsprite(x uint8)   { s[0x21] = x }
func (s sprite) scriptEnt() uint16     { return s.u16(0x22) }
func (s sprite) setScriptEnt(x uint16) { s.setu16(0x22, x) }
func (s sprite) sprite0x28() uint32    { return binary.LittleEndian.Uint32(s[0x24:]) }
func (s sprite) setSprite0x28(x uint32) {
	binary.LittleEndian.PutUint32(s[0x24:], x)
}
func (s sprite) width() int16            { return s.s16(0x28) }
func (s sprite) setWidth(x int16)        { s.set16(0x28, x) }
func (s sprite) height() int16           { return s.s16(0x2a) }
func (s sprite) setHeight(x int16)       { s.set16(0x2a, x) }
func (s sprite) newzoomx() uint16        { return s.u16(0x2c) }
func (s sprite) setNewzoomx(x uint16)    { s.setu16(0x2c, x) }
func (s sprite) newzoomy() uint16        { return s.u16(0x2e) }
func (s sprite) setNewzoomy(x uint16)    { s.setu16(0x2e, x) }
func (s sprite) unknown0x30() uint16     { return s.u16(0x30) }
func (s sprite) setUnknown0x30(x uint16) { s.setu16(0x30, x) }
func (s sprite) unknown0x32() uint16     { return s.u16(0x32) }
func (s sprite) setUnknown0x32(x uint16) { s.setu16(0x32, x) }
func (s sprite) prev() uint16            { return s.u16(0x34) }
func (s sprite) setPrev(x uint16)        { s.setu16(0x34, x) }

// screen records live in the main script's vram at basemain+idx.
const (
	scState     = 0x00
	scNumelem   = 0x01
	scScreenID  = 0x02
	scToNext    = 0x04
	scLink      = 0x06
	scNewx      = 0x0e
	scNewy      = 0x10
	scWidth     = 0x12
	scHeight    = 0x14
	scDepx      = 0x16
	scDepy      = 0x18
	scDepz      = 0x1a
	scCredonOff = 0x1c
	scCreducing = 0x1d
	scRedmax    = 0x1e
	scRedshift  = 0x1f
)

func (v *VM) sca(idx uint16) uint32 { return v.basemain + uint32(idx) }

func (v *VM) scrState(idx uint16) int8       { return int8(v.mem[v.sca(idx)]) }
func (v *VM) setScrState(idx uint16, x int8) { v.mem[v.sca(idx)] = byte(x) }
func (v *VM) scrScreenID(idx uint16) uint16  { return v.ru16(v.sca(idx) + scScreenID) }
func (v *VM) scrToNext(idx uint16) uint16    { return v.ru16(v.sca(idx) + scToNext) }

// vram context fields below an entity's vram_org (alis get_0xNN_*)
func (v *VM) screenIDOf(vram uint32) uint16 { return v.ru16(vram - 0x16) }
func (v *VM) curScreenID() uint16           { return v.ru16(v.script.vramOrg - 0x16) }
func (v *VM) elemHead() uint16              { return v.ru16(v.script.vramOrg - 0x18) }
func (v *VM) setElemHead(x uint16)          { v.wu16(v.script.vramOrg-0x18, x) }
func (v *VM) xinv(vram uint32) uint8        { return v.mem[vram-0x03] }
func (v *VM) cforme(vram uint32) int16      { return v.r16(vram - 0x1a) }
