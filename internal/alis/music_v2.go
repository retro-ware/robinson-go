package alis

// MV2 music, the sample (mutype) player (alis music_v2.c). The chip (YM) and
// OPL2 players are not ported: Robinson's Requiem's songs use samples only.

const (
	muBufLen  = 1024 // kMuBufLen
	rdiffLen  = 4096 // kRDiffLen
	tabfrqLen = 0x358
)

type mv2Voice struct {
	freqsam             uint16
	startsam1, longsam1 uint32
	volsam              uint16
	startsam2, longsam2 uint32
	value               int16
	typ, delta          uint8
	loopsam, frqto      uint16
	sample              uint32
}

type mv2State struct {
	voices                              [4]mv2Voice
	tabfrq                              [tabfrqLen]uint32
	defvol                              [32]uint8
	prevmufreq, prevmuvol               int16
	mutype, muchip, muopl2              uint16
	muptr, mumax, mucnt, mubufa, mubufc uint16
	muspeed, muvolgen, mubreak          uint16
	frqmod                              uint32
	samples                             uint16

	mutaloop uint16
	smpidx   uint32          // audio.smpidx: frames of the last tick not yet output
	left     [muBufLen]int16 // audio.muadresse
	right    [muBufLen]int16 // muright
	rdiff    [rdiffLen]int32 // mrdiff
}

func (v *VM) mv2Gomusic() {
	a, m := &v.aud, &v.mv2
	a.muflag = 0
	var chipinst, opl2inst int
	for i := range 0x20 {
		if at := a.tabinst[i].address; at != 0 {
			switch v.mem[at-0x10] {
			case 5:
				chipinst++
			case 6:
				opl2inst++
			}
		}
	}
	m.muchip = b2u16(chipinst > 2)
	m.muopl2 = b2u16(opl2inst > 2)
	m.mutype = b2u16(chipinst <= 2)
	a.muvol = a.muvolume>>1 + 1
	a.mutemp = uint8(uint32(a.mutempo) * 6 >> 5)
	if m.muopl2 != 0 || m.mutype == 0 {
		// TODO(audio): the OPL2 and chip players.
		return
	}
	m.voices = [4]mv2Voice{}
	m.prevmuvol, m.prevmufreq = 0, 0
	m.calculfrq()
	if m.prevmuvol == 0 {
		m.prevmuvol = -1
	}
	a.rout = routMV2
	for i := range m.defvol {
		m.defvol[i] = 0x40
	}
	m.mumax = uint16(v.mem[a.mupnote])
	a.mupnote += 2
	m.mubreak, m.muptr, m.mucnt = 0, 0, 0
	m.mubufa, m.mubufc = a.muattac, a.muchute
	m.muspeed = 1
	m.smpidx = 0
	a.muflag = 1
}

func b2u16(b bool) uint16 {
	if b {
		return 1
	}
	return 0
}

func (m *mv2State) calculfrq() {
	m.frqmod = uint32(uint64(50000) * 0x48d378 / hostFreq)
	m.samples = uint16(uint64(0x3e7) * hostFreq / 50000)
	const freq = 4
	if freq != m.prevmufreq {
		m.mutaloop = min(m.samples+1, muBufLen)
		m.prevmufreq = freq
		for i := 1; i < tabfrqLen; i++ {
			m.tabfrq[i-1] = m.frqmod / uint32(i)
		}
	}
}

// tabfrqAt reads mv2a.tabfrq[k]. Portamento can push freqsam past the
// table; alis then reads the struct fields that follow it (sMV2Audio layout).
func (m *mv2State) tabfrqAt(k int) uint32 {
	if k < tabfrqLen {
		return m.tabfrq[k]
	}
	var b [144]byte
	put := func(o int, x uint16) { b[o], b[o+1] = byte(x), byte(x>>8) }
	copy(b[2:34], m.defvol[:])
	for i, x := range m.tail() {
		put(106+2*i, x)
	}
	put(136, uint16(m.frqmod))
	put(138, uint16(m.frqmod>>16))
	put(140, m.samples)
	o := (k - tabfrqLen) * 4
	if o+4 > len(b) {
		return 0 // past sMV2Audio: whatever the C build put there
	}
	return uint32(b[o]) | uint32(b[o+1])<<8 | uint32(b[o+2])<<16 | uint32(b[o+3])<<24
}

