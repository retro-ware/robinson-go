package alis

// The music player of START.EXE (0x10a14..0x123e7): a 4 voice module
// player on mixer channels 4..7 with the samples of cinstru's table. It
// works on the original's DS variables, and keeps its register quirks:
// the slides' speed is cleared before use, Bxx/Dxx always restart at row
// 0, 9xx and E6x do nothing, and the tremolo writes its delta as the
// volume.

const (
	dsSamples  = 0x160be // 32 entries: sample address, cinstru's third operand
	dsSong     = 0x160a8 // the module, after its 6 byte header
	dsMusVol   = 0x160a3
	dsMusFade  = 0x160ac // fade in ticks
	dsMusHold  = 0x160b0
	dsMusOut   = 0x160ae // fade out ticks
	dsMusAdlib = 0x1abb0 // the song uses FM instruments
	dsVoices   = 0x1b75c // 4 voices of 0x40 bytes; +0x3a their channel
	dsPlaying  = 0x1bae5

	dsOrders   = 0x1b85c // 0x80 pattern numbers
	dsSmpInfo  = 0x1b8dc // 32 x 0x10: start, loop, end, volume, -, finetune
	dsMusBase  = 0x1badc // the volume the fades start from
	dsMusMVol  = 0x1badd // the volume the voices are scaled by (64 = full)
	dsMusFM    = 0x1bae3
	dsMusOutOn = 0x1bae4
	dsOutCnt   = 0x1bae8
	dsOutLen   = 0x1baea
	dsInCnt    = 0x1baec
	dsInLen    = 0x1baee
	dsHoldCnt  = 0x1baf0
	dsSpeed    = 0x1baf2
	dsTick     = 0x1baf3
	dsPos      = 0x1baf4
	dsSongLen  = 0x1baf5
	dsBreakRow = 0x1baf6
	dsBreak    = 0x1baf7
	dsJumpRow  = 0x1baf8
	dsFineMask = 0x1baf9
	dsDelayNew = 0x1bafa
	dsDelay    = 0x1bafb
	dsRow      = 0x1bb01
	dsPattern  = 0x1bb05
	dsSongAddr = 0x1bb09
	dsFunkTab  = 0x1bb10
	dsSineTab  = 0x1bb20
	dsPeriods  = 0x1bb40 // 16 finetunes x 36 periods
)

// Voice fields.
const (
	vcNote     = 0x00 // the row's 4 bytes: +0/+1 sample and period, +2 sample and effect, +3 parameter
	vcStart    = 0x04
	vcEnd      = 0x08
	vcLoop     = 0x0c
	vcPeriod   = 0x14
	vcFine     = 0x16
	vcVol      = 0x17
	vcPortaDn  = 0x1a
	vcPortaSpd = 0x1b
	vcPortaTo  = 0x1c
	vcVib      = 0x1e
	vcVibPos   = 0x1f
	vcTrem     = 0x20
	vcTremPos  = 0x21
	vcWaves    = 0x22
	vcFunk     = 0x23
	vcFunkAcc  = 0x27
	vcFunkPos  = 0x28
	vcEnd2     = 0x2c
	vcNew      = 0x36
	vcChan     = 0x3a
)

// musicInit is 0x114f3: the voices' channels and the period table
// (0x11554).
func (v *VM) musicInit() {
	ch := uint32(dsChans + 4*0x40)
	for i := range uint32(4) {
		v.wu32(dsVoices+i*0x40+0x3a, ch+i*0x40)
	}
	t := v.ru32(dsPerTab)
	v.wu32(t, 0)
	for p := uint32(1); p < 0x400; p++ {
		v.wu32(t+p*4, 0x300d2*0x50/p)
	}
}

// musicStart is 0x10a14.
func (v *VM) musicStart() {
	if v.mem[dsFmusic] == 0 || v.mem[dsCard] == 0 {
		return
	}
	n := 0
	for i := range uint32(32) {
		if a := v.ru32(dsSamples + i*8); a != 0 && v.mem[a-0x10] == 6 {
			n++
		}
	}
	v.mem[dsMusAdlib] = 0
	if n >= 3 {
		v.mem[dsMusAdlib] = 1
	}
	v.musicPlay(v.ru32(dsSong), v.mem[dsMusAdlib])
}

