package alis

// The Sound Blaster engine of START.EXE (0x107cf..0x10fd3). Mixer channels
// of 0x40 bytes at 0x1abb8 (4 for effects and film speech, 4 for music)
// play signed 8-bit samples at a 16.16 step through a volume table; the
// block interrupt (0x10edb) starts the DSP on the last mixed buffer of
// 0x400 samples (time constant 190) and mixes the next one (0x10ac1) into
// the other. The port keeps the engine's state in v.mem at the original
// addresses; the two DMA buffers, in DOS memory in the original, are
// sbBuf.

const (
	sbBlockClocks = 0x400 * pitHz * (256 - 190) / 1000000 // 80640
	sbPlayRate    = 1000000.0 / (256 - 190)               // the DSP's samples per second

	dsCard     = 0x18aaa // sound card, 3: Sound Blaster
	dsFmusic   = 0x1609e
	dsFsound   = 0x1609f
	dsChans    = 0x1abb8 // the mixer channels
	dsNSfx     = 0x1abb1 // effect channels
	dsNMusic   = 0x1abb3 // music channels, after them
	dsNChans   = 0x1abb5
	dsMixAcc   = 0x1adb8 // 0x400 words of the mix
	dsMixFirst = 0x1b5b8 // a channel has set dsMixAcc
	dsMixed    = 0x1b5b9 // the mix buffer has a new block
	dsMixAny   = 0x1b5ba // a channel played
	dsMixWas   = 0x1b5bb // ... in the last block
	dsMixRate  = 0x1b5bc // 15000
	dsVolTab   = 0x1b5c6 // 0x60 volumes of 256 scaled samples
	dsClipBase = 0x1b5ca
	dsClipTab  = 0x1b5ce // unsigned sample for a sum from -0x400 on
	dsBlockCnt = 0x1b5d4 // blocks to the next music tick
	dsBlockPer = 0x1b5d6
	dsMusSpeed = 0x1bb0d
	dsPerTab   = 0x1bade // the step for each Amiga period
)

// channel fields
const (
	chState   = 0x00 // 0 off, 1 started, 2 mixed
	chVol     = 0x03 // 0..0x5f
	chPrio    = 0x04
	chOwner   = 0x06 // the script that played it
	chCount   = 0x08 // plays left: 1 once, 2 a next block queued, < 0 loops
	chFreq    = 0x0a
	chStart   = 0x0c
	chLoop    = 0x10
	chLoopEnd = 0x14
	chEnd     = 0x18
	chFrac    = 0x20
	chPos     = 0x22
	chStepF   = 0x26
	chStepI   = 0x28
)

// sbFilmBase is where the port maps the film's two speech buffers (the
// original copies the chunks into film memory).
const sbFilmBase = 0xf0000000

type sbState struct {
	on   bool
	buf  [2][0x400]byte
	play int // the buffer the DSP plays; the mixer writes the other
	film [2][0x1f40 + 4]byte
	out  sbOut
}

// sbInit is the setup of 0x107cf, 0x10c93 and 0x10d6b; 0x10c71 resets the
// channels as 0x37ab does.
func (v *VM) sbInit() {
	v.mem[dsCard] = 3
	v.wu16(dsMixRate, 15000)
	v.wu16(dsNChans, 8)
	v.wu16(dsNSfx, 4)
	v.wu16(dsNMusic, 4)
	t := uint32(memBlockB+0x100) &^ 0xff
	v.wu32(dsVolTab, t)
	v.wu32(dsClipBase, t+0x6000)
	v.wu32(dsPerTab, t+0x6800)
	for vol := range 0x60 {
		for s := range 256 {
			v.mem[t+uint32(vol<<8|s)] = uint8(int16(int8(s)) * int16(vol) >> 6)
		}
	}
	clip := v.ru32(dsClipBase)
	v.wu32(dsClipTab, clip+0x400)
	for i := range 0x800 {
		s := min(max((i-0x400)/2, -0x80), 0x7f)
		v.mem[clip+uint32(i)] = uint8(s + 0x80)
	}
	v.wu16(dsBlockCnt, 1)
	v.wu16(dsBlockPer, 1)
	v.mem[dsMusSpeed] = uint8(15000*256/0x400/50>>4 + 2)
	v.musicInit()
	for i := range v.sb.buf {
		for j := range v.sb.buf[i] {
			v.sb.buf[i][j] = 0x80
		}
	}
	v.sb.play = 0
	v.sbResetChans()
	v.sb.on = true
	v.sbPlayed(v.sb.buf[v.sb.play][:]) // the block the setup starts
}

