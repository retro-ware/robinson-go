package alis

import (
	"fmt"
	"runtime/debug"
	"strings"
)

// Crash reports: a fatal error or a Go panic in the VM carries where it
// happened, so a problem met while playing can be traced afterwards.

// traceLen is how many of the last opcodes a report lists.
const traceLen = 64

// opTrace is a ring of the last opcodes run (readexecOpcode).
type opTrace struct {
	ent  [traceLen]opRec
	next int
}

type opRec struct {
	script string
	pc     uint32 // the opcode's address
	code   uint8
	tick   uint32 // timeclock
}

func (t *opTrace) add(r opRec) {
	t.ent[t.next%traceLen] = r
	t.next++
}

// Report is the crash report of a fatal error: the message, the running
// script, the time, the last opcodes and the Go stack.
func (e *Error) Report() string {
	if e.report == "" {
		return e.Msg + "\n"
	}
	return e.report
}

// newError builds the Error of msg with its report; stack is the Go stack
// to show (the caller's when nil).
func (v *VM) newError(msg string, stack []byte) *Error {
	if stack == nil {
		stack = debug.Stack()
	}
	var b strings.Builder
	fmt.Fprintf(&b, "error: %s\n", msg)
	fmt.Fprintf(&b, "script %s pc 0x%x, entity %d, time %d ms, timeclock %d\n",
		v.scriptName(), v.scriptPC(), v.varD5, v.Now()/1000, v.timeclock)
	fmt.Fprintf(&b, "\nlast opcodes (oldest first):\n")
	t := &v.trace
	for i := max(0, t.next-traceLen); i < t.next; i++ {
		r := t.ent[i%traceLen]
		fmt.Fprintf(&b, "  tick %-8d %-12s 0x%06x op 0x%02x\n", r.tick, r.script, r.pc, r.code)
	}
	fmt.Fprintf(&b, "\nGo stack:\n%s", stack)
	return &Error{Msg: msg, report: b.String()}
}