// musicPlay is 0x1158e.
func (v *VM) musicPlay(song uint32, fm uint8) {
	v.mem[dsPlaying] = 0
	v.wu32(dsSongAddr, song)
	v.mem[dsMusFM] = fm
	v.mem[dsSongLen] = v.mem[song]
	v.mem[dsPos] = 0
	v.mem[dsTick] = 0
	v.mem[dsSpeed] = uint8(uint16(v.mem[dsMusSpeed]) * 6 >> 4)
	v.wu16(dsInCnt, v.ru16(dsMusFade))
	v.wu16(dsInLen, v.ru16(dsMusFade))
	v.wu16(dsOutCnt, v.ru16(dsMusOut))
	v.wu16(dsOutLen, v.ru16(dsMusOut))
	v.wu16(dsHoldCnt, v.ru16(dsMusHold))
	v.mem[dsMusOutOn] = 0
	v.mem[dsMusMVol] = v.mem[dsMusVol] >> 1
	v.mem[dsMusBase] = v.mem[dsMusVol] >> 1
	copy(v.mem[dsOrders:dsOrders+0x80], v.mem[song+2:])
	v.musicPattern(v.mem[dsOrders])
	v.musicSamples()
	v.mem[dsPlaying] = 1
}

// musicPattern points the row at the start of pattern p.
func (v *VM) musicPattern(p uint8) {
	a := uint32(p)<<10 + v.ru32(dsSongAddr) + 0x86
	v.wu32(dsRow, a)
	v.wu32(dsPattern, a)
}

// musicSamples is 0x116fe: the sample table from cinstru's samples.
func (v *VM) musicSamples() {
	for i := range uint32(31) {
		e := dsSmpInfo + 0x10 + i*0x10
		s := v.ru32(dsSamples + i*8)
		if int32(s) <= 0 {
			v.wu32(e, 0)
			v.wu32(e+4, 0)
			v.wu32(e+8, 0)
			continue
		}
		v.wu32(e, s)
		loop := uint32(0xffffffff)
		if l := v.ru32(s - 8); l != 0 {
			loop = l + s
		}
		v.wu32(e+4, loop)
		end := int32(v.ru32(s - 4))
		if end > 0 {
			if n := int32(v.ru32(s-0xe)) - 0x10; end >= n {
				end = n
			}
		} else {
			end = int32(v.ru32(s-0xe)) - 0x20
		}
		v.wu32(e+8, uint32(end)+s)
		v.mem[e+0xc] = 0x40
		v.mem[e+0xe] = 0
		v.mem[e+0xd] = 0x40
		v.mem[e+0xf] = 0
	}
}

// musicFade is 0x10a8a/0x11697: with a song playing, fade it out over
// ticks ticks, or stop it at 0.
func (v *VM) musicFade(ticks uint16) {
	if v.mem[dsCard] == 0 || v.mem[dsPlaying] == 0 {
		return
	}
	if ticks == 0 {
		v.musicStop()
		return
	}
	v.wu16(dsOutCnt, ticks)
	v.wu16(dsOutLen, ticks)
	v.mem[dsMusBase] = v.mem[dsMusMVol]
	v.mem[dsMusOutOn] = 1
}

// musicStop is 0x116c8.
func (v *VM) musicStop() {
	v.mem[dsPlaying] = 0
	v.mem[dsMusOutOn] = 0
	v.mem[dsMusFM] = 0
	for i := range uint32(4) {
		v.mem[v.ru32(dsVoices+i*0x40+vcChan)] = 0
	}
}

