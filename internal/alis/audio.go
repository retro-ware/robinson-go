package alis

// Audio (alis audio.c, sys.c sys_audio_callback_S16MSB with
// ALIS_SND_INTERPOLATE_TYPE 1). Four channels: 0-2 PSG effects or samples,
// 3 samples and the film speech queue. Music players are not ported yet.

const (
	chanNone    = 0
	chanDingZap = 1
	chanNoise   = 2
	chanExplode = 3
	chanSample  = 0x80
)

const (
	routNone = iota
	routMV1
	routMV2
)

const (
	hostFreq     = 44100
	fliQueueSize = 16
	fliChunkMax  = 4096
)

type channel struct {
	state, curson int8
	pere          int16
	typ           int
	data          []byte // sample bytes from address on
	addr          uint32 // VM address of data, 0 for film chunks
	played        uint32
	volume        int16
	length        uint32
	freq          int16
	loop          uint8
	dvol, dfreq   int16
}

type fliChunk struct {
	data []byte
	freq int16
}

type fliQueue struct {
	chunks     [fliQueueSize]fliChunk
	head, tail uint8
	played     uint32
}

type audioInstrument struct{ address, unknown uint32 }

type audioState struct {
	musicID  uint16
	channels [4]channel
	tabinst  [128]audioInstrument

	fsound, fmusic uint8

	mupnote          uint32
	muvolume         uint8
	maxvolume        uint16
	muvol, mustate   uint8
	mutempo, mutemp  uint8
	muattac, muchute uint16
	muduree          int16
	dattac, dchute   uint16
	muflag           int32

	rout       int
	fliq       fliQueue
	isrCounter uint32
}

const isrStep = 50 * 65536 / hostFreq

func (v *VM) fliReset() {
	c := &v.aud.channels[3]
	c.typ = chanNone
	c.curson = -0x80
	v.aud.fliq.head, v.aud.fliq.tail = 0, 0
}

// fliAudioPush queues film speech; it returns the bytes accepted.
func (v *VM) fliAudioPush(src []byte, freq int16) int {
	q := &v.aud.fliq
	left := src
	for len(left) != 0 {
		piece := min(len(left), fliChunkMax)
		next := (q.head + 1) & (fliQueueSize - 1)
		if next == q.tail {
			break
		}
		q.chunks[q.head] = fliChunk{append([]byte(nil), left[:piece]...), freq}
		q.head = next
		left = left[piece:]
	}
	c := &v.aud.channels[3]
	if c.typ == chanNone && q.head != q.tail {
		t := q.tail
		c.data, c.addr = q.chunks[t].data, 0
		c.length = uint32(len(c.data))
		c.freq = q.chunks[t].freq
		c.volume = 0x7f
		c.loop = 0
		c.played = 0
		c.curson = 0x7f
		q.tail = (t + 1) & (fliQueueSize - 1)
		c.typ = chanSample
	}
	return len(src) - len(left)
}

func (v *VM) playsample(typ int, addr uint32, freq int8, volume uint8, length uint32, loop uint16, prior int8) {
	a := &v.aud
	cson := int8(127)
	var c *channel
	maxc := 4
	if v.film.playing {
		maxc = 3
	}
	for i := range maxc {
		if prior == a.channels[i].curson {
			c, cson = &a.channels[i], prior
			break
		}
	}
	if c == nil {
		for i := range maxc {
			if a.channels[i].curson < cson {
				c, cson = &a.channels[i], a.channels[i].curson
			}
		}
	}
	if prior >= 0 && cson < 0 && cson != -0x80 || prior < cson {
		return
	}
	if freq < 1 || freq > 0x14 {
		freq = 10
	}
	c.addr = addr
	c.data = v.mem[addr:]
	c.volume = int16(volume)
	c.length = length
	c.freq = int16(freq) * 1000
	c.loop = uint8(loop)
	c.played = 0
	c.typ = typ
	c.curson = prior
}

