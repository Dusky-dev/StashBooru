# StashBooru app

A native client for StashBooru (and stock Stash) on **Android, Linux and
Windows**. It is a compiled Flutter app that draws its own interface, with
video played by libmpv through media_kit. There is no embedded web page.

## Using it

1. Enter the server address, for example `192.168.1.20:9999` (`http://` is
   assumed; a reverse-proxy path such as `https://example.com/stash` works).
2. If the server has a username and password, paste the **API key** from the
   web UI's *Settings → Security*. Leave it empty otherwise.

The app remembers the server. The API key is kept in the Android Keystore or
Windows Credential Manager. On Linux it is kept in an owner-only file
(`~/.local/share/dev.stashbooru.stashbooru/api-key`), because the system
keyring library aborts the app when no keyring service is running.

What it does (read-only for now):

- **All, Images, Videos.** Search, sort (Newest, Oldest, Date, Title, Rating,
  Random), infinite scrolling.
- **Characters, Artists, Copyrights, Tags, Galleries, Collections.** Open one
  to see its media. A Copyright includes its descendants.
- **Viewer.** Swipe, use ←/→ (Page Up/Down on a Video, where ←/→ seek) or the
  side buttons to move. Pinch or double-tap to zoom, `I` for details, Escape
  to close. Chips in the details panel open the related Character, Artist,
  Copyright, Tag, Gallery or Collection. Videos, and Images that are video
  clips, play in the built-in player. Images Flutter cannot decode (JPEG XL,
  and AVIF on some platforms) are shown by the same player, without zoom.
  Grid thumbnails need a format Flutter can decode: keep *Settings → Thumbnail
  compression* on JPEG or WebP.

On a stock Stash server, All and Copyrights should be hidden and everything
else should work, because the app only uses upstream queries there. This has
been tested against mocks only, not a real Stash server.

## Building

Install Flutter 3.50 (the version in `.github/workflows/native-app.yml`).

```bash
flutter pub get
flutter test
```

- **Linux:** needs `clang cmake ninja-build pkg-config libgtk-3-dev
  libsecret-1-dev libmpv-dev`. The bundle links the system libmpv, so the
  machine running it needs libmpv too (`libmpv2` on Debian/Ubuntu). Run `flutter build linux --release`. The
  bundle is in `build/linux/x64/release/bundle/`. Where github.com tarballs
  are unreachable, set `MEDIA_KIT_SYSTEM_MIMALLOC=1` and install
  `libmimalloc-dev`.
- **Windows:** `flutter build windows --release` on Windows with Visual
  Studio's C++ workload.
- **Android:** `flutter build apk --release --split-per-abi` with the Android
  SDK. Release APKs are signed with the debug key until a release keystore is
  configured, so a later build may not install over an earlier one.

CI builds all three on every change under `app/` and uploads them as workflow
artifacts. Each stable StashBooru release attaches them as
`stashbooru-app-*` downloads.

### Testing against a real server

`test/server_test.dart` sends every query the app uses to a live server, and
checks that each entity's counts match the media it opens:

```bash
STASHBOORU_TEST_URL=http://127.0.0.1:9999 STASHBOORU_TEST_KEY=<api key> \
  flutter test test/server_test.dart
```

It is skipped when `STASHBOORU_TEST_URL` is unset.