// musicVolume is 0x118ab: the fade in, the hold and the fade out; false
// when the song has ended.
func (v *VM) musicVolume() bool {
	base := uint32(v.mem[dsMusBase])
	var al uint8
	switch {
	case v.mem[dsMusOutOn] == 0 && v.ru16(dsInCnt) != 0:
		n := v.ru16(dsInCnt) - 1
		v.wu16(dsInCnt, n)
		al = -(uint8(uint32(n)*base/uint32(v.ru16(dsInLen))) - v.mem[dsMusBase])
	case v.mem[dsMusOutOn] == 0 && v.ru16(dsHoldCnt) != 0:
		v.wu16(dsHoldCnt, v.ru16(dsHoldCnt)-1)
		return true
	default:
		n := v.ru16(dsOutCnt)
		if n == 0 {
			v.musicStop()
			return false
		}
		n--
		v.wu16(dsOutCnt, n)
		al = uint8(uint32(n) * base / uint32(v.ru16(dsOutLen)))
	}
	v.mem[dsMusMVol] = al + 1
	return true
}

// musicTick is 0x117c7, run every few blocks.
func (v *VM) musicTick() {
	if v.mem[dsPlaying] == 0 {
		return
	}
	v.musicVolume()
	row := v.ru32(dsRow)
	if v.mem[dsSpeed] == 0 {
		v.musicStop()
		return
	}
	v.mem[dsTick]++
	if v.mem[dsSpeed] > v.mem[dsTick] {
		// 0x1182d; an arpeggio's unbalanced pop returns from it early
		for i := range uint32(4) {
			if !v.musicEffect(dsVoices + i*0x40) {
				break
			}
		}
		v.musicNextPattern()
		return
	}
	v.mem[dsTick] = 0
	for i := range uint32(4) {
		v.musicRow(dsVoices+i*0x40, row+i*4)
	}
	v.musicNextRow()
}

// musicNextRow is 0x11b66.
func (v *VM) musicNextRow() {
	v.wu32(dsRow, v.ru32(dsRow)+0x10)
	if d := v.mem[dsDelayNew]; d != 0 {
		v.mem[dsDelay] = d
		v.mem[dsDelayNew] = 0
	}
	if v.mem[dsDelay] != 0 {
		v.mem[dsDelay]--
		if v.mem[dsDelay] != 0 {
			v.wu32(dsRow, v.ru32(dsRow)-0x10)
		}
	}
	if v.mem[dsJumpRow] != 0 {
		v.mem[dsJumpRow] = 1
		v.wu32(dsRow, uint32(v.mem[dsBreakRow])<<4+v.ru32(dsPattern))
	}
	if v.ru32(dsRow)-v.ru32(dsPattern) >= 0x400 {
		v.musicNewPattern()
	}
	v.musicNextPattern()
}

// musicNextPattern is 0x11c41: a break or jump moves to the next order.
func (v *VM) musicNextPattern() {
	for v.mem[dsBreak] != 0 {
		v.musicNewPattern()
	}
}

// musicNewPattern is 0x11bd5. The break row is set and then overwritten
// by the pattern's start.
func (v *VM) musicNewPattern() {
	v.wu32(dsRow, uint32(v.mem[dsBreakRow])<<4+v.ru32(dsPattern))
	v.mem[dsBreakRow] = 0
	v.mem[dsBreak] = 0
	p := (v.mem[dsPos] + 1) & 0x7f
	if p == v.mem[dsSongLen] {
		p = 0
	}
	v.mem[dsPos] = p
	v.musicPattern(v.mem[dsOrders+uint32(p)])
}

func (v *VM) vcSwap(vc, off uint32) uint16 {
	return uint16(v.mem[vc+off])<<8 | uint16(v.mem[vc+off+1])
}

// musicChanVol is the voice volume scaled to the channel (0x10d19).
func (v *VM) musicChanVol(vc uint32, al uint8) {
	v.mem[v.ru32(vc+vcChan)+chVol] = uint8(uint16(al) * uint16(v.mem[dsMusMVol]) >> 6)
}

// musicStep sets the channel's step for period p (0x10d1e).
func (v *VM) musicStep(vc uint32, p uint16) {
	v.wu32(v.ru32(vc+vcChan)+chStepF, v.ru32(v.ru32(dsPerTab)+uint32(p)*4))
}

// musicChanStart is 0x10d23.
func (v *VM) musicChanStart(ch, start, end, loop uint32) {
	v.mem[ch] = 0
	if start == 0 {
		return
	}
	v.wu32(ch+0xc, start)
	v.wu32(ch+0x10, loop)
	v.wu16(ch+8, 0xffff)
	if loop == 0xffffffff {
		v.wu16(ch+8, 1)
	}
	v.wu32(ch+0x22, start)
	v.wu16(ch+0x20, 0)
	v.wu32(ch+0x18, end)
	v.wu32(ch+0x14, end)
	v.wu32(ch+0x1c, end)
}