func (v *VM) runson(typ int, pereson, priorson uint8, volson int16, freqson, longson uint16, dvol, dfreq int16) {
	a := &v.aud
	prior := int8(priorson)
	cson := int8(127)
	var c *channel
	for i := range 3 {
		ch := &a.channels[i]
		if prior < 0 && prior == ch.curson || prior >= 0 && int16(int8(pereson)) == ch.pere && prior == ch.curson {
			c, cson = ch, prior
			break
		}
	}
	if c == nil {
		for i := range 3 {
			if a.channels[i].curson < cson {
				c, cson = &a.channels[i], a.channels[i].curson
			}
		}
	}
	if prior >= 0 && cson < 0 && cson != -0x80 || prior < cson {
		return
	}
	c.curson = prior
	c.typ = typ
	c.freq = int16(freqson)
	c.dfreq = dfreq
	c.volume = volson
	c.dvol = dvol
	c.length = uint32(longson)
	c.pere = int16(int8(pereson))
	c.state = 2
}

func (v *VM) channelOff(i int) {
	c := &v.aud.channels[i]
	c.typ = chanNone
	c.volume, c.freq = 0, 0
	c.curson = -0x80
	c.pere, c.state, c.played = 0, 0, 0
	v.ioCanal(i)
}

func (v *VM) offsound() {
	for i := range 4 {
		v.channelOff(i)
	}
}

// ioCanal programs the PSG for an effect channel (io_canal_opl on the PC).
// TODO(audio): OPL2 synthesis of the effect channels.
func (v *VM) ioCanal(i int) {}

func moved(a, lo, hi uint32, delta int32) uint32 {
	switch {
	case a < lo:
		return a
	case a < hi:
		return 0
	}
	return uint32(int32(a) + delta)
}

func (v *VM) audioRelocate(lo, hi uint32, delta int32) {
	a := &v.aud
	for i := range a.tabinst {
		a.tabinst[i].address = moved(a.tabinst[i].address, lo, hi, delta)
	}
	if a.mupnote >= lo && a.mupnote < hi {
		a.muflag = 0
	}
	a.mupnote = moved(a.mupnote, lo, hi, delta)
	// TODO(audio): relocate the MV1/MV2 player pointers.
	for i := range a.channels {
		c := &a.channels[i]
		if c.addr < lo || c.addr >= v.finmem {
			continue
		}
		if at := moved(c.addr, lo, hi, delta); at != 0 {
			c.addr = at
			c.data = v.mem[at:]
		} else {
			v.channelOff(i)
		}
	}
}

func hermite4(x0, x1, x2, x3, t float32) float32 {
	c0 := x1
	c1 := .5 * (x2 - x0)
	c2 := x0 - 2.5*x1 + 2*x2 - .5*x3
	c3 := .5*(x3-x0) + 1.5*(x1-x2)
	return ((c3*t+c2)*t+c1)*t + c0
}

func smp(d []byte, i int) int32 {
	if i < 0 || i >= len(d) {
		return 0
	}
	return int32(int16(int8(d[i])) * 256)
}

// mixAudio renders one 20 ms buffer of interleaved stereo.
func (v *VM) mixAudio(pcm []int16) {
	clear(pcm)
	a := &v.aud
	n := len(pcm)
	const adv = 2
	if a.muflag > 0 {
		// TODO(audio): MV1/MV2 music.
	}
	if a.fsound == 0 {
		return
	}
	prev := a.isrCounter
	for i := range a.channels {
		a.isrCounter = prev
		c := &a.channels[i]
		switch c.typ {
		case chanSample:
			v.mixSample(i, c, pcm)
		case chanDingZap, chanNoise, chanExplode:
			if i >= 3 {
				break
			}
			for p := 0; p < n; p += adv {
				a.isrCounter += isrStep
				if a.isrCounter >= 0x10000 {
					a.isrCounter -= 0x10000
					v.psgStep(i, c)
				}
				// TODO(audio): OPL_calc output.
			}
		}
	}
}

