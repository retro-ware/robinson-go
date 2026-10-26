package main

import "github.com/go-gl/glfw/v3.3/glfw"

// scancode maps a GLFW key to the USB HID / SDL scancode the VM uses.
func scancode(k glfw.Key) int {
	switch {
	case k >= glfw.KeyA && k <= glfw.KeyZ:
		return 4 + int(k-glfw.KeyA)
	case k >= glfw.Key1 && k <= glfw.Key9:
		return 30 + int(k-glfw.Key1)
	case k >= glfw.KeyF1 && k <= glfw.KeyF12:
		return 58 + int(k-glfw.KeyF1)
	case k >= glfw.KeyKP1 && k <= glfw.KeyKP9:
		return 89 + int(k-glfw.KeyKP1)
	}
	return keymap[k]
}

var keymap = map[glfw.Key]int{
	glfw.Key0: 39, glfw.KeyEnter: 40, glfw.KeyEscape: 41, glfw.KeyBackspace: 42,
	glfw.KeyTab: 43, glfw.KeySpace: 44, glfw.KeyMinus: 45, glfw.KeyEqual: 46,
	glfw.KeyLeftBracket: 47, glfw.KeyRightBracket: 48, glfw.KeyBackslash: 49,
	glfw.KeySemicolon: 51, glfw.KeyApostrophe: 52, glfw.KeyGraveAccent: 53,
	glfw.KeyComma: 54, glfw.KeyPeriod: 55, glfw.KeySlash: 56, glfw.KeyCapsLock: 57,
	glfw.KeyPrintScreen: 70, glfw.KeyScrollLock: 71, glfw.KeyPause: 72,
	glfw.KeyInsert: 73, glfw.KeyHome: 74, glfw.KeyPageUp: 75, glfw.KeyDelete: 76,
	glfw.KeyEnd: 77, glfw.KeyPageDown: 78, glfw.KeyRight: 79, glfw.KeyLeft: 80,
	glfw.KeyDown: 81, glfw.KeyUp: 82, glfw.KeyNumLock: 83, glfw.KeyKPDivide: 84,
	glfw.KeyKPMultiply: 85, glfw.KeyKPSubtract: 86, glfw.KeyKPAdd: 87,
	glfw.KeyKPEnter: 88, glfw.KeyKP0: 98, glfw.KeyKPDecimal: 99, glfw.KeyWorld1: 100,
	glfw.KeyLeftControl: 224, glfw.KeyLeftShift: 225, glfw.KeyLeftAlt: 226, glfw.KeyLeftSuper: 227,
	glfw.KeyRightControl: 228, glfw.KeyRightShift: 229, glfw.KeyRightAlt: 230, glfw.KeyRightSuper: 231,
}