// tail is the sMV2Audio fields after trkval (prevmufreq .. mutadata), as
// the overreads above see them; mutadata is mv2_trkfrq_ste[3].data.
func (m *mv2State) tail() []uint16 {
	return []uint16{uint16(m.prevmufreq), uint16(m.prevmuvol), m.mutype, 0, m.muchip, m.muopl2,
		m.muptr, m.mumax, m.mucnt, m.mubufa, m.mubufc, m.muspeed, m.muvolgen, m.mubreak, 2}
}

// trkvalAt reads mv2a.trkval[i]. alis never fills the table and indexes past
// it into the following struct fields.
func (m *mv2State) trkvalAt(i int32) int16 {
	if f := m.tail(); i >= 36 && int(i-36) < len(f) {
		return int16(f[i-36])
	}
	return 0
}

func (v *VM) mv2Offmusic(much uint32) {
	a, m := &v.aud, &v.mv2
	a.muchute = uint16(much)
	if a.muchute == 0 {
		a.muflag = 0
		return
	}
	a.muvol = uint8(m.muvolgen)
	m.mubufc = a.muchute
	m.mubreak = 1
}

func (v *VM) mv2Soundrout() {
	a, m := &v.aud, &v.mv2
	prevmuspeed := m.muspeed
	if a.muflag == 0 {
		return
	}
	newvolgen := uint16(a.muvol)
	chute := m.mubreak != 0
	if !chute {
		if a.muattac == 0 {
			if a.muduree == 0 {
				chute = true
			} else if a.muduree > 0 {
				a.muduree--
			}
		} else {
			a.muattac--
			tmp := uint16(uint32(a.muvol) * uint32(a.muattac) / uint32(m.mubufa))
			newvolgen = uint16(int32(tmp&0xff00) | -(int32(int8(tmp)) - int32(a.muvol)))
		}
	}
	if chute {
		if a.muchute == 0 {
			a.muflag = 0
			return
		}
		a.muchute--
		newvolgen = uint16(uint32(a.muvol) * uint32(a.muchute) / uint32(m.mubufc))
	}
	m.muvolgen = newvolgen
	if m.muspeed == 0 {
		a.muflag = 0
		return
	}
	m.muspeed--
	if m.muspeed == 0 || int16(prevmuspeed) < 1 {
		m.muspeed = uint16(a.mutemp)
		if int16(m.mucnt-0x40) >= 0 {
			m.mucnt = 0
			m.muptr++
		}
		if int16(m.muptr-m.mumax) >= 0 {
			m.muptr = 0
		}
		at := uint32(m.mucnt)*0x10 + uint32(v.mem[a.mupnote+uint32(m.muptr)])*0x400 + a.mupnote + 0x84
		for i := range m.voices {
			at = v.mv2Soundvoix(at, &m.voices[i])
		}
		m.mucnt++
	}
	for i := range m.voices {
		m.checkefft(&m.voices[i])
	}
	// Stereo: voices 0,3 left, 1,2 right, 30% cross-feed.
	l, r := m.left[:m.mutaloop], m.right[:m.mutaloop]
	clear(l)
	clear(r)
	v.mv2Soundcal(&m.voices[0], l, true)
	v.mv2Soundcal(&m.voices[1], r, true)
	v.mv2Soundcal(&m.voices[3], l, false)
	v.mv2Soundcal(&m.voices[2], r, false)
	for i := range l {
		x := int32(l[i]) + int32(r[i])*77>>8
		y := int32(r[i]) + int32(l[i])*77>>8
		l[i] = int16(max(-32768, min(32767, x)))
		r[i] = int16(max(-32768, min(32767, y)))
	}
}

