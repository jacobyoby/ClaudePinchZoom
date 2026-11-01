//go:build windows

// Command ClaudePinchZoom turns touchpad and touchscreen pinch gestures into Ctrl+= / Ctrl+-
// while the Claude desktop app is the foreground window, since Claude ignores pinch on Windows.
//
// Pinch gestures go straight to the window under the fingers, so this reads raw finger
// positions from precision touchpads and touchscreens (HID digitizers) and detects the pinch itself.
package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

const claudeExe = "claude.exe"

var (
	devices = map[uintptr]*hidDevice{}
	tray    notifyIconData
	// taskbarCreated is broadcast when Explorer restarts; tray icons must be added again.
	taskbarCreated uintptr
)

func registerWindowMessage(name string) uintptr {
	id, _, err := procRegisterWindowMessageW.Call(uintptr(unsafe.Pointer(utf16Ptr(name))))
	if id == 0 {
		fatal("RegisterWindowMessageW failed for %q: %v", name, err)
	}
	return id
}

func init() {
	// Win32 windows and their message loop must stay on one OS thread.
	runtime.LockOSThread()
}

func exit(code int) {
	if tray.hWnd != 0 {
		procShellNotifyIconW.Call(nimDelete, uintptr(unsafe.Pointer(&tray)))
	}
	os.Exit(code)
}

func claudeIsForeground() bool {
	hwnd, _, _ := procGetForegroundWindow.Call()
	if hwnd == 0 {
		return false
	}
	var pid uint32
	procGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
	handle, _, _ := procOpenProcess.Call(processQueryLimitedInformation, 0, uintptr(pid))
	if handle == 0 {
		// Elevated or protected processes cannot be inspected; they are not Claude.
		return false
	}
	defer procCloseHandle.Call(handle)
	buf := make([]uint16, 1024)
	size := uint32(len(buf))
	if r, _, _ := procQueryFullProcessImageNameW.Call(handle, 0, uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size))); r == 0 {
		// The foreground process exited between the two calls; it is no longer in front.
		return false
	}
	return strings.EqualFold(filepath.Base(syscall.UTF16ToString(buf[:size])), claudeExe)
}

func sendZoom(step zoomStep) {
	key := uint16(vkOemPlus)
	if step == zoomOut {
		key = vkOemMinus
	}
	inputs := []keyboardInput{
		{inputType: inputKeyboard, wVk: vkControl},
		{inputType: inputKeyboard, wVk: key},
		{inputType: inputKeyboard, wVk: key, dwFlags: keyEventKeyUp},
		{inputType: inputKeyboard, wVk: vkControl, dwFlags: keyEventKeyUp},
	}
	sent, _, err := procSendInput.Call(uintptr(len(inputs)), uintptr(unsafe.Pointer(&inputs[0])), unsafe.Sizeof(inputs[0]))
	if int(sent) != len(inputs) {
		fatal("SendInput sent %d of %d key events for zoom step %d: %v", sent, len(inputs), step, err)
	}
}

func handleRawInput(lParam uintptr) {
	var size uint32
	if r, _, err := procGetRawInputData.Call(lParam, ridInput, 0, uintptr(unsafe.Pointer(&size)), rawHeaderSize); int32(r) != 0 {
		fatal("GetRawInputData size query failed: returned %d: %v", int32(r), err)
	}
	if size == 0 {
		return
	}
	buf := make([]byte, size)
	if r, _, err := procGetRawInputData.Call(lParam, ridInput, uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size)), rawHeaderSize); uint32(r) != size {
		fatal("GetRawInputData returned %d, expected %d bytes: %v", int32(r), size, err)
	}
	if *(*uint32)(unsafe.Pointer(&buf[0])) != rimTypeHID {
		return
	}
	handle := *(*uintptr)(unsafe.Pointer(&buf[8]))
	reportSize := *(*uint32)(unsafe.Pointer(&buf[rawHeaderSize]))
	reportCount := *(*uint32)(unsafe.Pointer(&buf[rawHeaderSize+4]))
	data := buf[rawHeaderSize+8:]
	if reportSize == 0 || uint64(reportSize)*uint64(reportCount) > uint64(len(data)) {
		fatal("malformed raw HID input from device 0x%X: report size %d, count %d, %d bytes", handle, reportSize, reportCount, len(data))
	}

	device, ok := devices[handle]
	if !ok {
		device = loadDevice(handle)
		devices[handle] = device
	}
	if len(device.fingers) == 0 {
		return
	}
	now := time.Now()
	for i := uint32(0); i < reportCount; i++ {
		device.applyReport(data[i*reportSize:(i+1)*reportSize], now)
	}

	contacts := device.activeContacts(now)
	if len(contacts) == 2 && !claudeIsForeground() {
		device.pinch = pinchState{}
		return
	}
	step, state := nextPinch(device.pinch, contacts)
	device.pinch = state
	if step != zoomNone {
		sendZoom(step)
	}
}

