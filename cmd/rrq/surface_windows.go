package main

import (
	"unsafe"

	"github.com/go-gl/glfw/v3.3/glfw"
	"github.com/gogpu/wgpu"
	"golang.org/x/sys/windows"
)

// surfaceTarget returns the window's HWND.
func surfaceTarget(win *glfw.Window) wgpu.SurfaceTargetUnsafe {
	var inst windows.Handle
	_ = windows.GetModuleHandleEx(0, nil, &inst)
	return wgpu.SurfaceTargetFromWindowsHWND(uintptr(inst), uintptr(unsafe.Pointer(win.GetWin32Window())))
}
