> **Moved:** this now lives in [jacobrakaiFoundation/claude-mods](https://github.com/jacobrakaiFoundation/claude-mods/tree/main/apps/ClaudePinchZoom), maintained by the Jacobrakai Foundation. This repository is archived; the copy here still works but won't get updates.

# Claude Pinch Zoom

Pinch-to-zoom for the Claude desktop app on macOS and Windows.

Claude desktop ignores pinch gestures. This tiny helper watches for pinches while Claude is the front app and sends the app's own zoom shortcuts (Cmd+= / Cmd+- on macOS, Ctrl+= / Ctrl+- on Windows). It does nothing in other apps. Like those shortcuts, it scales the whole window, not just the text.

## macOS

Requires macOS 14+ and Xcode.

```bash
git clone https://github.com/jacobyoby/ClaudePinchZoom.git
cd ClaudePinchZoom
DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer xcodebuild -project ClaudePinchZoom.xcodeproj -scheme ClaudePinchZoom -configuration Release -derivedDataPath build build
cp -R build/Build/Products/Release/ClaudePinchZoom.app ~/Applications/
open ~/Applications/ClaudePinchZoom.app
```

Or open `ClaudePinchZoom.xcodeproj` in Xcode and press Run.

Then grant **Accessibility** in System Settings → Privacy & Security → Accessibility, and open the app again. A magnifying glass appears in the menu bar. Add it to Login Items to start it automatically.

### Rebuilding

The default build is ad-hoc signed, so every rebuild gets a new signature and macOS forgets the Accessibility approval. Either reset it after a rebuild:

```bash
tccutil reset Accessibility org.jacobrakai.ClaudePinchZoom
```

or build with your own signing certificate so the approval sticks, by adding `CODE_SIGN_IDENTITY="Apple Development"` to the `xcodebuild` command.

### Tuning

`stepThreshold` in `Sources/main.swift` sets how much pinch makes one zoom step. Raise it if zoom jumps too fast.

The Xcode project is generated from `project.yml` with [XcodeGen](https://github.com/yonaskolb/XcodeGen) (`xcodegen generate`).

## Windows (beta)

Not yet tested on real hardware; reports welcome in this repo's issues. Built for precision touchpads and touchscreens on Windows 10/11. Windows sends a pinch straight to the window under your fingers, so the helper reads raw finger positions from the touchpad or touchscreen and detects the pinch itself. It needs no admin rights.

Build it with [Go](https://go.dev/dl/) (on any OS):

```bash
cd windows
GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "-H=windowsgui" -o ClaudePinchZoom.exe .
```

Use `GOARCH=arm64` for ARM PCs. Run `ClaudePinchZoom.exe`; it sits in the system tray (right-click it to quit). To start it with Windows, put a shortcut to it in the folder that opens from Win+R → `shell:startup`.

`stepRatio` in `windows/pinch.go` sets how much pinch makes one zoom step.