// periodRow is the row of finetune f in the period table.
func (v *VM) periodRow(f uint8) uint32 { return dsPeriods + uint32(f)*0x48 }

// musicRow is 0x119cb: the voice's new row from the pattern at row.
func (v *VM) musicRow(vc, row uint32) {
	if v.mem[dsMusFM] == 1 {
		v.fatal("music: the FM instrument player (START.EXE 0x1194a) is not ported")
		return
	}
	if v.ru16(vc) == 0 && v.mem[vc+2] == 0 {
		v.musicStep(vc, v.ru16(vc+vcPeriod))
	}
	copy(v.mem[vc:vc+4], v.mem[row:row+4])
	if smp := v.mem[vc+2]>>4 | v.mem[vc]&0xf0; smp != 0 {
		e := dsSmpInfo + uint32(smp)<<4
		v.wu32(vc+vcStart, v.ru32(e))
		v.wu32(vc+vcEnd, v.ru32(e+8))
		v.wu32(vc+vcEnd2, v.ru32(e+8))
		v.mem[vc+vcFine] = v.mem[e+0xe]
		v.mem[vc+vcVol] = v.mem[e+0xc]
		v.musicChanVol(vc, v.mem[e+0xc])
		v.wu32(vc+vcLoop, v.ru32(e+4))
		v.wu32(vc+vcFunkPos, 0)
	}
	if v.vcSwap(vc, 0)&0xfff == 0 {
		v.musicRowEffect(vc)
		return
	}
	fx := v.vcSwap(vc, 2)
	switch {
	case fx&0xff0 == 0xe50:
		v.mem[vc+vcFine] = v.mem[vc+3] & 0xf
	case fx>>8&0xf == 3 || fx>>8&0xf == 5:
		v.musicPortaTarget(vc)
		v.musicRowEffect(vc)
		return
	case fx>>8&0xf == 9:
		v.musicRowEffect(vc)
	}
	note := v.vcSwap(vc, 0) & 0xfff
	i := uint32(0)
	for ; i < 0x24; i++ {
		if note >= v.ru16(dsPeriods+i*2) {
			break
		}
	}
	if i == 0x24 {
		i = 0x23
	}
	v.wu16(vc+vcPeriod, v.ru16(v.periodRow(v.mem[vc+vcFine])+i*2))
	if v.vcSwap(vc, 2)&0xff0 == 0xed0 {
		v.musicRowEffect(vc)
		return
	}
	v.musicTrigger(vc)
	v.musicRowEffect(vc)
}

// musicTrigger starts the voice's sample (0x11b0c).
func (v *VM) musicTrigger(vc uint32) {
	if v.mem[vc+vcWaves]&2 == 0 {
		v.mem[vc+vcVibPos] = 0
	}
	if v.mem[vc+vcWaves]&0x20 == 0 {
		v.mem[vc+vcTremPos] = 0
	}
	v.mem[vc+vcNew] = 1
	ch := v.ru32(vc + vcChan)
	v.musicChanStart(ch, v.ru32(vc+vcStart), v.ru32(vc+vcEnd), v.ru32(vc+vcLoop))
	v.musicStep(vc, v.ru16(vc+vcPeriod))
	v.musicChanVol(vc, v.mem[vc+vcVol])
	v.mem[ch] = 1
}

// musicPortaTarget is 0x11e14.
func (v *VM) musicPortaTarget(vc uint32) {
	note := v.vcSwap(vc, 0) & 0xfff
	t := v.periodRow(v.mem[vc+vcFine])
	i := uint32(0)
	for ; ; i += 2 {
		if note >= v.ru16(t+i) {
			break
		}
		if i+2 == 0x48 {
			i = 0x46
			break
		}
	}
	if v.mem[vc+vcFine]&8 != 0 && i != 0 {
		i -= 2
	}
	to := v.ru16(t + i)
	v.wu16(vc+vcPortaTo, to)
	p := v.ru16(vc + vcPeriod)
	v.mem[vc+vcPortaDn] = 0
	switch {
	case p == to:
		v.wu16(vc+vcPortaTo, 0)
	case p > to:
		v.mem[vc+vcPortaDn] = 1
	}
}