// sbResetChans is 0x10c71.
func (v *VM) sbResetChans() {
	ch := uint32(dsChans)
	for range v.ru16(dsNChans) {
		v.wu16(ch+chPrio, 0x8000)
		v.wu16(ch+chOwner, 0xffff)
		v.mem[ch] = 0
		ch += 0x40
	}
}

// sbIRQ is the block interrupt 0x10edb.
func (v *VM) sbIRQ() {
	if !v.sb.on {
		return
	}
	v.sbPlayed(v.sb.buf[v.sb.play][:])
	v.sbMix()
	if v.mem[dsMixed] != 0 {
		v.sb.play ^= 1
	}
	n := v.ru16(dsBlockCnt) - 1
	v.wu16(dsBlockCnt, n)
	if n == 0 {
		v.wu16(dsBlockCnt, v.ru16(dsBlockPer))
		v.musicTick()
	}
}

// sbMix is 0x10ac1: the effect channels when sound is on, the music ones
// when music is on, into the buffer the DSP doesn't play.
func (v *VM) sbMix() {
	v.mem[dsMixed], v.mem[dsMixAny], v.mem[dsMixFirst] = 0, 0, 0
	ch := uint32(dsChans)
	for range v.ru16(dsNSfx) {
		if v.mem[dsFsound] != 0 {
			v.sbMixChan(ch)
		}
		ch += 0x40
	}
	for range v.ru16(dsNMusic) {
		if v.mem[dsFmusic] != 0 {
			v.sbMixChan(ch)
		}
		ch += 0x40
	}
	out := &v.sb.buf[v.sb.play^1]
	if v.mem[dsMixAny] == 0 {
		if v.mem[dsMixWas] != 0 {
			for i := range out {
				out[i] = 0x80
			}
			v.mem[dsMixWas] = 0
			v.mem[dsMixed] = 1
		}
		return
	}
	clip := v.ru32(dsClipTab)
	for i := range out {
		out[i] = v.mem[clip+uint32(int32(int16(v.ru16(dsMixAcc+uint32(i)*2))))]
	}
	v.mem[dsMixWas] = v.mem[dsMixAny]
	v.mem[dsMixed] = 1
}

// sbSample reads a sample byte; the film's buffers live outside v.mem.
func (v *VM) sbSample(a uint32) uint8 {
	if a >= sbFilmBase {
		a -= sbFilmBase
		if b := &v.sb.film[a/0x10000&1]; a%0x10000 < uint32(len(b)) {
			return b[a%0x10000]
		}
		return 0
	}
	return v.mem[a]
}

// sbMixChan is 0x10b7e for the channel at ch: 0x400 samples. The first
// channel of a block sets the mix, the others add to it. A first channel
// that ends clears the rest of the mix and one word past it, the flags
// at 0x1b5b8.
func (v *VM) sbMixChan(ch uint32) {
	if v.mem[ch] == 0 {
		return
	}
	v.mem[ch] = 2
	v.mem[dsMixAny] = 1
	tab := v.ru32(dsVolTab) + uint32(v.mem[ch+chVol])<<8
	pos := v.ru32(ch + chPos)
	frac := v.ru16(ch + chFrac)
	first := v.mem[dsMixFirst] == 0
	v.mem[dsMixFirst] = 1
	acc := uint32(dsMixAcc)
	for cx := 0x400; cx != 0; cx-- {
		s := uint16(int16(int8(v.mem[tab&^0xff|uint32(v.sbSample(pos))])))
		if first {
			v.wu16(acc, s)
		} else {
			v.wu16(acc, v.ru16(acc)+s)
		}
		acc += 2
		f := uint32(frac) + uint32(v.ru16(ch+chStepF))
		frac = uint16(f)
		pos += v.ru32(ch+chStepI) + f>>16
		if pos < v.ru32(ch+chEnd) {
			continue
		}
		c := int16(v.ru16(ch + chCount))
		n := c - 1
		v.wu16(ch+chCount, uint16(n))
		if n == 0 {
			v.mem[ch] = 0
			if first {
				for range cx {
					v.wu16(acc, 0)
					acc += 2
				}
			}
			return
		}
		if n <= 0 || c == -0x8000 { // jg after dec
			v.wu16(ch+chCount, uint16(c))
		}
		v.wu32(ch+chEnd, v.ru32(ch+chLoopEnd))
		pos = v.ru32(ch + chLoop)
		frac = 0
	}
	v.wu16(ch+chFrac, frac)
	v.wu32(ch+chPos, pos)
}

