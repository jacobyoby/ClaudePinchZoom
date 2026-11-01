//go:build windows

package main

import (
	"time"
	"unsafe"
)

const (
	pageGenericDesktop = 0x01
	pageDigitizer      = 0x0D
	usageX             = 0x30
	usageY             = 0x31
	usageTipSwitch     = 0x42
	usageContactID     = 0x51
	usageContactCount  = 0x54
	usageTouchScreen   = 0x04
	usageTouchPad      = 0x05
	// contactTimeout drops a finger whose lift report was missed.
	contactTimeout = 150 * time.Millisecond
)

type axis struct {
	logMin, logMax, physMin, physMax int32
}

// scale maps a raw value to physical units, so X and Y distances are comparable.
func (a axis) scale(raw uint32) float64 {
	logSpan := float64(a.logMax - a.logMin)
	physSpan := float64(a.physMax - a.physMin)
	if physSpan <= 0 {
		physSpan = logSpan
	}
	return (float64(int32(raw)-a.logMin) / logSpan) * physSpan
}

type finger struct {
	link         uint16
	x, y         axis
	hasContactID bool
}

type hidDevice struct {
	preparsed        []byte
	fingers          []finger
	hasContactCount  bool
	contactCountLink uint16
	contacts         map[uint32]trackedContact
	pinch            pinchState
}

type trackedContact struct {
	contact
	seen time.Time
}

func callHid(name string, status uintptr) uint32 {
	s := uint32(status)
	if s != hidpStatusSuccess && s != hidpStatusUsageNotFound && s != hidpStatusIncompatibleReport {
		fatal("%s failed: NTSTATUS 0x%08X", name, s)
	}
	return s
}

// loadDevice reads the device's HID descriptor and finds its finger collections.
func loadDevice(handle uintptr) *hidDevice {
	var size uint32
	if r, _, err := procGetRawInputDeviceInfoW.Call(handle, ridiPreparsed, 0, uintptr(unsafe.Pointer(&size))); r != 0 {
		fatal("GetRawInputDeviceInfoW size query failed for device 0x%X: %v", handle, err)
	}
	preparsed := make([]byte, size)
	if r, _, err := procGetRawInputDeviceInfoW.Call(handle, ridiPreparsed, uintptr(unsafe.Pointer(&preparsed[0])), uintptr(unsafe.Pointer(&size))); int32(r) <= 0 {
		fatal("GetRawInputDeviceInfoW failed for device 0x%X: %v", handle, err)
	}
	prep := uintptr(unsafe.Pointer(&preparsed[0]))

	var caps hidpCaps
	r, _, _ := procHidPGetCaps.Call(prep, uintptr(unsafe.Pointer(&caps)))
	if uint32(r) != hidpStatusSuccess {
		fatal("HidP_GetCaps failed for device 0x%X: NTSTATUS 0x%08X", handle, uint32(r))
	}
	device := &hidDevice{preparsed: preparsed, contacts: map[uint32]trackedContact{}}
	if caps.numberInputValueCaps == 0 {
		return device
	}
	valueCaps := make([]hidpValueCaps, caps.numberInputValueCaps)
	count := caps.numberInputValueCaps
	r, _, _ = procHidPGetValueCaps.Call(hidpInput, uintptr(unsafe.Pointer(&valueCaps[0])), uintptr(unsafe.Pointer(&count)), prep)
	if uint32(r) != hidpStatusSuccess {
		fatal("HidP_GetValueCaps failed for device 0x%X: NTSTATUS 0x%08X", handle, uint32(r))
	}

	xs, ys, ids := map[uint16]axis{}, map[uint16]axis{}, map[uint16]bool{}
	order := []uint16{}
	for _, vc := range valueCaps[:count] {
		if vc.isRange != 0 {
			continue
		}
		a := axis{vc.logicalMin, vc.logicalMax, vc.physicalMin, vc.physicalMax}
		switch {
		case vc.usagePage == pageGenericDesktop && vc.usageMin == usageX:
			if _, seen := xs[vc.linkCollection]; !seen {
				order = append(order, vc.linkCollection)
			}
			xs[vc.linkCollection] = a
		case vc.usagePage == pageGenericDesktop && vc.usageMin == usageY:
			ys[vc.linkCollection] = a
		case vc.usagePage == pageDigitizer && vc.usageMin == usageContactID:
			ids[vc.linkCollection] = true
		case vc.usagePage == pageDigitizer && vc.usageMin == usageContactCount:
			device.hasContactCount = true
			device.contactCountLink = vc.linkCollection
		}
	}
	for _, link := range order {
		y, ok := ys[link]
		if !ok {
			continue
		}
		device.fingers = append(device.fingers, finger{link: link, x: xs[link], y: y, hasContactID: ids[link]})
	}
	return device
}

func (d *hidDevice) usageValue(report []byte, page, link, usage uint16) (uint32, bool) {
	var value uint32
	r, _, _ := procHidPGetUsageValue.Call(hidpInput, uintptr(page), uintptr(link), uintptr(usage),
		uintptr(unsafe.Pointer(&value)), uintptr(unsafe.Pointer(&d.preparsed[0])),
		uintptr(unsafe.Pointer(&report[0])), uintptr(len(report)))
	return value, callHid("HidP_GetUsageValue", r) == hidpStatusSuccess
}

func (d *hidDevice) tipDown(report []byte, link uint16) bool {
	usages := make([]uint16, 32)
	length := uint32(len(usages))
	r, _, _ := procHidPGetUsages.Call(hidpInput, pageDigitizer, uintptr(link),
		uintptr(unsafe.Pointer(&usages[0])), uintptr(unsafe.Pointer(&length)),
		uintptr(unsafe.Pointer(&d.preparsed[0])), uintptr(unsafe.Pointer(&report[0])), uintptr(len(report)))
	if callHid("HidP_GetUsages", r) != hidpStatusSuccess {
		return false
	}
	for _, u := range usages[:length] {
		if u == usageTipSwitch {
			return true
		}
	}
	return false
}

// applyReport updates the device's tracked contacts from one HID input report.
func (d *hidDevice) applyReport(report []byte, now time.Time) {
	slots := len(d.fingers)
	// In parallel mode the contact count says how many finger slots hold real data;
	// in hybrid mode it is zero on continuation reports, which carry a single slot.
	if d.hasContactCount {
		if n, ok := d.usageValue(report, pageDigitizer, d.contactCountLink, usageContactCount); ok && n > 0 && int(n) < slots {
			slots = int(n)
		}
	}
	for _, f := range d.fingers[:slots] {
		rawX, okX := d.usageValue(report, pageGenericDesktop, f.link, usageX)
		rawY, okY := d.usageValue(report, pageGenericDesktop, f.link, usageY)
		if !okX || !okY {
			continue
		}
		id := uint32(f.link)
		if f.hasContactID {
			if v, ok := d.usageValue(report, pageDigitizer, f.link, usageContactID); ok {
				id = v
			}
		}
		if !d.tipDown(report, f.link) {
			delete(d.contacts, id)
			continue
		}
		d.contacts[id] = trackedContact{contact{id: id, x: f.x.scale(rawX), y: f.y.scale(rawY)}, now}
	}
}

// activeContacts returns fingers still down, dropping ones not reported within contactTimeout.
func (d *hidDevice) activeContacts(now time.Time) []contact {
	active := []contact{}
	for id, c := range d.contacts {
		if now.Sub(c.seen) > contactTimeout {
			delete(d.contacts, id)
			continue
		}
		active = append(active, c.contact)
	}
	return active
}