// musicRowEffect is 0x121af: the effects of a row's first tick.
func (v *VM) musicRowEffect(vc uint32) {
	v.musicFunk(vc)
	p := v.mem[vc+3]
	switch v.mem[vc+2] & 0xf {
	case 9: // 0x12085: the sample offset is never reached
	case 0xb: // 0x120f2
		v.mem[dsPos] = p - 1
		v.musicPattern(v.mem[dsOrders+uint32(p-1)])
		v.mem[dsBreakRow] = 0
		v.mem[dsBreak] = 1
	case 0xd: // 0x12166
		r := (p>>4)*10 + p&0xf
		if int8(r) >= 0x3f {
			r = 0
		}
		v.mem[dsBreakRow] = r
		v.mem[dsBreak] = 1
	case 0xe:
		v.musicExtended(vc)
	case 0xf: // 0x1218a
		v.mem[dsTick] = 0
		v.mem[dsSpeed] = uint8(uint16(v.mem[dsMusSpeed]) * uint16(p) >> 4)
	case 0xc: // 0x12130: also sets the volume of the sample numbered by the low nibble
		p = min(p, 0x40)
		v.mem[vc+vcVol] = p
		v.mem[dsSmpInfo+0xc+uint32(v.mem[vc+2]&0xf0)] = p
		v.musicChanVol(vc, p)
	default:
		v.musicStep(vc, v.ru16(vc+vcPeriod))
	}
}

// musicExtended is 0x121f2: the Exy effects.
func (v *VM) musicExtended(vc uint32) {
	p := v.mem[vc+3]
	tick := v.mem[dsTick]
	switch p >> 4 {
	case 1: // 0x11d6d
		if tick == 0 {
			v.mem[dsFineMask] = 0xf
			v.musicSlideUp(vc)
		}
	case 2: // 0x11dc0
		if tick == 0 {
			v.mem[dsFineMask] = 0xf
			v.musicSlideDown(vc)
		}
	case 3:
		v.mem[vc+vcFunk] = v.mem[vc+vcFunk]&0xf0 | p&0xf
	case 4:
		v.mem[vc+vcWaves] = v.mem[vc+vcWaves]&0xf0 | p&0xf
	case 5:
		v.mem[vc+vcFine] = p & 0xf
	case 7:
		v.mem[vc+vcWaves] = v.mem[vc+vcWaves]&0xf | p<<4
	case 9: // 0x122b6
		n := p & 0xf
		if n == 0 {
			return
		}
		if tick == 0 && v.vcSwap(vc, 0)&0xfff != 0 {
			return
		}
		if tick%n == 0 {
			v.musicRetrigger(vc)
		}
	case 0xa: // 0x1232a
		if tick == 0 {
			v.musicVolUp(vc, p&0xf)
		}
	case 0xb: // 0x1233f
		if tick == 0 {
			v.musicVolDown(vc, p&0xf)
		}
	case 0xc: // 0x12353
		if tick == p&0xf {
			v.mem[vc+vcVol] = 0
			v.musicChanVol(vc, 0)
		}
	case 0xd: // 0x1237a
		if tick == p&0xf && v.ru16(vc) != 0 {
			v.musicRetrigger(vc)
		}
	case 0xf: // 0x12398
		if tick == 0 {
			v.mem[vc+vcFunk] = v.mem[vc+vcFunk]&0xf | p<<4
			if p&0xf != 0 {
				v.musicFunk(vc)
			}
		}
	}
}

// musicRetrigger is 0x122eb. Only the step's low word is loaded: the high
// word is the sample start's.
func (v *VM) musicRetrigger(vc uint32) {
	ch := v.ru32(vc + vcChan)
	start := v.ru32(vc + vcStart)
	v.musicChanStart(ch, start, v.ru32(vc+vcEnd), v.ru32(vc+vcLoop))
	pt := v.ru32(dsPerTab) + uint32(v.ru16(vc+vcPeriod)<<2)
	v.wu32(ch+chStepF, start&0xffff0000|uint32(v.ru16(pt)))
	v.musicChanVol(vc, v.mem[vc+vcVol])
	v.mem[ch] = 1
}

