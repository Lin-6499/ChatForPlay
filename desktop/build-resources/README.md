# Installer-only resources

Keep this directory separate from `desktop/assets`: electron-builder excludes
the buildResources directory from the runtime app. The runtime window, tray and
launcher load PNGs from `desktop/assets`, which must be included in app.asar.
The Windows installer icon is explicitly configured as `desktop/assets/icon.ico`.
