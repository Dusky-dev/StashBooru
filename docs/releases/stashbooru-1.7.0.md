# StashBooru 1.7.0

The native app gains the Booru browser and a Settings page. The server is
unchanged from 1.6.2 apart from this release's note.

## Added

- **Booru browser.** On StashBooru servers, a **Booru** link sits beside the
  gear icon (in the menu on phones), as in the web UI. Pick one of the sites
  set up on the server, search by tags and scroll through the results. Open a
  post to see it larger with its tags grouped by kind; tap a tag to search
  for it, or **Download to library** to have the server save it, using the
  web UI's download folder and conversion settings. Sites, accounts and
  blacklists are still managed in the web UI.
- **Settings.** The gear icon now opens Settings: card size (small, medium,
  large), whether to load full-size images, video autoplay, mute and loop,
  clearing the image cache, and the server's address, version and
  **Disconnect**. These are stored on the device.

## Downloads

As before: `stashbooru-app-android-arm64-v8a.apk` (most phones),
`stashbooru-app-linux-x64.tar.gz` (needs `libmpv2`) and
`stashbooru-app-windows-x64.zip`, plus the server downloads. SHA-256 checksums
are in `CHECKSUMS_SHA256`. The APKs are signed with a development key and
install over earlier versions.

## Scope and limitations

Checked on Linux against a local StashBooru 1.5.1 server using its default
Danbooru source: search, a post page and a download into the library. Android
and Windows were not run. The app does not yet have the web UI's Following,
Identities, Duplicates or editing tools, the booru browser's bookmarks,
favorites, review and archive views, or playback of booru videos before they
are downloaded.
