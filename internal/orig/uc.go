//go:build orig

// Package orig runs the original START.EXE (X-32 extender, 32-bit flat
// image) under unicorn with a minimal DOS, VGA, timer, mouse and keyboard
// around it, on the same virtual clock as the Go port, as a test oracle.
//
// It needs unicorn 2.1.4 (static library and headers) in ref/unicorn and
// builds only with the "orig" tag.
package orig

/*
#cgo CFLAGS: -I${SRCDIR}/../../ref/unicorn/include
#cgo LDFLAGS: ${SRCDIR}/../../ref/unicorn/lib/libunicorn.a -lpthread -lm
#include <stdlib.h>
#include <unicorn/unicorn.h>

extern void goHookIntr(uc_engine *uc, uint32_t intno, void *user);
extern uint32_t goHookIn(uc_engine *uc, uint32_t port, int size, void *user);
extern void goHookOut(uc_engine *uc, uint32_t port, int size, uint32_t value, void *user);
extern void goHookCode(uc_engine *uc, uint64_t addr, uint32_t size, void *user);
extern void goHookWrite(uc_engine *uc, uc_mem_type type, uint64_t addr, int size, int64_t value, void *user);
extern bool goHookInvalid(uc_engine *uc, uc_mem_type type, uint64_t addr, int size, int64_t value, void *user);

static uc_err hook_intr(uc_engine *uc, uc_hook *h, uintptr_t user) {
	return uc_hook_add(uc, h, UC_HOOK_INTR, goHookIntr, (void *)user, 1, 0);
}
static uc_err hook_in(uc_engine *uc, uc_hook *h, uintptr_t user) {
	return uc_hook_add(uc, h, UC_HOOK_INSN, goHookIn, (void *)user, 1, 0, UC_X86_INS_IN);
}
static uc_err hook_out(uc_engine *uc, uc_hook *h, uintptr_t user) {
	return uc_hook_add(uc, h, UC_HOOK_INSN, goHookOut, (void *)user, 1, 0, UC_X86_INS_OUT);
}
static uc_err hook_code(uc_engine *uc, uc_hook *h, uintptr_t user, uint64_t begin, uint64_t end) {
	return uc_hook_add(uc, h, UC_HOOK_CODE, goHookCode, (void *)user, begin, end);
}
static uc_err hook_invalid(uc_engine *uc, uc_hook *h, uintptr_t user) {
	return uc_hook_add(uc, h, UC_HOOK_MEM_INVALID, goHookInvalid, (void *)user, 1, 0);
}
static uc_err hook_write(uc_engine *uc, uc_hook *h, uintptr_t user, uint64_t begin, uint64_t end) {
	return uc_hook_add(uc, h, UC_HOOK_MEM_WRITE, goHookWrite, (void *)user, begin, end);
}
static uc_err flush_tb(uc_engine *uc) { return uc_ctl_flush_tb(uc); }
// ctx_len and ctx_data are the CPU state in a saved context: the context
// is a header and the state, whose size is the header's first field.
static size_t ctx_len(uc_context *c) { return *(size_t *)c; }
static char *ctx_data(uc_engine *uc, uc_context *c) {
	return (char *)c + (uc_context_size(uc) - ctx_len(c));
}
static uc_err set_mmr(uc_engine *uc, int reg, uint64_t base, uint32_t limit) {
	uc_x86_mmr m = {0, base, limit, 0};
	return uc_reg_write(uc, reg, &m);
}
*/
import "C"

import (
	"fmt"
	"runtime/cgo"
	"unsafe"
)

type ucErr C.uc_err

func (e ucErr) Error() string { return C.GoString(C.uc_strerror(C.uc_err(e))) }

func ucCheck(e C.uc_err) error {
	if e == C.UC_ERR_OK {
		return nil
	}
	return ucErr(e)
}

// regID is a unicorn x86 register id.
type regID int

// x86 register ids used here.
const (
	rEAX    regID = C.UC_X86_REG_EAX
	rEBX    regID = C.UC_X86_REG_EBX
	rECX    regID = C.UC_X86_REG_ECX
	rEDX    regID = C.UC_X86_REG_EDX
	rESI    regID = C.UC_X86_REG_ESI
	rEDI    regID = C.UC_X86_REG_EDI
	rEBP    regID = C.UC_X86_REG_EBP
	rESP    regID = C.UC_X86_REG_ESP
	rEIP    regID = C.UC_X86_REG_EIP
	rEFLAGS regID = C.UC_X86_REG_EFLAGS
	rCS     regID = C.UC_X86_REG_CS
	rDS     regID = C.UC_X86_REG_DS
	rES     regID = C.UC_X86_REG_ES
	rFS     regID = C.UC_X86_REG_FS
	rGS     regID = C.UC_X86_REG_GS
	rSS     regID = C.UC_X86_REG_SS
	rGDTR   regID = C.UC_X86_REG_GDTR
)

// engine is a unicorn x86-32 instance with host-backed memory.
type engine struct {
	uc     *C.uc_engine
	handle cgo.Handle
	bufs   []unsafe.Pointer

	onIntr    func(intno uint32)
	onIn      func(port uint16, size int) uint32
	onOut     func(port uint16, size int, val uint32)
	onCode    map[uint32]func()
	onInvalid func(addr uint64, size int, write bool)
	onWrite   func(addr uint32, size int, val uint32)
}

func newEngine() (*engine, error) {
	e := &engine{onCode: map[uint32]func(){}}
	if err := ucCheck(C.uc_open(C.UC_ARCH_X86, C.UC_MODE_32, &e.uc)); err != nil {
		return nil, err
	}
	e.handle = cgo.NewHandle(e)
	var h C.uc_hook
	u := C.uintptr_t(e.handle)
	for _, err := range []C.uc_err{
		C.hook_intr(e.uc, &h, u),
		C.hook_in(e.uc, &h, u),
		C.hook_out(e.uc, &h, u),
		C.hook_invalid(e.uc, &h, u),
	} {
		if err := ucCheck(err); err != nil {
			e.close()
			return nil, err
		}
	}
	return e, nil
}

