package main

import (
	"bytes"
	"fmt"
	"hash/crc32"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
)

// The scripted sessions in testdata must reproduce the CRC logs of the C
// reference (ref/alis built with ALIS_DET=1) frame for frame, and their
// mixed audio must have the CRC32 of the reference ALIS_WAV output.

func TestMain(m *testing.M) {
	if os.Getenv("RRQH_RUN_MAIN") != "" {
		main()
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func TestScriptsMatchReference(t *testing.T) {
	data := filepath.Join("..", "..", "orig", "cd")
	if _, err := os.Stat(data); err != nil {
		t.Skip("game data not present in orig/cd")
	}
	for _, tc := range []struct {
		name  string
		maxMs int
	}{{"attract", 900000}, {"start", 150000}, {"walk", 120000}, {"play2", 1200000}, {"play6", 1200000}} {
		t.Run(tc.name, func(t *testing.T) {
			td, _ := filepath.Abs(filepath.Join("..", "..", "testdata"))
			tmp := t.TempDir()
			log, wav := filepath.Join(tmp, "crc.log"), filepath.Join(tmp, "out.wav")
			cmd := exec.Command(os.Args[0], "-data", ".", "-maxms", strconv.Itoa(tc.maxMs),
				"-input", filepath.Join(td, tc.name+".txt"), "-log", log, "-wav", wav, "-save", tmp)
			cmd.Dir = data
			cmd.Env = append(os.Environ(), "RRQH_RUN_MAIN=1")
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("harness: %v\n%s", err, out)
			}
			got, _ := os.ReadFile(log)
			want, _ := os.ReadFile(filepath.Join(td, tc.name+".c.log"))
			if !bytes.Equal(got, want) {
				gl, wl := bytes.Split(got, []byte("\n")), bytes.Split(want, []byte("\n"))
				for i := range min(len(gl), len(wl)) {
					if !bytes.Equal(gl[i], wl[i]) {
						t.Fatalf("line %d: got %q, want %q", i+1, gl[i], wl[i])
					}
				}
				t.Fatalf("got %d lines, want %d", len(gl), len(wl))
			}
			pcm, _ := os.ReadFile(wav)
			wantCRC, _ := os.ReadFile(filepath.Join(td, tc.name+".c.wavcrc"))
			if got := fmt.Sprintf("%08x", crc32.ChecksumIEEE(pcm)); got != string(bytes.TrimSpace(wantCRC)) {
				t.Errorf("audio CRC %s, want %s", got, bytes.TrimSpace(wantCRC))
			}
		})
	}
}