// sbStart is 0x1088e: channel ch plays from its start at its frequency.
func (v *VM) sbStart(ch uint32) {
	if v.mem[dsCard] == 0 {
		return
	}
	v.mem[ch] = 0
	v.sbSetStep(ch)
	v.wu32(ch+chPos, v.ru32(ch+chStart))
	v.wu16(ch+chFrac, 0)
	v.mem[ch+1] = 1
	v.mem[ch] = 1
}

func (v *VM) sbSetStep(ch uint32) {
	step := uint32(v.ru16(ch+chFreq)) << 16 / uint32(v.ru16(dsMixRate))
	v.wu16(ch+chStepF, uint16(step))
	v.wu32(ch+chStepI, step>>16)
}

// sbStop is 0x108e4: effect channel n off, all of them for n < 0.
func (v *VM) sbStop(n int16) {
	if v.mem[dsCard] == 0 {
		return
	}
	if int8(n) < 0 {
		ch := uint32(dsChans)
		for range v.ru16(dsNSfx) {
			v.mem[ch] = 0
			ch += 0x40
		}
		return
	}
	if n < int16(v.ru16(dsNSfx)) {
		v.mem[dsChans+uint32(uint16(n)*0x40)] = 0
	}
}

// sbAlloc is 0x9197: an effect channel for owner at priority prio (< 0:
// the channel already at that priority), the one already playing it, a
// free one, or the lowest priority one if prio is at least as high; 0
// for none.
func (v *VM) sbAlloc(owner, prio uint16) uint32 {
	n := v.ru16(dsNSfx)
	take := func(ch uint32) uint32 {
		v.wu16(ch+chOwner, owner)
		v.wu16(ch+chPrio, prio)
		return ch
	}
	ch := uint32(dsChans)
	if int16(prio) < 0 {
		for range n {
			if v.ru16(ch+chPrio) == prio {
				return take(ch)
			}
			ch += 0x40
		}
	} else {
		for range n {
			if v.mem[ch] != 0 && v.ru16(ch+chOwner) == owner && v.ru16(ch+chPrio) == prio {
				return take(ch)
			}
			ch += 0x40
		}
	}
	ch = dsChans
	low, ax := ch, int16(0x7fff)
	for range n {
		if v.mem[ch] == 0 {
			return take(ch)
		}
		if p := int16(v.ru16(ch + chPrio)); ax > p {
			low, ax = ch, p
		}
		ch += 0x40
	}
	if int16(prio) >= 0 && ax < 0 && ax != -0x8000 {
		return 0
	}
	if int16(prio) < ax {
		return 0
	}
	return take(low)
}

// sbFilmChan is 0x109e7: the film's channel, a free effect channel or the
// first one.
func (v *VM) sbFilmChan() uint32 {
	ch := uint32(dsChans)
	for range v.ru16(dsNSfx) {
		if v.mem[ch] == 0 {
			v.wu16(ch+chPrio, 0x7f)
			return ch
		}
		ch += 0x40
	}
	ch = dsChans
	v.mem[ch] = 0
	v.wu16(ch+chPrio, 0x7f)
	return ch
}

// sbFilmPush is 0x10941: n bytes of speech at a on channel ch at freq; it
// spins until the mixer has started them. False when sound is off.
func (v *VM) sbFilmPush(ch, a, n uint32, freq uint16) bool {
	if v.mem[dsFsound] == 0 || v.mem[dsCard] == 0 {
		return false
	}
	if v.mem[ch] == 0 {
		v.wu32(ch+chStart, a)
		v.wu16(ch+chFreq, freq)
		v.wu32(ch+chEnd, a+n)
		v.wu16(ch+chCount, 1)
		v.wu32(ch+chPos, a)
		v.wu16(ch+chFrac, 0)
		v.sbSetStep(ch)
		v.mem[ch+chVol] = 0x5f
		v.mem[ch+1] = 1
		v.mem[ch] = 1
		for v.mem[ch] == 1 && v.state != stateStopped {
			v.waitSB()
		}
		return true
	}
	v.wu32(ch+chLoop, a)
	v.wu32(ch+chLoopEnd, a+n)
	v.wu16(ch+chCount, 2)
	for v.ru16(ch+chCount) == 2 && v.state != stateStopped {
		v.waitSB()
	}
	return true
}
