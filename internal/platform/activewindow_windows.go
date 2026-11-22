//go:build windows

package platform

import (
	"fmt"
	"path/filepath"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	user32   = windows.NewLazySystemDLL("user32.dll")
	kernel32 = windows.NewLazySystemDLL("kernel32.dll")

	procGetForegroundWindow     = user32.NewProc("GetForegroundWindow")
	procGetWindowTextW          = user32.NewProc("GetWindowTextW")
	procGetWindowTextLengthW    = user32.NewProc("GetWindowTextLengthW")
	procGetWindowThreadPID      = user32.NewProc("GetWindowThreadProcessId")
	procOpenProcess             = kernel32.NewProc("OpenProcess")
	procQueryFullProcessImgName = kernel32.NewProc("QueryFullProcessImageNameW")
	procCloseHandle             = kernel32.NewProc("CloseHandle")
)

const (
	processQueryLimitedInformation = 0x1000
)

type windowsActiveWindowWatcher struct{}

func newActiveWindowWatcher() ActiveWindowWatcher {
	return windowsActiveWindowWatcher{}
}

func (windowsActiveWindowWatcher) ActiveWindow() (ActiveWindowInfo, error) {
	hwnd, _, _ := procGetForegroundWindow.Call()
	if hwnd == 0 {
		// No window has focus (e.g. the desktop itself, or a brief gap
		// during an alt-tab). Not an error - just nothing to sample.
		return ActiveWindowInfo{}, ErrUnavailable
	}

	title, err := windowTitle(hwnd)
	if err != nil {
		// A title we can't read isn't fatal - process identity matters
		// more than the title for categorization, so keep going with an
		// empty title rather than dropping the whole sample.
		title = ""
	}

	pid, err := windowProcessID(hwnd)
	if err != nil {
		return ActiveWindowInfo{}, fmt.Errorf("platform(windows): resolving window PID: %w", err)
	}

	processPath, err := processImagePath(pid)
	if err != nil {
		// Common for protected/system processes we're not allowed to
		// query even with PROCESS_QUERY_LIMITED_INFORMATION - degrade to
		// "unknown" rather than failing the whole sample.
		processPath = ""
	}

	return ActiveWindowInfo{
		ProcessName: filepath.Base(processPath),
		WindowTitle: title,
		PID:         int(pid),
	}, nil
}

func windowTitle(hwnd uintptr) (string, error) {
	length, _, _ := procGetWindowTextLengthW.Call(hwnd)
	if length == 0 {
		return "", nil // legitimately untitled window, not an error
	}
	buf := make([]uint16, length+1)
	n, _, _ := procGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if n == 0 {
		return "", fmt.Errorf("GetWindowTextW returned 0")
	}
	return syscall.UTF16ToString(buf[:n]), nil
}

func windowProcessID(hwnd uintptr) (uint32, error) {
	var pid uint32
	ret, _, callErr := procGetWindowThreadPID.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
	if ret == 0 {
		return 0, fmt.Errorf("GetWindowThreadProcessId: %w", callErr)
	}
	return pid, nil
}

// processImagePath resolves a PID to its executable path using
// PROCESS_QUERY_LIMITED_INFORMATION, which - unlike full
// PROCESS_QUERY_INFORMATION - doesn't require elevated privileges and is
// the documented modern replacement for the older
// GetModuleFileNameEx-based approach.
func processImagePath(pid uint32) (string, error) {
	handle, _, callErr := procOpenProcess.Call(
		uintptr(processQueryLimitedInformation),
		0, // bInheritHandle = FALSE
		uintptr(pid),
	)
	if handle == 0 {
		return "", fmt.Errorf("OpenProcess(pid=%d): %w", pid, callErr)
	}
	defer procCloseHandle.Call(handle)

	buf := make([]uint16, windows.MAX_PATH)
	size := uint32(len(buf))
	ret, _, callErr := procQueryFullProcessImgName.Call(
		handle,
		0, // dwFlags = 0 (Win32 path format)
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(unsafe.Pointer(&size)),
	)
	if ret == 0 {
		return "", fmt.Errorf("QueryFullProcessImageNameW(pid=%d): %w", pid, callErr)
	}
	return syscall.UTF16ToString(buf[:size]), nil
}
