//go:build windows

package platform

import (
	"fmt"
	"time"
	"unsafe"
)

var (
	procGetLastInputInfo = user32.NewProc("GetLastInputInfo")
	procGetTickCount64   = kernel32.NewProc("GetTickCount64")
)

// lastInputInfo mirrors the Win32 LASTINPUTINFO struct exactly
// cbSize must be set to sizeof(LASTINPUTINFO) before the call, per the
// documented contract, or GetLastInputInfo fails.
type lastInputInfo struct {
	cbSize uint32
	dwTime uint32 // tick count (ms since boot) of the last input event
}

type windowsIdleWatcher struct{}

func newIdleWatcher() IdleWatcher {
	return windowsIdleWatcher{}
}

func (windowsIdleWatcher) IdleDuration() (time.Duration, error) {
	info := lastInputInfo{cbSize: uint32(unsafe.Sizeof(lastInputInfo{}))}
	ret, _, callErr := procGetLastInputInfo.Call(uintptr(unsafe.Pointer(&info)))
	if ret == 0 {
		return 0, fmt.Errorf("platform(windows): GetLastInputInfo: %w", callErr)
	}

	nowTicks, _, _ := procGetTickCount64.Call()

	// dwTime is a 32-bit tick count and wraps roughly every 49.7 days;
	// nowTicks is 64-bit and doesn't. Compare only the low 32 bits of
	// nowTicks against dwTime so a wraparound near boot doesn't produce a
	// nonsensical multi-decade "idle" duration.
	nowLow32 := uint32(nowTicks)
	elapsedMS := nowLow32 - info.dwTime // unsigned subtraction handles wraparound correctly
	return time.Duration(elapsedMS) * time.Millisecond, nil
}