// musicFunk is 0x123b6: EFx's counter (it inverts no sample bytes).
func (v *VM) musicFunk(vc uint32) {
	n := v.mem[vc+vcFunk] >> 4
	if n == 0 {
		return
	}
	v.mem[vc+vcFunkAcc] += v.mem[dsFunkTab+uint32(n)]
	if v.mem[vc+vcFunkAcc]&0x80 == 0 {
		return
	}
	v.mem[vc+vcFunkAcc] = 0
	p := v.ru32(vc+vcFunkPos) + 1
	if p >= v.ru32(vc+vcEnd) {
		p = v.ru32(vc + vcLoop)
	}
	v.wu32(vc+vcFunkPos, p)
}

// musicEffect is 0x11c4b: the effects of the other ticks; false when an
// arpeggio ends the tick's voice loop.
func (v *VM) musicEffect(vc uint32) bool {
	v.musicFunk(vc)
	fx := v.vcSwap(vc, 2) & 0xfff
	if fx == 0 {
		v.musicStep(vc, v.ru16(vc+vcPeriod))
		return true
	}
	switch fx >> 8 {
	case 0xe:
		v.musicExtended(vc)
	case 0:
		return v.musicArpeggio(vc)
	case 1:
		v.musicSlideUp(vc)
	case 2:
		v.musicSlideDown(vc)
	case 3:
		if p := v.mem[vc+3]; p != 0 {
			v.mem[vc+vcPortaSpd] = p
			v.mem[vc+3] = 0
		}
		v.musicPorta(vc)
	case 4:
		if p := v.mem[vc+3]; p != 0 {
			a := v.mem[vc+vcVib]
			if p&0xf != 0 {
				a = a&0xf0 | p&0xf
			}
			if p&0xf0 != 0 {
				a = a&0xf | p&0xf0
			}
			v.mem[vc+vcVib] = a
		}
		v.musicVibrato(vc)
	case 5:
		v.musicPorta(vc)
		v.musicVolSlide(vc)
	case 6:
		v.musicVibrato(vc)
		v.musicVolSlide(vc)
	default:
		v.musicStep(vc, v.ru16(vc+vcPeriod))
		switch fx >> 8 {
		case 7:
			v.musicTremolo(vc)
		case 0xa:
			v.musicVolSlide(vc)
		}
	}
	return true
}

// musicArpeggio is 0x11cf3. On the third ticks it pops the return address
// it never pushed, which ends the voice loop.
func (v *VM) musicArpeggio(vc uint32) bool {
	var n uint32
	switch v.mem[dsTick] % 3 {
	case 0:
		v.musicStep(vc, v.ru16(vc+vcPeriod))
		return false
	case 1:
		n = uint32(v.mem[vc+3] >> 4)
	case 2:
		n = uint32(v.mem[vc+3] & 0xf)
	}
	t := v.periodRow(v.mem[vc+vcFine])
	p := v.ru16(vc + vcPeriod)
	for range 0x24 {
		if p >= v.ru16(t) {
			v.musicStep(vc, v.ru16(t+n*2))
			return true
		}
		t += 2
	}
	return true
}

// musicSlideUp is 0x11d7d: ebx is cleared after the parameter is loaded,
// so the period only gets clamped.
func (v *VM) musicSlideUp(vc uint32) {
	v.mem[dsFineMask] = 0xff
	if int16(v.ru16(vc+vcPeriod)&0xfff) <= 0x71 {
		v.wu16(vc+vcPeriod, 0x71)
	}
	v.musicStep(vc, v.ru16(vc+vcPeriod))
}

// musicSlideDown is 0x11dd0.
func (v *VM) musicSlideDown(vc uint32) {
	v.mem[dsFineMask] = 0xff
	if int16(v.ru16(vc+vcPeriod)&0xfff) >= 0x358 {
		v.wu16(vc+vcPeriod, 0x358)
	}
	v.musicStep(vc, v.ru16(vc+vcPeriod))
}

