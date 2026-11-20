# StashBooru 1.5.0

The first native StashBooru app, for Android, Linux and Windows. The server is
unchanged from 1.4.0 apart from this release's note.

## The app

A compiled app (not a web page in a wrapper) that connects to your StashBooru
server, or a regular Stash server. Enter the server address and, if the server
has a password, the API key from Settings → Security.

- Browse All, Images, Videos, Characters, Artists, Copyrights, Tags, Galleries
  and Collections, with search, sort and infinite scrolling.
- Swipe or use the arrow keys through media, zoom images, and play Videos in
  the built-in player. `I` shows details; its chips open the related pages.
- Read-only for now: it can't edit anything.

## Downloads

- **Android:** `stashbooru-app-android-arm64-v8a.apk` for almost every current
  phone (`armeabi-v7a` for older 32-bit phones, `x86_64` for emulators).
- **Linux:** `stashbooru-app-linux-x64.tar.gz`. Extract it and run
  `stashbooru`. It needs the system libmpv (`libmpv2` on Debian/Ubuntu).
- **Windows:** `stashbooru-app-windows-x64.zip`. Extract it and run
  `stashbooru.exe`.
- The server downloads are the same as in earlier releases. SHA-256 checksums
  for everything are in `CHECKSUMS_SHA256`.

## Scope and limitations

- The APKs are signed with a development key. A later build signed with a
  release key will need the app uninstalled first (the server address and API
  key are entered again).
- The Windows app and a regular Stash server were not tested while running; the
  Linux app and the APK builds were, against a StashBooru server with sample
  media. Grid thumbnails need *Settings → Thumbnail compression* on JPEG or WebP.
- No database migration since 1.1.0. The limitations listed for 1.0 still apply.
