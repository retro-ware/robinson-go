package alis

import "encoding/binary"

// VM memory is one flat byte array in game byte order (little endian on the
// PC). These are alis' xread/xwrite accessors (mem.h); offsets wrap like the
// u32 arithmetic of the original.

func (v *VM) r8(o uint32) uint8   { return v.mem[o] }
func (v *VM) r16(o uint32) int16  { return int16(binary.LittleEndian.Uint16(v.mem[o:])) }
func (v *VM) r32(o uint32) int32  { return int32(binary.LittleEndian.Uint32(v.mem[o:])) }
func (v *VM) ru16(o uint32) uint16 { return binary.LittleEndian.Uint16(v.mem[o:]) }
func (v *VM) ru32(o uint32) uint32 { return binary.LittleEndian.Uint32(v.mem[o:]) }

func (v *VM) w8(o uint32, x uint8)   { v.mem[o] = x }
func (v *VM) w16(o uint32, x int16)  { binary.LittleEndian.PutUint16(v.mem[o:], uint16(x)) }
func (v *VM) w32(o uint32, x int32)  { binary.LittleEndian.PutUint32(v.mem[o:], uint32(x)) }
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

// ioMalloc allocates a block below finmem and returns the address of a
// pointer pair in tabptr that refers to it (v3: 32-bit handle table).
func (v *VM) ioMalloc(rawsize int32) int32 {
	size := int32((uint32(rawsize)-1)|3) + 9
	blockloc := int32(v.finmem) - size
	if int32(v.finprog) < blockloc {
		v.finmem = uint32(blockloc)
		v.w32(uint32(blockloc), size)
		for i := uint32(0); i < 0x10; i++ {
			if v.r32(v.tabptr+i*8) == 0 {
				v.w32(v.tabptr+i*8+0, blockloc+8)
				v.w32(v.tabptr+i*8+4, blockloc+8)
				return int32(v.tabptr + i*8)
			}
		}
	}
	return 0
}

func (v *VM) ioMfree(addr int32) {
	blockloc := v.r32(uint32(addr) + 4)
	v.w32(uint32(addr), 0)
	v.w32(uint32(addr)+4, 0)
	if int32(v.finmem) == blockloc-8 {
		v.finmem += uint32(v.r32(v.finmem))
	}
}
