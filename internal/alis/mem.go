package alis

import "encoding/binary"

// VM memory is one flat byte array in game byte order (little endian on the
// PC). These are alis' xread/xwrite accessors (mem.h); offsets wrap like the
// u32 arithmetic of the original.

func (v *VM) r8(o uint32) uint8    { return v.mem[o] }
func (v *VM) r16(o uint32) int16   { return int16(binary.LittleEndian.Uint16(v.mem[o:])) }
func (v *VM) r32(o uint32) int32   { return int32(binary.LittleEndian.Uint32(v.mem[o:])) }
func (v *VM) ru16(o uint32) uint16 { return binary.LittleEndian.Uint16(v.mem[o:]) }
func (v *VM) ru32(o uint32) uint32 { return binary.LittleEndian.Uint32(v.mem[o:]) }

func (v *VM) w8(o uint32, x uint8)    { v.mem[o] = x }
func (v *VM) w16(o uint32, x int16)   { binary.LittleEndian.PutUint16(v.mem[o:], uint16(x)) }
func (v *VM) w32(o uint32, x int32)   { binary.LittleEndian.PutUint32(v.mem[o:], uint32(x)) }
func (v *VM) wu16(o uint32, x uint16) { binary.LittleEndian.PutUint16(v.mem[o:], x) }
func (v *VM) wu32(o uint32, x uint32) { binary.LittleEndian.PutUint32(v.mem[o:], x) }

func (v *VM) add8(o uint32, x int8)   { v.mem[o] += uint8(x) }
func (v *VM) add16(o uint32, x int16) { v.w16(o, v.r16(o)+x) }
func (v *VM) add32(o uint32, x int32) { v.w32(o, v.r32(o)+x) }
func (v *VM) sub8(o uint32, x int8)   { v.mem[o] -= uint8(x) }
func (v *VM) sub16(o uint32, x int16) { v.w16(o, v.r16(o)-x) }
func (v *VM) sub32(o uint32, x int32) { v.w32(o, v.r32(o)-x) }

// read24 is alis' _read_le24: values above 0x7fffff collapse to their top
// byte shifted into the low byte (it is only used for small offsets).
func (v *VM) read24(o uint32) uint32 {
	r := uint32(v.mem[o]) | uint32(v.mem[o+1])<<8 | uint32(v.mem[o+2])<<16
	if r > 0x7fffff {
		return (r << 8) & 0xff
	}
	return r
}

// r32wc / w32wc access a 16.16 world coordinate: v3 PC data stores the
// integer word first, so the words are swapped against a plain dword.
func (v *VM) r32wc(base, off uint32) int32 {
	hi := uint16(v.r16(base + off))
	lo := uint16(v.r16(base + off + 2))
	return int32(uint32(hi)<<16 | uint32(lo))
}

func (v *VM) w32wc(base, off uint32, val int32) {
	v.w16(base+off, int16(val>>16))
	v.w16(base+off+2, int16(val))
}

// ioMalloc is memalloc (START.EXE 0x1c58): a block taken from the top of
// memory, with its size in the 8 bytes before it, and a handle in tabptr
// that points to it. Without a free handle the block stays taken.
func (v *VM) ioMalloc(rawsize int32) int32 {
	size := ((uint32(rawsize) - 1) | 3) + 9
	blockloc := v.finmem() - size
	if blockloc < v.finprog() {
		return 0
	}
	v.setFinmem(blockloc)
	v.wu32(blockloc, size)
	for i := uint32(0); i < 0x10; i++ {
		if h := tabptr + i*8; v.ru32(h) == 0 {
			v.wu32(h, blockloc+8)
			v.wu32(h+4, blockloc+8)
			return int32(h)
		}
	}
	return 0
}

// ioMfree is memfree (START.EXE 0x1ca2): the handle is freed; the memory
// only if the block is the top one.
func (v *VM) ioMfree(addr int32) {
	blockloc := v.ru32(uint32(addr)+4) - 8
	v.wu32(uint32(addr), 0)
	if v.finmem() == blockloc {
		v.setFinmem(v.finmem() + v.ru32(blockloc))
	}
}

// finmem (0x15f26) is the bottom of the blocks taken from the top of
// memory, finprog (0x15f66) the end of the loaded scripts.
const (
	dsFinmem  = 0x15f26
	dsFinprog = 0x15f66
)

func (v *VM) finmem() uint32      { return v.ru32(dsFinmem) }
func (v *VM) setFinmem(x uint32)  { v.wu32(dsFinmem, x) }
func (v *VM) finprog() uint32     { return v.ru32(dsFinprog) }
func (v *VM) setFinprog(x uint32) { v.wu32(dsFinprog, x) }

const dsMatmask = 0x16032 // material mask of the tests and the land queries

func (v *VM) matmask() int16     { return v.r16(dsMatmask) }
func (v *VM) setMatmask(x int16) { v.w16(dsMatmask, x) }