func (v *VM) psgStep(i int, c *channel) {
	if c.played < c.length {
		c.played++
		vol := int32(c.dvol) + int32(c.volume)
		if vol > -1 {
			vol = min(vol, 0x7fff)
			c.volume = int16(vol)
			f := uint32(int32(c.dfreq) + int32(c.freq))
			if int32(f<<16) > -1 {
				c.freq = int16(f)
				v.ioCanal(i)
				return
			}
		}
	}
	c.typ = chanNone
	c.volume, c.freq = 0, 0
	c.curson = -0x80
	c.state, c.played = 0, 0
	v.ioCanal(i)
}

func (v *VM) mixSample(i int, c *channel, buf []int16) {
	q := &v.aud.fliq
	volratio := int32(c.volume >> 1)
	data, length, loop, played := c.data, c.length, c.loop, c.played
	ratio := uint32(int32(c.freq)<<16) / hostFreq
	start := uint64(played) * uint64(ratio)
	idx := int(start >> 16)
	acc := uint32(start) & 0xffff
	lp := loop > 1
	get := func(k int) int32 {
		if k >= 0 && k < int(length) {
			return smp(data, k)
		}
		return 0
	}
	var x0, x1, x2, x3 int32
	switch {
	case idx-1 >= 0:
		x0 = smp(data, idx-1)
	case lp:
		x0 = smp(data, int(length)-1)
	}
	x1 = smp(data, idx)
	x2 = get(idx + 1)
	if idx+1 >= int(length) && lp {
		x2 = smp(data, 0)
	}
	x3 = get(idx + 2)
	if idx+2 >= int(length) && lp {
		x3 = smp(data, 0)
	}
	for bi := 0; bi < len(buf); {
		add := acc >> 16
		if add > 0 {
			idx += int(add)
			for step := 1; step <= int(add); step++ {
				x0, x1, x2 = x1, x2, x3
				k := idx - int(add) + step + 2
				switch {
				case k < int(length):
					x3 = smp(data, k)
				case lp:
					w := k - int(length)
					x3 = 0
					if w >= 0 && w < int(length) {
						x3 = smp(data, w)
					}
				default:
					x3 = 0
				}
			}
			if idx >= int(length)+1 {
				if loop > 1 {
					loop--
					lp = loop > 1
					played, idx, acc = 0, 0, 0
					x0, x1, x2, x3 = smp(data, int(length)-1), smp(data, 0), get(1), get(2)
				} else if i == 3 && q.head != q.tail {
					q.played++
					t := q.tail
					data = q.chunks[t].data
					length = uint32(len(data))
					c.data, c.addr, c.length = data, 0, length
					c.freq = q.chunks[t].freq
					q.tail = (t + 1) & (fliQueueSize - 1)
					ratio = uint32(int32(c.freq)<<16) / hostFreq
					played, idx, acc = 0, 0, 0
					x0, x1, x2, x3 = x3, get(0), get(1), get(2)
				} else {
					if i == 3 {
						q.played++
					}
					c.typ = chanNone
					c.curson = -0x80
					break
				}
			}
		}
		a0 := x3 - x2 - x0 + x1
		a1 := x0 - x1 - a0
		a2 := x2 - x0
		t := float32(float64(acc)/65536.0) - float32(add)
		r := (float32(a0)*(t*t*t) + float32(a1)*(t*t) + float32(a2)*t + float32(x1)) / 256
		s0 := int16(int32(r * float32(volratio)))
		buf[bi] += s0
		buf[bi+1] += s0
		acc &= 0xffff
		bi += 2
		played++
		acc += ratio
	}
	if &c.data[0] == &data[0] {
		c.played = played
		c.loop = loop
	}
}

// TODO(audio): port music_v1.c, music_v2.c.
func (v *VM) sleepUntilMusicStops()       {}
func (v *VM) sleepUntilMusicStopsAtExit() {}
func (v *VM) mv1Gomusic()                 {}
func (v *VM) mv2Gomusic()                 {}
func (v *VM) mv1Offmusic(arg uint32)      {}
func (v *VM) mv2Offmusic(arg uint32)      {}
