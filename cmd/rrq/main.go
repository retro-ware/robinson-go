// Command rrq plays Robinson's Requiem (PC CD) in a native window.
package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/go-gl/glfw/v3.3/glfw"
	"github.com/retro-ware/robinson-go/internal/alis"
)

const (
	screenW = 320
	screenH = 200
)

func init() { runtime.LockOSThread() }

// host runs the VM in real time: virtual µs are paced against the wall
// clock, input is queued from the window thread and applied when the VM
// polls, frames go to the window thread.
type host struct {
	start time.Time

	mu     sync.Mutex
	events []func(*alis.Input)
	quit   bool
	frame  []uint32
	fresh  bool

	snd *sound
}

func (h *host) WallUs() uint64 { return uint64(time.Since(h.start).Microseconds()) }

func (h *host) WaitUntil(us uint64) {
	if d := time.Duration(us)*time.Microsecond - time.Since(h.start); d > 0 {
		time.Sleep(d)
	}
}

func (h *host) PollInput(us uint64, in *alis.Input) {
	h.mu.Lock()
	for _, e := range h.events {
		e(in)
	}
	h.events = h.events[:0]
	if h.quit {
		in.Quit = true
	}
	h.mu.Unlock()
}

func (h *host) Frame(us uint64, argb []uint32, w, ht int) {
	h.mu.Lock()
	copy(h.frame, argb)
	h.fresh = true
	h.mu.Unlock()
}

func (h *host) Audio(s []int16) {
	if h.snd != nil {
		h.snd.push(s)
	}
}

func (h *host) Time() (int, int, int) {
	t := time.Now()
	return t.Hour(), t.Minute(), t.Second()
}

func (h *host) event(f func(*alis.Input)) {
	h.mu.Lock()
	h.events = append(h.events, f)
	h.mu.Unlock()
}

func main() {
	data := flag.String("data", "", "directory with the game files (default: orig/cd next to the binary or in the working directory)")
	save := flag.String("save", "", "save game directory (default: the user config directory)")
	scale := flag.Int("scale", 3, "initial window scale")
	mute := flag.Bool("mute", false, "no sound")
	flag.Parse()

	dataDir := *data
	if dataDir == "" {
		dataDir = "orig/cd"
	}
	saveDir := *save
	if saveDir == "" {
		if d, err := os.UserConfigDir(); err == nil {
			saveDir = filepath.Join(d, "robinson-go")
		} else {
			saveDir = dataDir
		}
	}
	if err := os.MkdirAll(saveDir, 0o755); err != nil {
		log.Fatal(err)
	}

	h := &host{start: time.Now(), frame: make([]uint32, screenW*screenH)}
	if !*mute {
		snd, err := newSound()
		if err != nil {
			log.Printf("audio: %v", err)
		}
		h.snd = snd
	}
	vm, err := alis.New(h, alis.NewFileSys(dataDir, saveDir))
	if err != nil {
		log.Fatal(err)
	}

	if err := glfw.Init(); err != nil {
		log.Fatal(err)
	}
	defer glfw.Terminate()
	glfw.WindowHint(glfw.ClientAPI, glfw.NoAPI)
	glfw.WindowHint(glfw.Resizable, glfw.True)
	win, err := glfw.CreateWindow(screenW**scale, screenH*6/5**scale, "Robinson's Requiem", nil, nil)
	if err != nil {
		log.Fatal(err)
	}
	disp, err := newDisplay(win, screenW, screenH)
	if err != nil {
		log.Fatal(err)
	}
	win.SetInputMode(glfw.CursorMode, glfw.CursorHidden)

	win.SetKeyCallback(func(_ *glfw.Window, key glfw.Key, _ int, action glfw.Action, _ glfw.ModifierKey) {
		sc := scancode(key)
		if sc == 0 {
			return
		}
		switch action {
		case glfw.Press:
			h.event(func(in *alis.Input) { in.KeyDown(sc) })
		case glfw.Release:
			h.event(func(in *alis.Input) { in.KeyUp(sc) })
		}
	})
	win.SetCursorPosCallback(func(w *glfw.Window, x, y float64) {
		ww, wh := w.GetSize()
		vw, vh := float64(ww), float64(ww)*3/4
		if vh > float64(wh) {
			vh = float64(wh)
			vw = vh * 4 / 3
		}
		gx := int((x - (float64(ww)-vw)/2) * screenW / vw)
		gy := int((y - (float64(wh)-vh)/2) * screenH / vh)
		gx = min(max(gx, 0), screenW-1)
		gy = min(max(gy, 0), screenH-1)
		h.event(func(in *alis.Input) { in.Mouse.X, in.Mouse.Y = gx, gy })
	})
	win.SetMouseButtonCallback(func(_ *glfw.Window, b glfw.MouseButton, action glfw.Action, _ glfw.ModifierKey) {
		down := action == glfw.Press
		switch b {
		case glfw.MouseButtonLeft:
			h.event(func(in *alis.Input) { in.Mouse.LB = down })
		case glfw.MouseButtonRight:
			h.event(func(in *alis.Input) { in.Mouse.RB = down })
		}
	})

	done := make(chan error, 1)
	vm.OnHang = func(e *alis.Error) { crashReport(e, saveDir) }
	go func() { done <- vm.Run() }()

	frame := make([]uint32, screenW*screenH)
	for !win.ShouldClose() {
		glfw.PollEvents()
		select {
		case err := <-done:
			if err != nil {
				crashReport(err, saveDir)
			}
			return
		default:
		}
		fw, fh := win.GetFramebufferSize()
		if err := disp.resize(fw, fh); err != nil {
			log.Fatal(err)
		}
		h.mu.Lock()
		copy(frame, h.frame)
		h.mu.Unlock()
		if err := disp.draw(frame, screenW, screenH); err != nil {
			log.Fatal(err)
		}
	}
	h.mu.Lock()
	h.quit = true
	h.mu.Unlock()
	vm.Stop()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
	}
}