func (v *VM) mv2Soundvoix(at uint32, vc *mv2Voice) uint32 {
	be16 := func(o uint32) uint16 { return uint16(v.mem[o])<<8 | uint16(v.mem[o+1]) }
	if note := uint32(be16(at))<<16 | uint32(be16(at+2)); note != 0 {
		vc.value = int16(note >> 16)
		vc.typ = uint8(note >> 8)
		vc.delta = uint8(note)
	}
	next := at + 2
	if newfreq := int16(be16(at)); newfreq != 0 {
		inst := uint16(v.mem[next]) & 0xf0
		if newfreq>>12&1 != 0 {
			newfreq &= 0xfff
			inst |= 0x100
		}
		inst = (inst>>1 - 8) >> 3
		// checkport
		if d := v.mem[next] & 0xf; d == 3 && vc.freqsam != 0 && int16(vc.freqsam-uint16(vc.value)) >= 0 {
			vc.delta = -v.mem[next+1]
		}
		v.mv2Soundins(vc, newfreq, inst)
	}
	v.mv2Checkcom(next, &vc.volsam)
	return next + 2
}

func (v *VM) mv2Soundins(vc *mv2Voice, newfreq int16, inst uint16) {
	a, m := &v.aud, &v.mv2
	var sample uint32
	var tval int32
	if int(inst) < len(a.tabinst) {
		sample, tval = a.tabinst[inst].address, int32(a.tabinst[inst].unknown)
	}
	if tval != 0 {
		// trkval is all zero, so the search never matches a note.
		newfreq = m.trkvalAt(36 + 2*tval)
	}
	if int16(uint16(newfreq)-0x71) < 0 {
		newfreq = 0x71
	}
	if newfreq > 0x357 {
		newfreq = 0x357
	}
	vc.freqsam = uint16(newfreq)
	typ := -1
	if sample != 0 {
		typ = int(v.mem[sample-0x10])
	}
	switch typ {
	case 2:
		vc.sample = sample - 0x10
		length := v.ru32(sample - 0xe)
		lend := min(v.ru32(sample-4), length-0x20)
		if lend != 0 {
			lend += 0x10
		}
		vc.longsam2 = lend - v.ru32(sample-8)
		vc.startsam2 = sample + lend - 1
		aend := length - 0x20
		if int32(vc.longsam2) > 0 {
			aend = lend
		}
		vc.startsam1 = sample + aend - 1
		vc.longsam1 = aend
		vc.volsam = 0x40
		vc.frqto = 0
	case 5, 6:
	default:
		smp := vc.sample
		*vc = mv2Voice{freqsam: vc.freqsam, frqto: vc.frqto, sample: smp}
	}
}

func (v *VM) mv2Checkcom(at uint32, volsam *uint16) {
	a, m := &v.aud, &v.mv2
	data := uint16(v.mem[at+1])
	switch v.mem[at] & 0xf {
	case 0xb:
		m.muptr = data - 1
		m.mucnt = 0x40
	case 0xc:
		if data > 0x40 {
			data = 0
		}
		*volsam = data
	case 0xd:
		m.mucnt = 0x40
	case 0xf:
		n := uint32(a.mutempo) * uint32(data&0x1f) >> 5
		a.mutemp = uint8(n)
		m.muspeed = uint16(n)
	}
}

func (m *mv2State) checkefft(vc *mv2Voice) {
	newval := uint16(vc.value)
	delta := uint16(vc.delta)
	stop := func() { vc.value, vc.typ, vc.delta = 0, 0, 0 }
	switch vc.typ & 0xf {
	case 1:
		newval = vc.freqsam - delta
		if int16(newval-0x71) < 0 {
			newval = 0x71
			stop()
		}
		vc.freqsam = newval
	case 2:
		newval = vc.freqsam + delta
		if int16(newval-0x357) >= 0 {
			newval = 0x357
			stop()
		}
		vc.freqsam = newval
	case 3:
		// delta is a zero-extended byte, so alis only ever takes the upward branch
		f := vc.freqsam + delta
		if int16(f-newval) >= 0 {
			f = newval
		}
		vc.freqsam = f
	case 5, 6, 0xa:
		if delta>>4 == 0 {
			vc.volsam -= delta & 0xf
			if int16(vc.volsam) < 0 {
				vc.volsam = 0
			}
		} else {
			vc.volsam += delta >> 4
			if vc.volsam > 0x40 {
				vc.volsam = 0x40
			}
		}
	}
}

