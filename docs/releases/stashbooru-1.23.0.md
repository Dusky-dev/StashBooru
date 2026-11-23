# StashBooru 1.23.0

Web UI and server: a Twitter / X source in the Booru browser, and Artist
profile pictures moved into **Set image** with Pixiv and X added. This release
also covers 1.22.0, which was not published separately. The native app is
unchanged from 1.21.0 apart from its version number.

There is no database migration in this release.

## Added

- **Twitter / X in the Booru browser.** A new **Twitter / X** source browses:
  - `@handle` or `user:123`: an account's posts with images or videos,
    without retweets and replies;
  - `id:123`: one post;
  - words: a recent search, which X limits to the last 7 days.

  Photos download at original size and videos as MP4. A post's images are its
  media files. The source needs your own X API bearer token in its API key
  setting; X API access is paid or credit-based.
- **Pixiv and X profile pictures.** Connected Pixiv accounts (with the Pixiv
  OAuth token set) and X accounts offer their profile picture, as Pawchive and
  Kemono accounts already did.

## Changed

- **Artist pictures are chosen under Set image.** While editing an Artist,
  **Set image** lists **From … profile picture** for each connected creator
  account. Choosing one loads the picture into the form, and **Save** keeps
  it, like any other image. The Identities section no longer has its own
  picture buttons, and nothing is saved until you press Save.

## Downloads

As in 1.21.0: the server downloads, `stashbooru-app-android-arm64-v8a.apk`
(most phones), `stashbooru-app-linux-x64.tar.gz` (needs `libmpv2`) and
`stashbooru-app-windows-x64.zip`. SHA-256 checksums are in `CHECKSUMS_SHA256`.

## Scope and limitations

Checked with unit tests and mounted browser tests on synthetic data.
- Twitter / X and Pixiv profile pictures were tested with simulated responses
  only; no live X or Pixiv request was made. Some X API details (search
  operators and media field names) follow the X documentation and have not
  been seen in a live response.
- X pages by cursor: the browser remembers each page it opens, and a page more
  than five past the last one opened is refused until the earlier pages are
  opened.
