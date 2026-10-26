package main

import (
	"sync"
	"time"

	"github.com/ebitengine/oto/v3"
)

// sound plays the VM's 44.1 kHz stereo mix through oto. It is a jitter
// buffer: playback starts (and restarts after an underrun) only once
// primeBytes are queued, and a backlog beyond maxBytes (the VM catching up
// after a stall) is trimmed from the front back to primeBytes.
type sound struct {
	mu      sync.Mutex
	buf     []byte
	playing bool
	player  *oto.Player
}

const (
	bytesPerMs = 44100 * 4 / 1000
	primeBytes = 60 * bytesPerMs &^ 3
	maxBytes   = 250 * bytesPerMs &^ 3
)

func newSound() (*sound, error) {
	ctx, ready, err := oto.NewContext(&oto.NewContextOptions{SampleRate: 44100, ChannelCount: 2,
		Format: oto.FormatSignedInt16LE, BufferSize: 30 * time.Millisecond})
	if err != nil {
		return nil, err
	}
	<-ready
	s := &sound{}
	s.player = ctx.NewPlayer(s)
	s.player.SetBufferSize(primeBytes)
	s.player.Play()
	return s, nil
}

func (s *sound) push(pcm []int16) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, v := range pcm {
		s.buf = append(s.buf, byte(v), byte(v>>8))
	}
	if len(s.buf) > maxBytes {
		drop := len(s.buf) - primeBytes
		s.buf = s.buf[:copy(s.buf, s.buf[drop:])]
	}
}

// Read feeds oto; silence while (re)priming.
func (s *sound) Read(p []byte) (int, error) {
	s.mu.Lock()
	if !s.playing && len(s.buf) >= primeBytes {
		s.playing = true
	}
	n := 0
	if s.playing {
		n = copy(p, s.buf)
		s.buf = s.buf[:copy(s.buf, s.buf[n:])]
		if n < len(p) {
			s.playing = false
		}
	}
	s.mu.Unlock()
	clear(p[n:])
	return len(p), nil
}
