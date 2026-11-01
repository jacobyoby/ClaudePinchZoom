import AppKit
import ApplicationServices
import os

/// Turns trackpad pinch gestures into Cmd+= / Cmd+- while Claude desktop is frontmost,
/// since the Claude app ignores pinch on its own.

let claudeBundleID = "com.anthropic.claudefordesktop"
/// Accumulated magnification needed for one zoom step.
let stepThreshold: CGFloat = 0.12
let keyCodeEquals: CGKeyCode = 24
let keyCodeMinus: CGKeyCode = 27
let log = Logger(subsystem: "org.jacobrakai.ClaudePinchZoom", category: "pinch")

enum ZoomStep { case zoomIn, zoomOut }

/// Pure: given the running total and a new delta, returns the steps to emit and the leftover total.
func accumulate(total: CGFloat, delta: CGFloat, threshold: CGFloat) -> (steps: [ZoomStep], remainder: CGFloat) {
    let sum = total + delta
    let count = Int((abs(sum) / threshold).rounded(.down))
    let step: ZoomStep = sum > 0 ? .zoomIn : .zoomOut
    let remainder = sum - CGFloat(count) * threshold * (sum > 0 ? 1 : -1)
    return (Array(repeating: step, count: count), remainder)
}

func claudeIsFrontmost() -> Bool {
    NSWorkspace.shared.frontmostApplication?.bundleIdentifier == claudeBundleID
}

func postZoomKey(step: ZoomStep) {
    let code = step == .zoomIn ? keyCodeEquals : keyCodeMinus
    for keyDown in [true, false] {
        guard let event = CGEvent(keyboardEventSource: nil, virtualKey: code, keyDown: keyDown) else {
            fatalError("CGEvent creation failed for keyCode=\(code) keyDown=\(keyDown)")
        }
        event.flags = .maskCommand
        event.post(tap: .cghidEventTap)
    }
}

/// Mutable pinch total, owned by the event-tap callback on the main run loop.
nonisolated(unsafe) var pinchTotal: CGFloat = 0
nonisolated(unsafe) var eventTap: CFMachPort?

let tapCallback: CGEventTapCallBack = { _, type, cgEvent, _ in
    if type == .tapDisabledByTimeout || type == .tapDisabledByUserInput {
        log.warning("event tap disabled type=\(type.rawValue, privacy: .public); re-enabling")
        if let tap = eventTap { CGEvent.tapEnable(tap: tap, enable: true) }
        return Unmanaged.passUnretained(cgEvent)
    }
    guard let nsEvent = NSEvent(cgEvent: cgEvent), nsEvent.type == .magnify else {
        return Unmanaged.passUnretained(cgEvent)
    }
    guard claudeIsFrontmost() else {
        pinchTotal = 0
        return Unmanaged.passUnretained(cgEvent)
    }
    if nsEvent.phase == .began { pinchTotal = 0 }
    let result = accumulate(total: pinchTotal, delta: nsEvent.magnification, threshold: stepThreshold)
    pinchTotal = nsEvent.phase == .ended || nsEvent.phase == .cancelled ? 0 : result.remainder
    result.steps.forEach(postZoomKey)
    return Unmanaged.passUnretained(cgEvent)
}

func requireAccessibility() {
    let options = ["AXTrustedCheckOptionPrompt": true] as CFDictionary
    let trusted = AXIsProcessTrustedWithOptions(options)
    log.notice("accessibility trusted=\(trusted, privacy: .public)")
    guard trusted else {
        let alert = NSAlert()
        alert.messageText = "Claude Pinch Zoom needs Accessibility access"
        alert.informativeText = "Enable it in System Settings → Privacy & Security → Accessibility, then relaunch."
        alert.runModal()
        exit(1)
    }
}

func installEventTap() -> CFMachPort {
    // NSEvent magnify gestures arrive as CGEventType 29 (kCGEventGesture).
    let mask = CGEventMask(1) << 29
    guard let tap = CGEvent.tapCreate(tap: .cgSessionEventTap, place: .headInsertEventTap,
                                      options: .listenOnly, eventsOfInterest: mask,
                                      callback: tapCallback, userInfo: nil) else {
        fatalError("CGEvent.tapCreate failed: grant Input Monitoring + Accessibility to ClaudePinchZoom")
    }
    let source = CFMachPortCreateRunLoopSource(kCFAllocatorDefault, tap, 0)
    CFRunLoopAddSource(CFRunLoopGetMain(), source, .commonModes)
    CGEvent.tapEnable(tap: tap, enable: true)
    return tap
}


final class StatusItemController: NSObject {
    let item = NSStatusBar.system.statusItem(withLength: NSStatusItem.squareLength)

    func install() {
        item.button?.image = NSImage(systemSymbolName: "plus.magnifyingglass", accessibilityDescription: "Claude Pinch Zoom")
        let menu = NSMenu()
        menu.addItem(withTitle: "Claude Pinch Zoom is on", action: nil, keyEquivalent: "")
        menu.addItem(.separator())
        menu.addItem(withTitle: "Quit", action: #selector(NSApplication.terminate(_:)), keyEquivalent: "q")
        item.menu = menu
    }
}

let app = NSApplication.shared
app.setActivationPolicy(.accessory)
requireAccessibility()
eventTap = installEventTap()
let statusController = StatusItemController()
statusController.install()
log.notice("event tap installed")
app.run()