// fVolume is alis f_volume (ST/STE scale).
func (m *mv2State) fVolume(volsam int16) int16 {
	vol := int16(uint16(int32(volsam)*int32(m.muvolgen))>>6) - 1
	if vol < 0 {
		return 0
	}
	return vol << 8
}

// mv2Soundcal is alis mv2_soundcal_c.
func (v *VM) mv2Soundcal(vc *mv2Voice, dst []int16, first bool) {
	m := &v.mv2
	var freq uint32
	if fs := int16(vc.freqsam); fs < 0 {
		freq = m.tabfrqAt(int(-int32(fs))>>2) >> 2
	} else {
		freq = m.tabfrqAt(int(fs))
	}
	startsam1, longsam1 := vc.startsam1, vc.longsam1
	vol := int16(uint8(uint16(m.fVolume(int16(vc.volsam))) >> 8))
	startsam2, longsam2 := vc.startsam2, vc.longsam2-1
	step := freq
	if int32(longsam2) < 1 {
		longsam2 = 0
		freq = 0
	}
	frqto := vc.frqto
	sample := vc.sample
	if sample == 0 || int(sample)+6 > len(v.mem) {
		return
	}
	smpend := sample + 0x10 + v.ru32(sample+2) - 0x11
	if smpend > v.finmem() || startsam1 < sample+0x10 || startsam1 > smpend {
		return
	}
	for i := range dst {
		before := uint32(uint16(longsam1))<<16 | uint32(frqto)
		after := before - step
		sw := before < step
		frqto = uint16(after)
		longsam1 = (longsam1>>16-b2u32(sw))<<16 | after>>16
		if sw {
			step = freq
			longsam1 = longsam2
			startsam1 = startsam2
			if longsam2 == 0 {
				break
			}
		}
		var s int8
		if o := startsam1 - longsam1; int(o) < len(v.mem) {
			s = int8(v.mem[o])
		}
		x := int16(int32(s) * int32(vol))
		if first {
			dst[i] = x
		} else {
			dst[i] = int16(max(-32768, min(32767, int32(dst[i])+int32(x))))
		}
	}
	vc.longsam1 = longsam1
	vc.startsam1 = startsam1
	vc.frqto = frqto
}

func b2u32(b bool) uint32 {
	if b {
		return 1
	}
	return 0
}

// mixMusic writes the music frames of one output buffer (the head of
// sys_audio_callback_S16MSB) and returns how many right-channel diffs to apply.
func (v *VM) mixMusic(pcm []int16) int {
	a, m := &v.aud, &v.mv2
	if a.muflag <= 0 || a.rout != routMV2 {
		return 0
	}
	put := func(f, k int) {
		pcm[2*f], pcm[2*f+1] = m.left[k], m.left[k]
		if f < rdiffLen {
			m.rdiff[f] = int32(m.right[k]) - int32(m.left[k])
		}
	}
	rem := len(pcm) / 2
	buflen := int(m.mutaloop)
	f := 0
	if m.smpidx != 0 {
		n := min(int(m.smpidx), rem)
		base := buflen - int(m.smpidx)
		for k := range n {
			put(f+k, base+k)
		}
		f += n
		rem -= n
		m.smpidx -= uint32(n)
	}
	for i := 0; i < (len(pcm)+buflen-1)/buflen && rem > 0; i++ {
		v.mv2Soundrout()
		n := min(buflen, rem)
		for k := range n {
			put(f+k, k)
		}
		f += n
		rem -= n
		m.smpidx = uint32(buflen - n)
	}
	return min(f, rdiffLen)
}
