package alis

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
)

// Script loading and entity creation (alis script.c, v3 PC paths).

var tfibo = [16]int8{
	-0x22, -0x15, -0x0d, -0x08,
	-0x05, -0x03, -0x02, -0x01,
	0x01, 0x02, 0x03, 0x05,
	0x08, 0x0d, 0x15, 0x22,
}

func le16(b []byte) uint16 { return binary.LittleEndian.Uint16(b) }
func le32(b []byte) uint32 { return binary.LittleEndian.Uint32(b) }

func (v *VM) atprogAt(i int) uint32       { return uint32(v.r32(v.atprog + uint32(i)*4)) }
func (v *VM) atprogSet(i int, val uint32) { v.w32(v.atprog+uint32(i)*4, int32(val)) }

// debprotf is a binary search of the program table for an id (-1: absent).
func (v *VM) debprotf(target int16) int16 {
	start, end := 0, int(v.nbprog)-1
	for {
		mid := (end + start) >> 1
		cur := int16(v.ru16(v.atprogAt(mid)))
		if cur == target {
			return int16(mid)
		}
		if cur < target {
			start = mid + 1
		} else {
			end = mid - 1
		}
		if start > end {
			return -1
		}
	}
}

func (v *VM) searchInsert(size int, target int) int {
	start, end := 0, size-1
	for start <= end {
		mid := start + (end-start)/2
		cur := int(v.ru16(v.atprogAt(mid)))
		if cur == target {
			return mid
		}
		if cur < target {
			start = mid + 1
		} else {
			end = mid - 1
		}
	}
	return start
}

// readScriptFile reads and unpacks a script; name is the file name used for
// lookups. It returns the unpacked program (header + code + resources).
func (v *VM) readScriptFile(name string) ([]byte, error) {
	raw, err := v.fs.ReadFile(name)
	if err != nil {
		return nil, err
	}
	if len(raw) < packedHeaderSize {
		return nil, fmt.Errorf("%s: too short", name)
	}
	magic := le32(raw)
	isMain := le16(raw[4:]) == 0
	unpackSz := int(magic & 0xffffff)

	v.typepack = int8(magic >> 24)
	v.typepack &^= 1 // PC: 0x81/0xa1 mark the packers like 0x80/0xa0

	pos := packedHeaderSize
	if isMain {
		pos += vmSpecsSize
	}
	if v.typepack >= 0 {
		// stored
		n := unpackSz - pos
		if n <= 0 {
			return nil, fmt.Errorf("%s: file too small", name)
		}
		out := make([]byte, n)
		copy(out, raw[min(pos, len(raw)):])
		return out, nil
	}

	unpackSz -= packedHeaderSize
	if isMain {
		unpackSz -= vmSpecsSize
	}
	out := make([]byte, unpackSz+1024)
	if uint8(v.typepack)&0xf0 == 0xa0 {
		dict := raw[pos : pos+packedDictionarySize]
		pos += packedDictionarySize
		_, over := unpackNew(raw[pos:], out, unpackSz, dict)
		if over > 0x7f {
			return nil, fmt.Errorf("damaged script file: %s", name)
		}
	} else {
		modpack := 8
		if uint8(v.typepack) == 0x80 {
			modpack = 1
		}
		unpackOld(raw[pos:], out[:unpackSz], modpack)
	}
	return out[:unpackSz], nil
}

// scriptLoad loads a program by file name (alis script_load).
func (v *VM) scriptLoad(name string) *scriptData {
	data, err := v.readScriptFile(name)
	if errors.Is(err, os.ErrNotExist) {
		return nil // alis: a missing file is not fatal
	}
	if err != nil {
		v.fatal(err.Error())
		return nil
	}
	return v.scriptInit(name, data)
}

// scriptInit places an unpacked program in the arena (alis script_init).
func (v *VM) scriptInit(name string, data []byte) *scriptData {
	id := int16(le16(data))
	insert := v.debprotf(id)
	if insert > 0 && insert < int16(v.nbprog) {
		if s := v.loadedScripts[insert]; s.header.id == uint16(id) {
			return s
		}
	}

	s := &scriptData{name: name, sz: uint32(len(data)), typ: uint8(v.typepack)}
	s.header = scriptHeader{
		id:            le16(data[0:]),
		unknown01:     data[4],
		unknown02:     data[5],
		codeLocOffset: le16(data[2:]),
		retOffset:     le32(data[6:]),
		dwUnknown3:    le32(data[10:]),
		dwUnknown4:    le32(data[14:]),
		wUnknown5:     le16(data[18:]),
		vramAllocSz:   le16(data[20:]),
		wUnknown7:     le16(data[22:]),
	}

	s.dataOrg = v.finprog()
	if s.dataOrg > v.finmem() || uint32(len(data)) > v.finmem()-s.dataOrg {
		v.fatal("Out of game memory loading " + name)
		return nil
	}
	copy(v.mem[s.dataOrg:], data)

	ins := v.searchInsert(int(v.nbprog), int(s.header.id))
	v.setFinprog(v.finprog() + uint32(len(data)))
	v.dernprog += 4
	v.nbprog++
	for i := int(v.nbprog) - 2; i >= ins; i-- {
		v.atprogSet(i+1, v.atprogAt(i))
		v.loadedScripts[i+1] = v.loadedScripts[i]
	}
	v.atprogSet(ins, s.dataOrg)
	v.loadedScripts[ins] = s

	v.convertSamples(s)
	return s
}

