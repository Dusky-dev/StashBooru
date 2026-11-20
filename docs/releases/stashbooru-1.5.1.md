# StashBooru 1.5.1

The native app (Android, Linux, Windows) now looks like the web UI. The server
is unchanged from 1.5.0 apart from this release's note.

## Changed

- The app uses the web UI's dark colours, square-cornered cards and buttons,
  and no Material ripples or tinted surfaces.
- PCs get a top navbar in the web order (All, Images, Videos, Characters,
  Artists, Copyrights, Tags, Galleries, Collections); phones get the web's
  bottom bar with a menu button.
- Lists have the web's centred search and sort toolbar and its "1-N of M"
  count. Cards show the whole image with the title underneath; Character,
  Artist, Tag and other cards show their Video and Image counts.
- A Character, Artist or other page opens with its image and name. On a PC,
  Escape goes back, and an item's details open in a side panel.

## Downloads

As in 1.5.0: `stashbooru-app-android-arm64-v8a.apk` (most phones),
`stashbooru-app-linux-x64.tar.gz` (needs `libmpv2`) and
`stashbooru-app-windows-x64.zip`, plus the server downloads. SHA-256 checksums
are in `CHECKSUMS_SHA256`. The APKs are still signed with a development key;
they install over 1.5.0.

## Scope and limitations

Checked on Linux at desktop and phone sizes against a StashBooru server with
sample media. The Windows app and a physical Android phone were not checked.
It still can't edit anything. The 1.5.0 limitations still apply.
