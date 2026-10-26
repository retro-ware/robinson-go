package main

import (
	"sync"

	"github.com/ebitengine/oto/v3"
)

// sound plays the VM's 44.1 kHz stereo mix through oto.
type sound struct {
	mu     sync.Mutex
	buf    []byte
	player *oto.Player
}

const maxQueued = 44100 * 4 / 5 // bytes: ~200 ms of s16 stereo

func newSound() (*sound, error) {
	ctx, ready, err := oto.NewContext(&oto.NewContextOptions{SampleRate: 44100, ChannelCount: 2, Format: oto.FormatSignedInt16LE})
	if err != nil {
		return nil, err
	}
	<-ready
	s := &sound{}
	s.player = ctx.NewPlayer(s)
	s.player.Play()
	return s, nil
}

func (s *sound) push(pcm []int16) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.buf) > maxQueued {
		return // the device fell behind; drop rather than drift
	}
	for _, v := range pcm {
		s.buf = append(s.buf, byte(v), byte(v>>8))
	}
}

// Read feeds oto; silence when the VM has not produced enough.
func (s *sound) Read(p []byte) (int, error) {
	s.mu.Lock()
	n := copy(p, s.buf)
	s.buf = s.buf[:copy(s.buf, s.buf[n:])]
	s.mu.Unlock()
	clear(p[n:])
	return len(p), nil
}
