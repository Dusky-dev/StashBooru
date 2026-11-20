# StashBooru 1.5.2

Fixes for the native app. The server is unchanged from 1.5.1 apart from this
release's note.

## Fixed

- **Linux: crash when playing.** On systems whose mpv uses LuaJIT (Arch and
  others), an error inside one of mpv's built-in scripts crashed the app,
  because Flutter's bundled exception unwinder was picked up instead of the
  system one. The app now loads the system unwinder first.
- **JPEG XL images.** The app now decodes JPEG XL itself, so stills and
  animations open on Android and Windows (whose mpv has no JPEG XL decoder)
  and JPEG XL thumbnails show in the grid on every platform.

## Downloads

As in 1.5.0: `stashbooru-app-android-arm64-v8a.apk` (most phones),
`stashbooru-app-linux-x64.tar.gz` (needs `libmpv2`) and
`stashbooru-app-windows-x64.zip`, plus the server downloads. SHA-256 checksums
are in `CHECKSUMS_SHA256`. The APKs are signed with a development key and
install over earlier versions.

## Scope and limitations

The Linux crash was reproduced with a LuaJIT test program linked like the old
app (it crashed) and like the new one (it did not); the reporter's own Arch
setup has not been rechecked. JPEG XL decoding was checked against libjxl on
lossy, lossless and recompressed-JPEG files; it is slower than a native
decoder (about a second for a 4-megapixel image on a PC, more on a phone), and
the thumbnail stays visible meanwhile. Android and Windows were not run.