func showTrayMenu(hwnd uintptr) {
	menu, _, err := procCreatePopupMenu.Call()
	if menu == 0 {
		fatal("CreatePopupMenu failed: %v", err)
	}
	defer procDestroyMenu.Call(menu)
	procAppendMenuW.Call(menu, mfString, menuQuitID, uintptr(unsafe.Pointer(utf16Ptr("Quit Claude Pinch Zoom"))))
	var pt point
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	// Required so the menu closes when the user clicks elsewhere.
	procSetForegroundWindow.Call(hwnd)
	cmd, _, _ := procTrackPopupMenu.Call(menu, tpmReturnCmd|tpmNoNotify, uintptr(pt.x), uintptr(pt.y), 0, hwnd, 0)
	if cmd == menuQuitID {
		procDestroyWindow.Call(hwnd)
	}
}

func wndProc(hwnd, message, wParam, lParam uintptr) uintptr {
	if message == taskbarCreated {
		addTrayIcon(hwnd)
		return 0
	}
	switch message {
	case wmInput:
		handleRawInput(lParam)
	case wmTrayIcon:
		if lParam == wmRButtonUp || lParam == wmLButtonUp {
			showTrayMenu(hwnd)
		}
		return 0
	case wmDestroy:
		procShellNotifyIconW.Call(nimDelete, uintptr(unsafe.Pointer(&tray)))
		tray.hWnd = 0
		procPostQuitMessage.Call(0)
		return 0
	}
	r, _, _ := procDefWindowProcW.Call(hwnd, message, wParam, lParam)
	return r
}

func createWindow(instance uintptr) uintptr {
	className := utf16Ptr("ClaudePinchZoom")
	wc := wndClassEx{lpfnWndProc: syscall.NewCallback(wndProc), hInstance: instance, lpszClassName: className}
	wc.cbSize = uint32(unsafe.Sizeof(wc))
	if r, _, err := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc))); r == 0 {
		fatal("RegisterClassExW failed: %v", err)
	}
	hwnd, _, err := procCreateWindowExW.Call(0, uintptr(unsafe.Pointer(className)), uintptr(unsafe.Pointer(className)),
		0, 0, 0, 0, 0, 0, 0, instance, 0)
	if hwnd == 0 {
		fatal("CreateWindowExW failed: %v", err)
	}
	return hwnd
}

func registerDigitizers(hwnd uintptr) {
	targets := []rawInputDevice{
		{usagePage: pageDigitizer, usage: usageTouchPad, flags: ridevInputSink, target: hwnd},
		{usagePage: pageDigitizer, usage: usageTouchScreen, flags: ridevInputSink, target: hwnd},
	}
	if r, _, err := procRegisterRawInputDevices.Call(uintptr(unsafe.Pointer(&targets[0])), uintptr(len(targets)), unsafe.Sizeof(targets[0])); r == 0 {
		fatal("RegisterRawInputDevices failed for touchpad and touchscreen: %v", err)
	}
}

func addTrayIcon(hwnd uintptr) {
	icon, _, err := procLoadIconW.Call(0, idiApplication)
	if icon == 0 {
		fatal("LoadIconW failed: %v", err)
	}
	tray = notifyIconData{hWnd: hwnd, uFlags: nifMessage | nifIcon | nifTip, uCallbackMessage: wmTrayIcon, hIcon: icon}
	tray.cbSize = uint32(unsafe.Sizeof(tray))
	tip, _ := syscall.UTF16FromString("Claude Pinch Zoom")
	copy(tray.szTip[:], tip)
	if r, _, err := procShellNotifyIconW.Call(nimAdd, uintptr(unsafe.Pointer(&tray))); r == 0 {
		fatal("Shell_NotifyIconW add failed: %v", err)
	}
}

func main() {
	// One instance only; a second would send every zoom step twice.
	mutex, _, err := procCreateMutexW.Call(0, 0, uintptr(unsafe.Pointer(utf16Ptr("Local\\ClaudePinchZoom"))))
	if mutex == 0 {
		fatal("CreateMutexW failed: %v", err)
	}
	if err == syscall.Errno(errorAlreadyExists) {
		fatal("Claude Pinch Zoom is already running (see the tray icon).")
	}
	taskbarCreated = registerWindowMessage("TaskbarCreated")
	instance, _, err := procGetModuleHandleW.Call(0)
	if instance == 0 {
		fatal("GetModuleHandleW failed: %v", err)
	}
	hwnd := createWindow(instance)
	registerDigitizers(hwnd)
	addTrayIcon(hwnd)

	var m msg
	for {
		r, _, err := procGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		switch int32(r) {
		case -1:
			fatal("GetMessageW failed: %v", err)
		case 0:
			exit(0)
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}
}
