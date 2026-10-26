// Package script reads input scripts: one event per line, "ms op a b"
// with the time in ms of virtual time since the game timer started:
//
//	1500 key Space down
//	1600 key Space up
//	2000 mouse 160 100
//	2100 lb down
//
// Lines starting with # are comments.
package script

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

// Event is one input event: op is key (a = key name, b = down or up),
// mouse (a, b = x, y in 0..319, 0..199), lb or rb (a = down or up) or
// quit.
type Event struct {
	Ms       uint64
	Op, A, B string
}

// Read reads the events of the script at path in file order.
func Read(path string) ([]Event, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var evs []Event
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if line == "" || line[0] == '#' {
			continue
		}
		fs := strings.Fields(line)
		if len(fs) < 2 {
			continue
		}
		ms, err := strconv.ParseUint(fs[0], 10, 64)
		if err != nil {
			continue
		}
		e := Event{Ms: ms, Op: fs[1]}
		if len(fs) > 2 {
			e.A = fs[2]
		}
		if len(fs) > 3 {
			e.B = fs[3]
		}
		evs = append(evs, e)
	}
	return evs, sc.Err()
}
