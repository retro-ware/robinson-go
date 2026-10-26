package alis

// Keys use USB HID / SDL2 scancodes so the input scripts of the C harness
// (SDL key names) replay unchanged.

const kScancodes = 512

const (
	ScA         = 4
	ScZ         = 29
	Sc1         = 30
	Sc9         = 38
	Sc0         = 39
	ScReturn    = 40
	ScEscape    = 41
	ScBackspace = 42
	ScTab       = 43
	ScSpace     = 44
	ScMinus     = 45
	ScEquals    = 46
	ScLBracket  = 47
	ScRBracket  = 48
	ScBackslash = 49
	ScNonUSHash = 50
	ScSemicolon = 51
	ScApostr    = 52
	ScGrave     = 53
	ScComma     = 54
	ScPeriod    = 55
	ScSlash     = 56
	ScCapsLock  = 57
	ScF1        = 58
	ScF10       = 67
	ScF11       = 68
	ScF12       = 69
	ScPrintScr  = 70
	ScScrollLk  = 71
	ScPause     = 72
	ScInsert    = 73
	ScHome      = 74
	ScPageUp    = 75
	ScDelete    = 76
	ScEnd       = 77
	ScPageDown  = 78
	ScRight     = 79
	ScLeft      = 80
	ScDown      = 81
	ScUp        = 82
	ScNumLock   = 83
	ScKPDivide  = 84
	ScKPMult    = 85
	ScKPMinus   = 86
	ScKPPlus    = 87
	ScKPEnter   = 88
	ScKP1       = 89
	ScKP9       = 97
	ScKP0       = 98
	ScKPPeriod  = 99
	ScNonUSBack = 100
	ScLCtrl     = 224
	ScLShift    = 225
	ScLAlt      = 226
	ScLGUI      = 227
	ScRCtrl     = 228
	ScRShift    = 229
	ScRAlt      = 230
	ScRGUI      = 231
	ScMode      = 257
)

// SDL keycodes that are not plain characters are scancode|1<<30.
const sdlkScancodeMask = 1 << 30

const (
	sdlkF1     = ScF1 | sdlkScancodeMask
	sdlkF10    = ScF10 | sdlkScancodeMask
	sdlkUp     = ScUp | sdlkScancodeMask
	sdlkDown   = ScDown | sdlkScancodeMask
	sdlkLeft   = ScLeft | sdlkScancodeMask
	sdlkRight  = ScRight | sdlkScancodeMask
	sdlkKP1    = ScKP1 | sdlkScancodeMask
	sdlkKP9    = ScKP9 | sdlkScancodeMask
	sdlkKP0    = ScKP0 | sdlkScancodeMask
	sdlkKPPlus = ScKPPlus | sdlkScancodeMask
	sdlkKPMin  = ScKPMinus | sdlkScancodeMask
	sdlkLShift = ScLShift | sdlkScancodeMask
	sdlkRShift = ScRShift | sdlkScancodeMask
	sdlkLCtrl  = ScLCtrl | sdlkScancodeMask
	sdlkRCtrl  = ScRCtrl | sdlkScancodeMask
	sdlkLAlt   = ScLAlt | sdlkScancodeMask
	sdlkRAlt   = ScRAlt | sdlkScancodeMask
	sdlkCaps   = ScCapsLock | sdlkScancodeMask
	sdlkMode   = ScMode | sdlkScancodeMask
)

// scancodeToKeycode is SDL's default US layout keymap.
func scancodeToKeycode(sc int) int32 {
	switch {
	case sc >= ScA && sc <= ScZ:
		return int32('a' + sc - ScA)
	case sc >= Sc1 && sc <= Sc9:
		return int32('1' + sc - Sc1)
	case sc == Sc0:
		return '0'
	}
	switch sc {
	case ScReturn:
		return '\r'
	case ScEscape:
		return 0x1b
	case ScBackspace:
		return 8
	case ScTab:
		return 9
	case ScSpace:
		return ' '
	case ScMinus:
		return '-'
	case ScEquals:
		return '='
	case ScLBracket:
		return '['
	case ScRBracket:
		return ']'
	case ScBackslash:
		return '\\'
	case ScNonUSHash:
		return '#'
	case ScSemicolon:
		return ';'
	case ScApostr:
		return '\''
	case ScGrave:
		return '`'
	case ScComma:
		return ','
	case ScPeriod:
		return '.'
	case ScSlash:
		return '/'
	case ScDelete:
		return 0x7f
	case ScNonUSBack:
		return '<'
	}
	return int32(sc) | sdlkScancodeMask
}

// ScancodeByName resolves SDL_GetScancodeFromName names (case-insensitive).
func ScancodeByName(name string) int {
	if len(name) == 1 {
		c := name[0]
		switch {
		case c >= 'a' && c <= 'z':
			return ScA + int(c-'a')
		case c >= 'A' && c <= 'Z':
			return ScA + int(c-'A')
		case c >= '1' && c <= '9':
			return Sc1 + int(c-'1')
		case c == '0':
			return Sc0
		}
	}
	for sc, n := range scancodeNames {
		if equalFold(n, name) {
			return sc
		}
	}
	return 0
}

func equalFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		x, y := a[i], b[i]
		if x >= 'A' && x <= 'Z' {
			x += 32
		}
		if y >= 'A' && y <= 'Z' {
			y += 32
		}
		if x != y {
			return false
		}
	}
	return true
}

var scancodeNames = map[int]string{
	ScReturn: "Return", ScEscape: "Escape", ScBackspace: "Backspace", ScTab: "Tab", ScSpace: "Space",
	ScMinus: "-", ScEquals: "=", ScLBracket: "[", ScRBracket: "]", ScBackslash: "\\", ScNonUSHash: "#",
	ScSemicolon: ";", ScApostr: "'", ScGrave: "`", ScComma: ",", ScPeriod: ".", ScSlash: "/",
	ScCapsLock: "CapsLock", ScF1: "F1", ScF1 + 1: "F2", ScF1 + 2: "F3", ScF1 + 3: "F4", ScF1 + 4: "F5",
	ScF1 + 5: "F6", ScF1 + 6: "F7", ScF1 + 7: "F8", ScF1 + 8: "F9", ScF10: "F10", ScF11: "F11", ScF12: "F12",
	ScPrintScr: "PrintScreen", ScScrollLk: "ScrollLock", ScPause: "Pause", ScInsert: "Insert", ScHome: "Home",
	ScPageUp: "PageUp", ScDelete: "Delete", ScEnd: "End", ScPageDown: "PageDown", ScRight: "Right",
	ScLeft: "Left", ScDown: "Down", ScUp: "Up", ScNumLock: "Numlock", ScKPEnter: "Keypad_Enter",
	ScKPPlus: "Keypad_+", ScKPMinus: "Keypad_-", ScKPMult: "Keypad_*", ScKPDivide: "Keypad_/",
	ScKP1: "Keypad_1", ScKP1 + 1: "Keypad_2", ScKP1 + 2: "Keypad_3", ScKP1 + 3: "Keypad_4", ScKP1 + 4: "Keypad_5",
	ScKP1 + 5: "Keypad_6", ScKP1 + 6: "Keypad_7", ScKP1 + 7: "Keypad_8", ScKP9: "Keypad_9", ScKP0: "Keypad_0",
	ScKPPeriod: "Keypad_.", ScLCtrl: "Left_Ctrl", ScLShift: "Left_Shift", ScLAlt: "Left_Alt",
	ScRCtrl: "Right_Ctrl", ScRShift: "Right_Shift", ScRAlt: "Right_Alt",
}
