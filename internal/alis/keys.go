package alis

// Hosts report keys as USB HID scancodes (SDL names in the input scripts);
// the VM turns them into the PC scancodes the game's keyboard handler reads.

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

// pcScancode is the PC keyboard (set 1) make code of a USB HID key as
// DOSBox sends it: ext keys come after an 0xE0 prefix. 0 = no key.
func pcScancode(hid int) (code uint8, ext bool) {
	switch {
	case hid >= ScA && hid <= ScZ:
		return "\x1e\x30\x2e\x20\x12\x21\x22\x23\x17\x24\x25\x26\x32\x31\x18\x19\x10\x13\x1f\x14\x16\x2f\x11\x2d\x15\x2c"[hid-ScA], false
	case hid >= Sc1 && hid <= Sc0:
		return uint8(2 + hid - Sc1), false
	case hid >= ScF1 && hid <= ScF10:
		return uint8(0x3b + hid - ScF1), false
	case hid >= ScKP1 && hid <= ScKP9:
		return "\x4f\x50\x51\x4b\x4c\x4d\x47\x48\x49"[hid-ScKP1], false
	}
	switch hid {
	case ScReturn:
		return 0x1c, false
	case ScEscape:
		return 0x01, false
	case ScBackspace:
		return 0x0e, false
	case ScTab:
		return 0x0f, false
	case ScSpace:
		return 0x39, false
	case ScMinus:
		return 0x0c, false
	case ScEquals:
		return 0x0d, false
	case ScLBracket:
		return 0x1a, false
	case ScRBracket:
		return 0x1b, false
	case ScBackslash, ScNonUSHash:
		return 0x2b, false
	case ScSemicolon:
		return 0x27, false
	case ScApostr:
		return 0x28, false
	case ScGrave:
		return 0x29, false
	case ScComma:
		return 0x33, false
	case ScPeriod:
		return 0x34, false
	case ScSlash:
		return 0x35, false
	case ScCapsLock:
		return 0x3a, false
	case ScF11:
		return 0x57, false
	case ScF12:
		return 0x58, false
	case ScPrintScr:
		return 0x37, true
	case ScScrollLk:
		return 0x46, false
	case ScInsert:
		return 0x52, true
	case ScHome:
		return 0x47, true
	case ScPageUp:
		return 0x49, true
	case ScDelete:
		return 0x53, true
	case ScEnd:
		return 0x4f, true
	case ScPageDown:
		return 0x51, true
	case ScRight:
		return 0x4d, true
	case ScLeft:
		return 0x4b, true
	case ScDown:
		return 0x50, true
	case ScUp:
		return 0x48, true
	case ScNumLock:
		return 0x45, false
	case ScKPDivide:
		return 0x35, true
	case ScKPMult:
		return 0x37, false
	case ScKPMinus:
		return 0x4a, false
	case ScKPPlus:
		return 0x4e, false
	case ScKPEnter:
		return 0x1c, true
	case ScKP0:
		return 0x52, false
	case ScKPPeriod:
		return 0x53, false
	case ScNonUSBack:
		return 0x56, false
	case ScLCtrl:
		return 0x1d, false
	case ScLShift:
		return 0x2a, false
	case ScLAlt:
		return 0x38, false
	case ScLGUI:
		return 0x5b, true
	case ScRCtrl:
		return 0x1d, true
	case ScRShift:
		return 0x36, false
	case ScRAlt:
		return 0x38, true
	case ScRGUI:
		return 0x5c, true
	}
	return 0, false
}

// keytab maps a scancode-1 to the game's key code (START.EXE 0x18d10, the
// 'Q' layout): 0xe0-0xe3 are the shift bits 2, 1, 4, 8; 0xfe is ignored.
var keytab = [128]uint8{
	0x1b, 0x26, 0x82, 0x22, 0x27, 0x28, 0x60, 0x8a, 0x21, 0x87, 0x85, 0x29,
	0x2d, 0x08, 0x09, 0x71, 0x77, 0x65, 0x72, 0x74, 0x79, 0x75, 0x69, 0x6f,
	0x70, 0x5e, 0x24, 0x0d, 0xe2, 0x61, 0x73, 0x64, 0x66, 0x67, 0x68, 0x6a,
	0x6b, 0x6c, 0x3b, 0x97, 0xe6, 0xe0, 0x3c, 0x7a, 0x78, 0x63, 0x76, 0x62,
	0x6e, 0x6d, 0x3b, 0x2e, 0x2f, 0xe1, 0x2a, 0xe3, 0x20, 0xe3, 0xbb, 0xbc,
	0xbd, 0xbe, 0xbf, 0xc0, 0xc1, 0xc2, 0xc3, 0xc4, 0xff, 0xff, 0x37, 0x38,
	0x39, 0x2d, 0x34, 0x35, 0x36, 0x2b, 0x31, 0x32, 0x33, 0x30, 0x2e, 0xff,
	0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
	0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
	0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
	0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
}

// The AZERTY and QWERTZ layouts replace entries 0x0e-0x36 (0x18c15,
// 0x18c67; START.EXE 0x2268).
var (
	keytabA = [0x29]uint8{
		0x09, 0x61, 0x7a, 0x65, 0x72, 0x74, 0x79, 0x75, 0x69, 0x6f, 0x70, 0x5e, 0x24, 0x0d,
		0xe2, 0x71, 0x73, 0x64, 0x66, 0x67, 0x68, 0x6a, 0x6b, 0x6c, 0x6d, 0x97, 0xe6, 0xe0,
		0x3c, 0x77, 0x78, 0x63, 0x76, 0x62, 0x6e, 0x2c, 0x3b, 0x3a, 0x3d, 0xe1, 0x2a,
	}
	keytabZ = [0x29]uint8{
		0x09, 0x71, 0x77, 0x65, 0x72, 0x74, 0x7a, 0x75, 0x69, 0x6f, 0x70, 0x5e, 0x24, 0x0d,
		0xe2, 0x61, 0x73, 0x64, 0x66, 0x67, 0x68, 0x6a, 0x6b, 0x6c, 0x3b, 0x97, 0xe6, 0xe0,
		0x3c, 0x79, 0x78, 0x63, 0x76, 0x62, 0x6e, 0x6d, 0x3b, 0x2e, 0x2f, 0xe1, 0x2a,
	}
)

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
