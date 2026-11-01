# Claude Pinch Zoom

Trackpad pinch-to-zoom for the Claude desktop app on macOS.

Claude desktop ignores pinch gestures. This tiny menu-bar helper watches for pinches while Claude is the front app and sends Cmd+= / Cmd+- (the app's own zoom shortcuts). It does nothing in other apps.

## Install

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

## Rebuilding

The default build is ad-hoc signed, so every rebuild gets a new signature and macOS forgets the Accessibility approval. Either reset it after a rebuild:

```bash
tccutil reset Accessibility org.jacobrakai.ClaudePinchZoom
```

or build with your own signing certificate so the approval sticks, by adding `CODE_SIGN_IDENTITY="Apple Development"` to the `xcodebuild` command.

## Tuning

`stepThreshold` in `Sources/main.swift` sets how much pinch makes one zoom step. Raise it if zoom jumps too fast.

The Xcode project is generated from `project.yml` with [XcodeGen](https://github.com/yonaskolb/XcodeGen) (`xcodegen generate`).
