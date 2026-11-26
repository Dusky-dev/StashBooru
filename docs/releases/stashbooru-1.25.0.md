# StashBooru 1.25.0

Web UI and server: a customizable Image details panel, a media gallery and
per-media ignore in Booru browser previews, Rule34 tag types, and upstream
Stash updates. This release also covers 1.24.0, which was not published
separately. The native app is unchanged from 1.23.0 apart from its version
number.

This release includes a database migration (schema 105) that adds a
`missing_since` date to files and folders. Back up your database before
upgrading, as for any migration.

## Added

- **Customize Image details.** The sliders button beside the Details / Edit
  tabs lets you remove any part of the details panel (title, tag sections,
  file rows and more) and bring it back later. The choice is kept in this
  browser. The panel can be resized only while customizing.
- **Media gallery in Booru previews.** A post with several media shows them as
  small thumbnails at the top of the preview; choose one to jump to it.
- **Verify and Purge missing tasks** (from upstream Stash). Verify marks files
  that are no longer on disk as missing; Purge missing removes them. They
  replace Clean.
- **Image rating keys** in the Image viewer (from upstream Stash).

## Changed

- **Q ignores the selected media.** In a Booru post with several media, Q
  ignores only the selected one and moves to the next; ignoring the last one
  ignores the post. Each media also has its own ignore button. Posts with one
  media work as before.
- **Steadier preview keys.** Keyboard shortcuts keep working after moving to
  another post.
- **Convert is harder to switch off by accident.** The convert option is a
  labelled switch on its own line, apart from the download buttons.
- **Rule34 MD5 lookups keep tag types.** A Rule34 match lists its Artist,
  Character, Copyright and meta tags as such instead of putting every tag in
  General.
- **Details panel controls.** The file path leads the file details. Hide
  details sits at the Image's top-left corner, where the **Details** button
  appears when the panel is hidden. Filename tokens are a setting (off by
  default): Settings › Interface › Image titles.
- **Upstream Stash updates**, including scraper URL conflict warnings and a
  password length check.

## Downloads

As in 1.23.0: the server downloads, `stashbooru-app-android-arm64-v8a.apk`
(most phones), `stashbooru-app-linux-x64.tar.gz` (needs `libmpv2`) and
`stashbooru-app-windows-x64.zip`. SHA-256 checksums are in `CHECKSUMS_SHA256`.

## Scope and limitations

Checked with unit tests and mounted browser tests on synthetic data.
- The Booru gallery and per-media ignore were not tried against live booru
  sites.
- Verify and Purge missing were not run against a real library.