// musicPorta is 0x11e97, with the speed cleared like the slides'.
func (v *VM) musicPorta(vc uint32) {
	to := v.ru16(vc + vcPortaTo)
	if to == 0 {
		return
	}
	p := v.ru16(vc + vcPeriod)
	if v.mem[vc+vcPortaDn] == 0 {
		if int16(to) <= int16(p) {
			v.wu16(vc+vcPeriod, to)
			v.wu16(vc+vcPortaTo, 0)
		}
	} else if int16(to) >= int16(p) {
		v.wu16(vc+vcPeriod, to)
		v.wu16(vc+vcPortaTo, 0)
	}
	p = v.ru16(vc + vcPeriod)
	if v.mem[vc+vcFunk]&0xf != 0 { // glissando
		t := v.periodRow(v.mem[vc+vcFine])
		i := uint32(0)
		for ; ; i += 2 {
			if int16(p) >= int16(v.ru16(t+i)) {
				break
			}
			if i+2 == 0x48 {
				i = 0x46
				break
			}
		}
		p = v.ru16(t + i)
	}
	v.musicStep(vc, p)
}

// musicWave is the waveform value at position pos (0x11f5d/0x12008);
// ramp is the vibrato position, which the tremolo's ramp also tests.
func (v *VM) musicWave(wave, pos, ramp uint8) uint8 {
	i := pos >> 2 & 0x1f
	switch wave & 3 {
	case 0:
		return v.mem[dsSineTab+uint32(i)]
	case 1:
		if int8(ramp) < 0 {
			return 0xff - i<<3
		}
		return i << 3
	}
	return 0xff
}

// musicVibrato is 0x11f5d.
func (v *VM) musicVibrato(vc uint32) {
	pos := v.mem[vc+vcVibPos]
	a := v.musicWave(v.mem[vc+vcWaves], pos, pos)
	d := uint16(a) * uint16(v.mem[vc+vcVib]&0xf) >> 7
	p := v.ru16(vc + vcPeriod)
	if int8(pos) >= 0 {
		p += d
	} else {
		p -= d
	}
	v.musicStep(vc, p)
	v.mem[vc+vcVibPos] += v.mem[vc+vcVib] >> 2 & 0x3c
}

// musicTremolo is 0x11fe5. The clamped volume is computed and dropped:
// the channel gets the delta.
func (v *VM) musicTremolo(vc uint32) {
	if p := v.mem[vc+3]; p != 0 {
		a := v.mem[vc+vcTrem]
		if p&0xf != 0 {
			a = a&0xf0 | p&0xf
		}
		if p&0xf0 != 0 {
			a = a&0xf | p&0xf0
		}
		v.mem[vc+vcTrem] = a
	}
	a := v.musicWave(v.mem[vc+vcWaves]>>4, v.mem[vc+vcTremPos], v.mem[vc+vcVibPos])
	d := uint16(a) * uint16(v.mem[vc+vcTrem]&0xf) >> 7
	v.musicChanVol(vc, uint8(d))
	v.mem[vc+vcTremPos] += v.mem[vc+vcTrem] >> 2 & 0x3c
}

// musicVolSlide is 0x1209b.
func (v *VM) musicVolSlide(vc uint32) {
	p := v.mem[vc+3]
	if p>>4 != 0 {
		v.musicVolUp(vc, p>>4)
	} else {
		v.musicVolDown(vc, p&0xf)
	}
}

// musicVolUp is 0x120a6.
func (v *VM) musicVolUp(vc uint32, n uint8) {
	vol := v.mem[vc+vcVol] + n
	if int8(vol) > 0x40 {
		vol = 0x40
	}
	v.mem[vc+vcVol] = vol
	v.musicChanVol(vc, vol)
}

// musicVolDown is 0x120cf.
func (v *VM) musicVolDown(vc uint32, n uint8) {
	vol := v.mem[vc+vcVol] - n
	if int8(vol) <= 0 {
		vol = 0
	}
	v.mem[vc+vcVol] = vol
	v.musicChanVol(vc, vol)
}