func (e *engine) close() {
	if e.uc != nil {
		C.uc_close(e.uc)
		e.uc = nil
	}
	for _, b := range e.bufs {
		C.free(b)
	}
	e.bufs = nil
	if e.handle != 0 {
		e.handle.Delete()
		e.handle = 0
	}
}

// mapMem maps size bytes at linear addr, backed by zeroed host memory that
// the returned slice aliases.
func (e *engine) mapMem(addr uint64, size int) ([]byte, error) {
	p := C.calloc(C.size_t(size), 1)
	if p == nil {
		return nil, fmt.Errorf("calloc %d", size)
	}
	e.bufs = append(e.bufs, p)
	if err := ucCheck(C.uc_mem_map_ptr(e.uc, C.uint64_t(addr), C.uint64_t(size), C.UC_PROT_ALL, p)); err != nil {
		return nil, err
	}
	return unsafe.Slice((*byte)(p), size), nil
}

// hookCode calls f before the instruction at linear address addr runs.
func (e *engine) hookCode(addr uint32, f func()) error {
	if _, ok := e.onCode[addr]; !ok {
		var h C.uc_hook
		if err := ucCheck(C.hook_code(e.uc, &h, C.uintptr_t(e.handle), C.uint64_t(addr), C.uint64_t(addr))); err != nil {
			return err
		}
	}
	e.onCode[addr] = f
	return nil
}

func (e *engine) reg(id regID) uint32 {
	var v C.uint64_t
	C.uc_reg_read(e.uc, C.int(id), unsafe.Pointer(&v))
	return uint32(v)
}

func (e *engine) setReg(id regID, v uint32) error {
	x := C.uint64_t(v)
	return ucCheck(C.uc_reg_write(e.uc, C.int(id), unsafe.Pointer(&x)))
}

func (e *engine) setGDTR(base uint64, limit uint32) error {
	return ucCheck(C.set_mmr(e.uc, C.int(rGDTR), C.uint64_t(base), C.uint32_t(limit)))
}

// start runs from offset eip in the current code segment until stop() or
// an error.
func (e *engine) start(eip uint32) error {
	return ucCheck(C.uc_emu_start(e.uc, C.uint64_t(eip), 0xffffffffffffffff, 0, 0))
}

func (e *engine) stop() { C.uc_emu_stop(e.uc) }

func (e *engine) flushTB() { C.flush_tb(e.uc) }

// editCPUState calls f on a copy of unicorn's CPU state (QEMU's
// CPUX86State) and writes it back.
func (e *engine) editCPUState(f func(b []byte) error) error {
	var c *C.uc_context
	if err := ucCheck(C.uc_context_alloc(e.uc, &c)); err != nil {
		return err
	}
	defer C.uc_context_free(c)
	if err := ucCheck(C.uc_context_save(e.uc, c)); err != nil {
		return err
	}
	b := unsafe.Slice((*byte)(unsafe.Pointer(C.ctx_data(e.uc, c))), int(C.ctx_len(c)))
	if err := f(b); err != nil {
		return err
	}
	return ucCheck(C.uc_context_restore(e.uc, c))
}

//export goHookIntr
func goHookIntr(uc *C.uc_engine, intno C.uint32_t, user unsafe.Pointer) {
	e := cgo.Handle(uintptr(user)).Value().(*engine)
	e.onIntr(uint32(intno))
}

//export goHookIn
func goHookIn(uc *C.uc_engine, port C.uint32_t, size C.int, user unsafe.Pointer) C.uint32_t {
	e := cgo.Handle(uintptr(user)).Value().(*engine)
	return C.uint32_t(e.onIn(uint16(port), int(size)))
}

//export goHookOut
func goHookOut(uc *C.uc_engine, port C.uint32_t, size C.int, val C.uint32_t, user unsafe.Pointer) {
	e := cgo.Handle(uintptr(user)).Value().(*engine)
	e.onOut(uint16(port), int(size), uint32(val))
}

//export goHookCode
func goHookCode(uc *C.uc_engine, addr C.uint64_t, size C.uint32_t, user unsafe.Pointer) {
	e := cgo.Handle(uintptr(user)).Value().(*engine)
	if f := e.onCode[uint32(addr)]; f != nil {
		f()
	}
}

// hookWrite calls f for every write to [from, to).
func (e *engine) hookWrite(from, to uint32, f func(addr uint32, size int, val uint32)) error {
	e.onWrite = f
	var h C.uc_hook
	return ucCheck(C.hook_write(e.uc, &h, C.uintptr_t(e.handle), C.uint64_t(from), C.uint64_t(to-1)))
}

//export goHookWrite
func goHookWrite(uc *C.uc_engine, typ C.uc_mem_type, addr C.uint64_t, size C.int, value C.int64_t, user unsafe.Pointer) {
	e := cgo.Handle(uintptr(user)).Value().(*engine)
	e.onWrite(uint32(addr), int(size), uint32(value))
}

//export goHookInvalid
func goHookInvalid(uc *C.uc_engine, typ C.uc_mem_type, addr C.uint64_t, size C.int, value C.int64_t, user unsafe.Pointer) C.bool {
	e := cgo.Handle(uintptr(user)).Value().(*engine)
	if e.onInvalid != nil {
		e.onInvalid(uint64(addr), int(size), typ == C.UC_MEM_WRITE_UNMAPPED || typ == C.UC_MEM_WRITE_PROT)
	}
	return false
}
