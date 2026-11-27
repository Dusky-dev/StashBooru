# StashBooru 1.26.0

Web UI and server: Following becomes an experimental feature that is off by
default, and Booru post media gets a List / Gallery switch and shorter media
lists. The native app is unchanged apart from its version number. There is no
database migration.

## Changed

- **Following is experimental and off by default.** Turn it on in Settings ›
  Experimental › Following (shown in Advanced mode). While it is off, the
  Following page, Follow buttons, Followed posts tabs and new-post counts are
  hidden and automatic checks stop. Your follows and check settings are kept,
  so turning it back on picks up where you left off.
- **Gallery or list for Booru post media.** A post with several media has a
  **List / Gallery** switch. Gallery shows every media as rows of thumbnails;
  choose one to see it in the list. The choice is remembered in this browser.
- **Shorter media lists.** A long list shows the first file, the open one with
  the files before and after it, and the last; **…** shows the rest.

## Downloads

As in 1.25.0: the server downloads, `stashbooru-app-android-arm64-v8a.apk`
(most phones), `stashbooru-app-linux-x64.tar.gz` (needs `libmpv2`) and
`stashbooru-app-windows-x64.zip`. SHA-256 checksums are in `CHECKSUMS_SHA256`.

## Scope and limitations

Checked with unit tests and mounted browser tests on synthetic data. The
Gallery view and shorter media lists were not tried against live booru sites.
