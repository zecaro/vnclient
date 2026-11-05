package vnclient

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
)

// Keysym maps a Fyne key to an X11 keysym.
func Keysym(k fyne.KeyName) uint32 {
	switch k {
	case fyne.KeyReturn, fyne.KeyEnter:
		return 0xff0d
	case fyne.KeyBackspace:
		return 0xff08
	case fyne.KeyTab:
		return 0xff09
	case fyne.KeyEscape:
		return 0xff1b
	case fyne.KeyDelete:
		return 0xffff
	case fyne.KeyHome:
		return 0xff50
	case fyne.KeyEnd:
		return 0xff57
	case fyne.KeyPageUp:
		return 0xff55
	case fyne.KeyPageDown:
		return 0xff56
	case fyne.KeyLeft:
		return 0xff51
	case fyne.KeyUp:
		return 0xff52
	case fyne.KeyRight:
		return 0xff53
	case fyne.KeyDown:
		return 0xff54
	case desktop.KeyShiftLeft, desktop.KeyShiftRight:
		return 0xffe1
	case desktop.KeyControlLeft, desktop.KeyControlRight:
		return 0xffe3
	case desktop.KeyAltLeft, desktop.KeyAltRight:
		return 0xffe9
	case desktop.KeySuperLeft, desktop.KeySuperRight:
		return 0xffeb
	}
	return 0
}
