//go:build windows

package platform

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

// gdi32 and the procs below are new to this file; user32/kernel32
// themselves are already declared in activewindow_windows.go - Go
// shares package-level vars across every file under the same build tag,
// so they're reused here rather than redeclared.
var (
	gdi32 = windows.NewLazySystemDLL("gdi32.dll")

	procGetDC                  = user32.NewProc("GetDC")
	procReleaseDC              = user32.NewProc("ReleaseDC")
	procGetSystemMetrics       = user32.NewProc("GetSystemMetrics")
	procCreateCompatibleDC     = gdi32.NewProc("CreateCompatibleDC")
	procCreateCompatibleBitmap = gdi32.NewProc("CreateCompatibleBitmap")
	procSelectObject           = gdi32.NewProc("SelectObject")
	procBitBlt                 = gdi32.NewProc("BitBlt")
	procGetDIBits              = gdi32.NewProc("GetDIBits")
	procDeleteDC               = gdi32.NewProc("DeleteDC")
	procDeleteObject           = gdi32.NewProc("DeleteObject")
)

const (
	smCxScreen   = 0
	smCyScreen   = 1
	srcCopy      = 0x00CC0020
	biRGB        = 0
	dibRGBColors = 0
)

// bitmapInfoHeader mirrors the Win32 BITMAPINFOHEADER struct exactly
// (field order and sizes matter - this is passed directly to
// GetDIBits as a raw pointer).
type bitmapInfoHeader struct {
	size          uint32
	width         int32
	height        int32
	planes        uint16
	bitCount      uint16
	compression   uint32
	sizeImage     uint32
	xPelsPerMeter int32
	yPelsPerMeter int32
	clrUsed       uint32
	clrImportant  uint32
}

type windowsScreenCapturer struct{}

func newScreenCapturer() ScreenCapturer {
	return windowsScreenCapturer{}
}

// CaptureScreen uses the standard Win32 GDI screen-copy sequence
// (GetDC → CreateCompatibleDC/Bitmap → BitBlt → GetDIBits), the same
// well-documented approach every native Windows screenshot tool is
// built on - no third-party screenshot library, no cgo. Every GDI
// object created here is explicitly released/deleted before returning,
// GDI handles are a genuinely limited OS resource and this runs
// repeatedly on an interval (spec section 27).
func (windowsScreenCapturer) CaptureScreen(path string) error {
	widthRaw, _, _ := procGetSystemMetrics.Call(uintptr(smCxScreen))
	heightRaw, _, _ := procGetSystemMetrics.Call(uintptr(smCyScreen))
	width, height := int(widthRaw), int(heightRaw)
	if width <= 0 || height <= 0 {
		return ErrUnavailable
	}

	screenDC, _, _ := procGetDC.Call(0)
	if screenDC == 0 {
		return ErrUnavailable
	}
	defer procReleaseDC.Call(0, screenDC)

	memDC, _, _ := procCreateCompatibleDC.Call(screenDC)
	if memDC == 0 {
		return ErrUnavailable
	}
	defer procDeleteDC.Call(memDC)

	bitmap, _, _ := procCreateCompatibleBitmap.Call(screenDC, uintptr(width), uintptr(height))
	if bitmap == 0 {
		return ErrUnavailable
	}
	defer procDeleteObject.Call(bitmap)

	oldObj, _, _ := procSelectObject.Call(memDC, bitmap)
	defer procSelectObject.Call(memDC, oldObj)

	ok, _, _ := procBitBlt.Call(memDC, 0, 0, uintptr(width), uintptr(height), screenDC, 0, 0, uintptr(srcCopy))
	if ok == 0 {
		return ErrUnavailable
	}

	header := bitmapInfoHeader{
		size:        uint32(unsafe.Sizeof(bitmapInfoHeader{})),
		width:       int32(width),
		height:      int32(-height), // negative = top-down DIB rows, avoiding a manual vertical flip
		planes:      1,
		bitCount:    32,
		compression: biRGB,
	}
	pixels := make([]byte, width*height*4)
	res, _, _ := procGetDIBits.Call(
		memDC, bitmap, 0, uintptr(height),
		uintptr(unsafe.Pointer(&pixels[0])),
		uintptr(unsafe.Pointer(&header)),
		uintptr(dibRGBColors),
	)
	if res == 0 {
		return ErrUnavailable
	}

	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for i := 0; i < width*height; i++ {
		// GDI's 32-bit DIB is BGRA byte order; Go's image.RGBA wants RGBA.
		b, g, r, a := pixels[i*4+0], pixels[i*4+1], pixels[i*4+2], pixels[i*4+3]
		if a == 0 {
			a = 255 // BitBlt output doesn't carry meaningful alpha - treat the capture as fully opaque
		}
		img.Pix[i*4+0], img.Pix[i*4+1], img.Pix[i*4+2], img.Pix[i*4+3] = r, g, b, a
	}

	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("platform: creating screenshot file: %w", err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		return fmt.Errorf("platform: encoding screenshot: %w", err)
	}
	return nil
}
