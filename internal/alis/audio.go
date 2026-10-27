package alis

// Host audio: the blocks the emulated DSP plays (sb.go), unsigned 8-bit at
// sbPlayRate, resampled linearly to stereo 16-bit at hostFreq.

const hostFreq = 44100

type sbOut struct {
	fifo []byte
	pos  float64 // in fifo
	log  func(end uint64, block []byte)
}

func (o *sbOut) push(block []byte) {
	o.fifo = append(o.fifo, block...)
}

// SBLog calls f with the end time (PIT clocks) and the samples of every
// block the DSP plays.
func (v *VM) SBLog(f func(end uint64, block []byte)) { v.sb.out.log = f }

func (v *VM) sbPlayed(block []byte) {
	if v.sb.out.log != nil {
		v.sb.out.log(v.clk.now+sbBlockClocks, block)
	}
	v.sb.out.push(block)
}

// mixAudio renders one 20 ms buffer of interleaved stereo.
func (v *VM) mixAudio(pcm []int16) {
	o := &v.sb.out
	const step = sbPlayRate / hostFreq
	for f := 0; f < len(pcm)/2; f++ {
		var s int16
		if i := int(o.pos); i+1 < len(o.fifo) {
			t := o.pos - float64(i)
			x := (1-t)*float64(o.fifo[i]) + t*float64(o.fifo[i+1])
			s = int16((x - 128) * 256)
			o.pos += step
		}
		pcm[2*f], pcm[2*f+1] = s, s
	}
	if n := int(o.pos); n > 0 {
		o.fifo = append(o.fifo[:0], o.fifo[n:]...)
		o.pos -= float64(n)
	}
}
