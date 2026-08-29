# GeoDuels Enhancer

This is the official browser extension for GeoDuels. It enhances Street View
gameplay with hidden street names, new movement modes, an in-game compass, and
other features that the GeoDuels web app cannot access across Google's Maps
Embed API iframe boundary.

The app owns all visible UI and launch preferences. The extension:

- runs only inside Google Maps Embed frames opened by GeoDuels;
- reports availability to the GeoDuels lobby and live heading during a match;
- powers the in-game compass;
- applies the match's road-label visibility;
- enforces No Move matches inside the Google frame;
- has no background worker, account access, analytics, or remote code.

## Local installation

The checked-in `manifest.json` includes localhost matches for development.
Store-ready archives should be built with:

```sh
npm run extension:package
```

This creates production Chrome and Firefox ZIP files under
`dist/extensions/geoduels-enhancer/` with localhost permissions removed.

### Chrome

Open `chrome://extensions`, enable Developer mode, choose **Load unpacked**,
and select `dist/extensions/geoduels-enhancer/chrome` after running the package
command. Brave uses the same Chromium extension package at
`brave://extensions`.

### Firefox

Open `about:debugging#/runtime/this-firefox`, choose **Load Temporary
Add-on**, and select `manifest.json`.

Run GeoDuels locally at `http://localhost:3000` or visit `https://geoduels.io`.
The in-game enhancement controls appear only after the Google frame reports
that the extension bridge is available.