// convertSamples decodes the packed (Fibonacci delta) samples of a program
// and applies invdigit.
func (v *VM) convertSamples(s *scriptData) {
	data := v.mem[s.dataOrg:]
	l := le32(data[0xe:])
	samples := int(int16(le16(data[l+0x10:])))
	for i := 0; i < samples; i++ {
		a := int32(le32(data[0xc+l:])) + int32(l) + int32(i)*4
		at := int32(le32(data[a:])) + a
		if at < 0 || uint32(at) >= s.sz {
			continue
		}
		sample := data[at:]
		if sample[0] != 1 && sample[0] != 2 {
			continue
		}
		if sample[6] == 1 && s.typ != 0 {
			fulllen := le32(sample[2:])
			length := int32(fulllen-0x10) >> 1
			temp := make([]byte, max(length, 0)+1)
			copy(temp, sample[0x10:0x10+max(length, 0)])

			newval := int8(temp[0])
			p0 := 0x11
			p2 := p0
			sample[p2-1] = byte(newval)
			for k := int32(1); k < length; k++ {
				p2 = p0
				newval += tfibo[temp[k]>>4]
				sample[p2] = byte(newval)
				newval += tfibo[temp[k]&0xf]
				sample[p2+1] = byte(newval)
				p0 = p2 + 2
			}
			sample[p2] = 0
			sample[p2+1] = 0

			fulllen -= 2
			binary.LittleEndian.PutUint32(sample[2:], fulllen)
			if sample[0] == 2 {
				loop := le32(sample[0xc:])
				if loop != 0 {
					for k := range uint32(8) {
						sample[loop+0x10+k] = 0xff
					}
				}
				smplen := fulllen - 0x10
				if loop != 0 {
					smplen = fulllen - 8
				}
				for k := range uint32(8) {
					sample[smplen+k] = 0xff
				}
			}
		}
		invdigit(sample)
	}
}

// invdigit strips the loop header of instruments and terminates the sample.
func invdigit(sample []byte) {
	if sample[0] == 2 {
		if loop := le32(sample[0xc:]); loop != 0 {
			length := int16(le32(sample[2:]) - loop - 0x21)
			if length >= 0 {
				dst := 0x10 + loop
				copy(sample[dst:dst+uint32(length)], sample[dst+8:dst+8+uint32(length)])
			}
			binary.LittleEndian.PutUint32(sample[0xc:], loop-0x10)
		}
		binary.LittleEndian.PutUint32(sample[2:], le32(sample[2:])-0x10)
	}
	length := le32(sample[2:]) - 0x10
	sample[0x10+length-1] = 0
}

// scriptLive creates an entity running prog, linked after the current one
// (START.EXE 0xb8ac).
func (v *VM) scriptLive(prog *scriptData) *scriptLive {
	newent := uint16(v.dernent)
	curent := v.varD5
	nextent := v.ru16(v.atent + uint32(curent) + 4)
	v.wu16(v.atent+uint32(curent)+4, newent)
	v.dernent = v.r16(v.atent + uint32(newent) + 4)
	v.wu16(v.atent+uint32(newent)+4, nextent)
	v.nbent++
	if v.nbent > int16(v.maxent) {
		v.alisError(5)
		return nil
	}
	return v.scriptCreate(prog, newent)
}

// scriptCreate sets up entity newent running prog (START.EXE 0xb906).
func (v *VM) scriptCreate(prog *scriptData, newent uint16) *scriptLive {
	s := &scriptLive{name: prog.name, data: prog}
	d := prog.dataOrg
	w5 := v.r16(d + 0x12)
	prevfin := int32(w5) + int32(v.finent)
	v.finent += uint32(int32(w5 + v.r16(d+0x16) + contextSz))
	s.vaccOff = int16(prevfin - int32(v.finent))
	s.vramOrg = v.finent
	vramLength := int32(v.r16(d + 0x14))
	if s.vramOrg+uint32(vramLength) >= v.ru32(dsDebsprit) {
		v.alisError(0xc)
		return nil
	}
	clear(v.mem[s.vramOrg : s.vramOrg+uint32(vramLength)])

	v.varD7 = int32(newent)

	o := s.vramOrg
	v.w16(o-0x0a, s.vaccOff)
	v.w16(o-0x1c, s.vaccOff)
	v.w16(o-0x1e, s.vaccOff)
	v.w32(o-0x08, int32(d+2+v.ru32(d+2)&0xffffff))
	v.wu16(o-0x10, prog.header.id)
	v.w32(o-0x14, int32(d))
	v.w8(o-0x2e, v.mem[d+5])
	v.w8(o-0x02, 1)
	v.w8(o-0x01, 1)
	v.w8(o-0x04, 0xff)
	v.w16(o-0x1a, -1)
	v.wu16(o-0x0e, newent)
	v.w8(o-0x24, 6)
	v.w16(o-0x18, 0)
	v.w16(o-0x0c, 0)
	v.w16(o-0x22, 0)
	v.w16(o-0x20, 0)
	v.w8(o-0x03, 0)
	v.w8(o-0x25, 0)
	v.w8(o-0x26, 0)
	v.w8(o-0x2a, 0)
	v.w8(o-0x28, 0)
	v.w8(o-0x2c, 0)
	v.w8(o-0x2d, 0)
	v.w8(o-0x2b, 0)
	v.w8(o-0x2f, 0)
	v.w16(o-0x32, 0)
	v.w16(o-0x34, 0)
	v.w8(o-0x30, 0)
	v.w16(o-0x3a, 0)
	v.w32(o-0x3e, 0)

	s.pc = uint32(v.r32(o - 0x08))
	s.pcOrg = s.pc

	v.w32(v.atent+uint32(newent), int32(s.vramOrg))

	v.liveScripts[newent/6] = s
	v.finent += uint32(vramLength)
	return s
}
