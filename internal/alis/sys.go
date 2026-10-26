package alis

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Host is the platform the VM runs on. The VM keeps a virtual clock that
// only advances when a script sleeps; the host decides how that maps to real
// time and supplies input and file access.
type Host interface {
	// WallUs is the real time in µs since start; virtual-time hosts return 0.
	WallUs() uint64
	// WaitUntil blocks until the virtual time us may be reached.
	WaitUntil(us uint64)
	// PollInput applies the input due at virtual time us.
	PollInput(us uint64, in *Input)
	// Frame receives the composed display (ARGB, w*h) every sample period.
	Frame(us uint64, argb []uint32, w, h int)
	// Audio receives interleaved signed 16-bit stereo at 44100 Hz.
	Audio(samples []int16)
	// Random is the C library rand() the original seeds from the clock.
	Random() uint16
	// Time returns the local wall time of day.
	Time() (h, m, s int)
}

// Mouse is the pointer state as the game sees it (game pixels).
type Mouse struct {
	X, Y      int
	LB, RB    bool
	LBClicked bool // latched presses, cleared when a script reads the mouse
	RBClicked bool
	Enabled   bool
}

// Input is the keyboard and mouse state. Keys are indexed by USB/SDL
// scancode; Button is the last key pressed (scancode and keycode).
type Input struct {
	Keys      [kScancodes]bool
	Mods      uint16
	ButtonSc  int
	ButtonSym int32
	Mouse     Mouse
	Quit      bool
}

// KeyDown records a key press like an SDL key event.
func (in *Input) KeyDown(sc int) {
	if sc <= 0 || sc >= kScancodes {
		return
	}
	in.Keys[sc] = true
	in.Mods |= modBit(sc)
	in.ButtonSc = sc
	in.ButtonSym = scancodeToKeycode(sc)
}

// KeyUp records a key release.
func (in *Input) KeyUp(sc int) {
	if sc <= 0 || sc >= kScancodes {
		return
	}
	in.Keys[sc] = false
	in.Mods &^= modBit(sc)
}

// SDL keyboard modifier bits.
const (
	kmodLShift = 0x0001
	kmodRShift = 0x0002
	kmodLCtrl  = 0x0040
	kmodRCtrl  = 0x0080
	kmodLAlt   = 0x0100
	kmodRAlt   = 0x0200
	kmodCaps   = 0x2000
	kmodShift  = kmodLShift | kmodRShift
	kmodCtrl   = kmodLCtrl | kmodRCtrl
	kmodAlt    = kmodLAlt | kmodRAlt
)

func modBit(sc int) uint16 {
	switch sc {
	case ScLShift:
		return kmodLShift
	case ScRShift:
		return kmodRShift
	case ScLCtrl:
		return kmodLCtrl
	case ScRCtrl:
		return kmodRCtrl
	case ScLAlt:
		return kmodLAlt
	case ScRAlt:
		return kmodRAlt
	}
	return 0
}

// FileSys gives the VM the game files: reads come from the save directory
// first, then the data directory (names match case-insensitively); writes
// go to the save directory.
type FileSys struct {
	DataDir string
	SaveDir string

	mu    sync.Mutex
	index map[string]string // lower-case name -> real path in DataDir
}

func NewFileSys(dataDir, saveDir string) *FileSys {
	return &FileSys{DataDir: dataDir, SaveDir: saveDir}
}

func (fs *FileSys) lookup(dir, name string) (string, bool) {
	p := filepath.Join(dir, name)
	if _, err := os.Stat(p); err == nil {
		return p, true
	}
	ents, err := os.ReadDir(dir)
	if err != nil {
		return "", false
	}
	for _, e := range ents {
		if strings.EqualFold(e.Name(), name) {
			return filepath.Join(dir, e.Name()), true
		}
	}
	return "", false
}

func cleanName(name string) string {
	name = strings.ReplaceAll(name, "\\", "/")
	if i := strings.LastIndexByte(name, ':'); i >= 0 {
		name = name[i+1:]
	}
	return strings.TrimLeft(name, "/")
}

// Path resolves a game file name to an existing file.
func (fs *FileSys) Path(name string) (string, bool) {
	name = cleanName(name)
	fs.mu.Lock()
	defer fs.mu.Unlock()
	if fs.SaveDir != "" {
		if p, ok := fs.lookup(fs.SaveDir, name); ok {
			return p, true
		}
	}
	return fs.lookup(fs.DataDir, name)
}

func (fs *FileSys) Exists(name string) bool {
	_, ok := fs.Path(name)
	return ok
}

func (fs *FileSys) ReadFile(name string) ([]byte, error) {
	p, ok := fs.Path(name)
	if !ok {
		return nil, os.ErrNotExist
	}
	return os.ReadFile(p)
}

// File is an open game file.
type File interface {
	io.Reader
	io.Writer
	io.Seeker
	io.Closer
}

// Open opens a file with ALIS open flags: 0x100 create if missing (r+),
// 0x200 truncate (w), 0x400 append.
func (fs *FileSys) Open(name string, mode uint16) (File, error) {
	name = cleanName(name)
	write := mode&0x700 != 0
	if !write {
		p, ok := fs.Path(name)
		if !ok {
			return nil, os.ErrNotExist
		}
		return os.Open(p)
	}
	dir := fs.SaveDir
	if dir == "" {
		dir = fs.DataDir
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	target := filepath.Join(dir, name)
	if p, ok := fs.lookup(dir, name); ok {
		target = p
	} else if mode&0x200 == 0 {
		// r+ / append on a file only in the data dir: copy it over first
		if src, ok := fs.lookup(fs.DataDir, name); ok && dir != fs.DataDir {
			if b, err := os.ReadFile(src); err == nil {
				if err := os.WriteFile(target, b, 0o644); err != nil {
					return nil, err
				}
			}
		}
	}
	switch {
	case mode&0x400 != 0:
		return os.OpenFile(target, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0o644)
	case mode&0x200 != 0:
		return os.OpenFile(target, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o644)
	default:
		return os.OpenFile(target, os.O_RDWR|os.O_CREATE, 0o644)
	}
}

func (fs *FileSys) Remove(name string) error {
	if fs.SaveDir == "" {
		return errors.New("no save dir")
	}
	p, ok := fs.lookup(fs.SaveDir, cleanName(name))
	if !ok {
		return os.ErrNotExist
	}
	return os.Remove(p)
}
