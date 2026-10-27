package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
)

// The sessions in testdata must reproduce, draw for draw, the screen and
// palette CRCs that the original START.EXE produced under cmd/origrun
// (-frames) with the same input.

func TestMain(m *testing.M) {
	if os.Getenv("RRQH_RUN_MAIN") != "" {
		main()
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func TestFramesMatchOriginal(t *testing.T) {
	data := filepath.Join("..", "..", "orig", "cd")
	if _, err := os.Stat(data); err != nil {
		t.Skip("game data not present in orig/cd")
	}
	for _, tc := range []struct {
		name  string
		input string
		maxMs int
	}{{"intro90", "", 90000}} {
		t.Run(tc.name, func(t *testing.T) {
			td, _ := filepath.Abs(filepath.Join("..", "..", "testdata"))
			tmp := t.TempDir()
			frames := filepath.Join(tmp, "frames")
			args := []string{"-data", ".", "-maxms", strconv.Itoa(tc.maxMs), "-frames", frames, "-save", tmp}
			if tc.input != "" {
				args = append(args, "-input", filepath.Join(td, tc.input))
			}
			cmd := exec.Command(os.Args[0], args...)
			cmd.Dir = data
			cmd.Env = append(os.Environ(), "RRQH_RUN_MAIN=1")
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("harness: %v\n%s", err, out)
			}
			got, _ := os.ReadFile(frames)
			want, _ := os.ReadFile(filepath.Join(td, tc.name+".orig.frames"))
			gl, wl := bytes.Split(got, []byte("\n")), bytes.Split(want, []byte("\n"))
			for i := range min(len(gl), len(wl)) {
				if !bytes.Equal(gl[i], wl[i]) {
					t.Fatalf("line %d: got %q, want %q", i+1, gl[i], wl[i])
				}
			}
			if len(gl) != len(wl) {
				t.Fatalf("got %d lines, want %d", len(gl), len(wl))
			}
		})
	}
}
