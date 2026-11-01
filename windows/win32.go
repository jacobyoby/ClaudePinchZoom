//go:build windows

package main

import (
	"fmt"
	"syscall"
	"unsafe"
)

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	shell32  = syscall.NewLazyDLL("shell32.dll")
	hid      = syscall.NewLazyDLL("hid.dll")

	procRegisterClassExW           = user32.NewProc("RegisterClassExW")
	procCreateWindowExW            = user32.NewProc("CreateWindowExW")
	procDefWindowProcW             = user32.NewProc("DefWindowProcW")
	procDestroyWindow              = user32.NewProc("DestroyWindow")
	procGetMessageW                = user32.NewProc("GetMessageW")
	procTranslateMessage           = user32.NewProc("TranslateMessage")
	procDispatchMessageW           = user32.NewProc("DispatchMessageW")
	procPostQuitMessage            = user32.NewProc("PostQuitMessage")
	procRegisterRawInputDevices    = user32.NewProc("RegisterRawInputDevices")
	procGetRawInputData            = user32.NewProc("GetRawInputData")
	procGetRawInputDeviceInfoW     = user32.NewProc("GetRawInputDeviceInfoW")
	procGetForegroundWindow        = user32.NewProc("GetForegroundWindow")
	procGetWindowThreadProcessId   = user32.NewProc("GetWindowThreadProcessId")
	procSendInput                  = user32.NewProc("SendInput")
	procMessageBoxW                = user32.NewProc("MessageBoxW")
	procRegisterWindowMessageW     = user32.NewProc("RegisterWindowMessageW")
	procCreatePopupMenu            = user32.NewProc("CreatePopupMenu")
	procAppendMenuW                = user32.NewProc("AppendMenuW")
	procTrackPopupMenu             = user32.NewProc("TrackPopupMenu")
	procDestroyMenu                = user32.NewProc("DestroyMenu")
	procSetForegroundWindow        = user32.NewProc("SetForegroundWindow")
	procGetCursorPos               = user32.NewProc("GetCursorPos")
	procLoadIconW                  = user32.NewProc("LoadIconW")
	procGetModuleHandleW           = kernel32.NewProc("GetModuleHandleW")
	procOpenProcess                = kernel32.NewProc("OpenProcess")
	procQueryFullProcessImageNameW = kernel32.NewProc("QueryFullProcessImageNameW")
	procCloseHandle                = kernel32.NewProc("CloseHandle")
	procCreateMutexW               = kernel32.NewProc("CreateMutexW")
	procShellNotifyIconW           = shell32.NewProc("Shell_NotifyIconW")
	procHidPGetCaps                = hid.NewProc("HidP_GetCaps")
	procHidPGetValueCaps           = hid.NewProc("HidP_GetValueCaps")
	procHidPGetUsageValue          = hid.NewProc("HidP_GetUsageValue")
	procHidPGetUsages              = hid.NewProc("HidP_GetUsages")
)

const (
	wmDestroy      = 0x0002
	wmInput        = 0x00FF
	wmLButtonUp    = 0x0202
	wmRButtonUp    = 0x0205
	wmTrayIcon     = 0x8000 + 1
	ridevInputSink = 0x00000100
	ridInput       = 0x10000003
	rimTypeHID     = 2
	ridiPreparsed  = 0x20000005
	rawHeaderSize  = 24

	hidpInput                    = 0
	hidpStatusSuccess            = 0x00110000
	hidpStatusUsageNotFound      = 0xC0110004
	hidpStatusIncompatibleReport = 0xC011000A

	nimAdd     = 0
	nimDelete  = 2
	nifMessage = 0x1
	nifIcon    = 0x2
	nifTip     = 0x4

	idiApplication = 32512
	mfString       = 0
	tpmReturnCmd   = 0x0100
	tpmNoNotify    = 0x0080
	menuQuitID     = 1

	inputKeyboard                  = 1
	keyEventKeyUp                  = 0x2
	vkControl                      = 0x11
	vkOemPlus                      = 0xBB
	vkOemMinus                     = 0xBD
	processQueryLimitedInformation = 0x1000
	errorAlreadyExists             = 183
)

