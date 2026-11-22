//go:build windows

package alert

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

// winmm's PlaySoundW is the smallest possible Windows audio backend: one
// syscall, no cgo, no COM, no dependency beyond golang.org/x/sys (which
// this project already uses for platform detection). It is also the
// same call the "System sounds" control panel itself uses, so anything
// that can play a Windows notification sound can play this.
var playSoundW = windows.NewLazySystemDLL("winmm.dll").NewProc("PlaySoundW")

const (
	// sndAsync returns immediately and plays on the audio thread -
	// required, otherwise the scheduler's tick loop would block for the
	// length of the tone.
	sndAsync = 0x0001
	// sndFilename means pszSound is a path to a WAV file rather than a
	// resource alias or a memory buffer.
	sndFilename = 0x00020000
	// sndNoDefault stops Windows from substituting its own default
	// beep (or a *different* sound) if this file cannot be read - a
	// wrong sound is worse than a logged failure, because it would
	// make "is the alarm working?" unanswerable.
	sndNoDefault = 0x00000002
)

func playFile(path string) error {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return fmt.Errorf("encoding path %q: %w", path, err)
	}
	ret, _, callErr := playSoundW.Call(uintptr(unsafe.Pointer(p)), 0, sndFilename|sndAsync|sndNoDefault)
	if ret == 0 {
		// PlaySound returns FALSE (0) and leaves no meaningful last-error
		// in the common failure case, so report the raw syscall error
		// alongside the fact that it failed rather than pretending.
		return fmt.Errorf("PlaySoundW(%q) returned FALSE: %v", path, callErr)
	}
	return nil
}
