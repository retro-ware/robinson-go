package alis

// Silmarils script/data unpackers (alis unpack.c).

const (
	packedHeaderSize     = 6
	packedDictionarySize = 8
	vmSpecsSize          = 16
)

// unpackOld is the run-length packer (typepack 0x80 on the PC: modpack 1;
// other pack types interleave the output every modpack bytes).
func unpackOld(packed []byte, out []byte, modpack int) {
	pp, pu := 0, 0
	if modpack == 1 {
		for pp < len(packed) && pu < len(out) {
			b := int8(packed[pp])
			pp++
			counter := int(b) & 0x7f
			n := min(counter, len(out)-pu)
			if b < 0 {
				var rep byte
				if pp < len(packed) {
					rep = packed[pp]
					pp++
				}
				for i := range n {
					out[pu+i] = rep
				}
			} else {
				real := min(n, len(packed)-pp)
				copy(out[pu:pu+real], packed[pp:pp+real])
				for i := real; i < n; i++ {
					out[pu+i] = 0
				}
				pp += real
			}
			pu += n
		}
		return
	}

	cnt := modpack
	next := func() int8 {
		if pp < len(packed) {
			pp++
			return int8(packed[pp-1])
		}
		return 0
	}
	for {
		b := next()
		counter := int(b) & 0x7f
		if b < 0 {
			rep := next()
			for ; counter > 0; counter-- {
				if pu >= len(out) {
					if cnt--; cnt < 1 {
						break
					}
					pu = modpack - cnt
				}
				out[pu] = uint8(rep)
				pu += modpack
			}
		} else {
			for ; counter > 0; counter-- {
				c := next()
				if pu >= len(out) {
					if cnt--; cnt < 1 {
						break
					}
					pu = modpack - cnt
				}
				out[pu] = uint8(c)
				pu += modpack
			}
		}
		if pp == len(packed) || cnt <= 0 {
			break
		}
	}
}

// newUnpacker is the bit-stream packer (typepack 0xa0) with its 68000
// register model: d5 is the bit buffer, d7 the bit counter.
type newUnpacker struct {
	d5     int32
	d7     uint8
	packed []byte // with zero padding past end
	end    int
	pp     int
	over   int
}

func rotll(shift uint8, value uint32) uint32 {
	shift &= 31
	if shift == 0 {
		return value
	}
	return value<<shift | value>>(32-shift)
}

func swapWords(v uint32) uint32 { return v<<16 | v>>16 }

func (u *newUnpacker) byte() int8  { return int8(u.d5) }
func (u *newUnpacker) word() int16 { return int16(u.d5) }

func (u *newUnpacker) decode(bit uint16) {
	b := uint8(bit)
	u.d5 &= -0x10000
	u.d7 -= b
	if int8(u.d7) < 0 {
		u.d7 += b
		d5 := swapWords(rotll(u.d7, uint32(u.d5)))
		var w uint16
		if u.pp < u.end {
			w = uint16(u.packed[u.pp])<<8 + uint16(u.packed[u.pp+1])
			u.pp += 2
		} else {
			u.over++ // streams may end early and rely on zero padding
		}
		d5 = swapWords(d5&0xffff0000 + uint32(w))
		b -= u.d7
		u.d7 = 0x10
		u.d5 = int32(rotll(b, d5))
		u.d7 -= b
	} else {
		u.d5 = int32(rotll(b, uint32(u.d5)))
	}
}

func (u *newUnpacker) count(start uint16, stop int16, counter *int16) {
	for {
		u.decode(start)
		*counter += u.word()
		if u.word() != stop {
			break
		}
	}
}

// unpackNew decodes packed into out; out must have slack past size because
// back-references are not clipped at the end (as the original). It returns
// the bytes written and how many words were read past the packed data.
func unpackNew(packed []byte, out []byte, size int, dict []byte) (int, int) {
	u := &newUnpacker{packed: make([]byte, len(packed)+2), end: len(packed)}
	copy(u.packed, packed)
	pu, n := 0, 0
	writeNeg := func(val int16, counter *int16) {
		offset := -int(val)
		for {
			src := pu + offset - 1
			var c byte
			if src >= 0 {
				c = out[src]
			}
			out[pu] = c
			pu++
			n++
			*counter--
			if *counter == -1 {
				break
			}
		}
	}
	for pu < size {
		u.decode(1)
		if u.byte() != 0 {
			counter := int16(0)
			u.count(2, 3, &counter)
			for {
				u.decode(8)
				out[pu] = byte(u.byte())
				pu++
				n++
				counter--
				if counter == -1 {
					break
				}
			}
			if pu >= size {
				break
			}
		}
		u.decode(3)
		bit := uint16(dict[uint8(u.byte())])
		u.d5 = u.d5&-0x10000 + int32(int16(u.d5&3))
		if u.word() != 0 {
			counter := u.word()
			u.decode(bit)
			writeNeg(u.word(), &counter)
		} else {
			u.decode(bit)
			save := uint16(u.word())
			counter := int16(0)
			u.count(3, 7, &counter)
			u.d5 = u.d5&-0x10000 + int32(save)
			counter += 4
			writeNeg(u.word(), &counter)
		}
	}
	return n, u.over
}