type point struct{ x, y int32 }

type wndClassEx struct {
	cbSize        uint32
	style         uint32
	lpfnWndProc   uintptr
	cbClsExtra    int32
	cbWndExtra    int32
	hInstance     uintptr
	hIcon         uintptr
	hCursor       uintptr
	hbrBackground uintptr
	lpszMenuName  *uint16
	lpszClassName *uint16
	hIconSm       uintptr
}

type msg struct {
	hwnd     uintptr
	message  uint32
	wParam   uintptr
	lParam   uintptr
	time     uint32
	pt       point
	lPrivate uint32
}

type rawInputDevice struct {
	usagePage uint16
	usage     uint16
	flags     uint32
	target    uintptr
}

type notifyIconData struct {
	cbSize           uint32
	hWnd             uintptr
	uID              uint32
	uFlags           uint32
	uCallbackMessage uint32
	hIcon            uintptr
	szTip            [128]uint16
	dwState          uint32
	dwStateMask      uint32
	szInfo           [256]uint16
	uVersion         uint32
	szInfoTitle      [64]uint16
	dwInfoFlags      uint32
	guidItem         [16]byte
	hBalloonIcon     uintptr
}

// keyboardInput is the Win32 INPUT struct with its KEYBDINPUT union member, padded to INPUT's size.
type keyboardInput struct {
	inputType   uint32
	_           uint32
	wVk         uint16
	wScan       uint16
	dwFlags     uint32
	time        uint32
	dwExtraInfo uintptr
	_           [8]byte
}

type hidpCaps struct {
	usage                     uint16
	usagePage                 uint16
	inputReportByteLength     uint16
	outputReportByteLength    uint16
	featureReportByteLength   uint16
	reserved                  [17]uint16
	numberLinkCollectionNodes uint16
	numberInputButtonCaps     uint16
	numberInputValueCaps      uint16
	numberInputDataIndices    uint16
	numberOutputButtonCaps    uint16
	numberOutputValueCaps     uint16
	numberOutputDataIndices   uint16
	numberFeatureButtonCaps   uint16
	numberFeatureValueCaps    uint16
	numberFeatureDataIndices  uint16
}

type hidpValueCaps struct {
	usagePage         uint16
	reportID          uint8
	isAlias           uint8
	bitField          uint16
	linkCollection    uint16
	linkUsage         uint16
	linkUsagePage     uint16
	isRange           uint8
	isStringRange     uint8
	isDesignatorRange uint8
	isAbsolute        uint8
	hasNull           uint8
	reserved          uint8
	bitSize           uint16
	reportCount       uint16
	reserved2         [5]uint16
	unitsExp          uint32
	units             uint32
	logicalMin        int32
	logicalMax        int32
	physicalMin       int32
	physicalMax       int32
	usageMin          uint16 // NotRange.Usage when isRange == 0
	usageMax          uint16
	rest              [6]uint16
}

func utf16Ptr(s string) *uint16 {
	p, err := syscall.UTF16PtrFromString(s)
	if err != nil {
		fatal("UTF16PtrFromString failed for %q: %v", s, err)
	}
	return p
}

func fatal(format string, args ...any) {
	text := fmt.Sprintf(format, args...)
	procMessageBoxW.Call(0, uintptr(unsafe.Pointer(utf16Ptr(text))), uintptr(unsafe.Pointer(utf16Ptr("Claude Pinch Zoom"))), 0x10)
	exit(1)
}

// Compile-time checks that the structs match the 64-bit Win32 layouts.
var (
	_ [72]byte  = [unsafe.Sizeof(hidpValueCaps{})]byte{}
	_ [64]byte  = [unsafe.Sizeof(hidpCaps{})]byte{}
	_ [40]byte  = [unsafe.Sizeof(keyboardInput{})]byte{}
	_ [976]byte = [unsafe.Sizeof(notifyIconData{})]byte{}
	_ [16]byte  = [unsafe.Sizeof(rawInputDevice{})]byte{}
	_ [48]byte  = [unsafe.Sizeof(msg{})]byte{}
	_ [80]byte  = [unsafe.Sizeof(wndClassEx{})]byte{}
)
