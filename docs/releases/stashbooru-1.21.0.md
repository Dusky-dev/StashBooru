# StashBooru 1.21.0

Web UI and server: Rule34 MD5 lookup, Booru browser keyboard controls, a
resizable Image details panel and Artist pictures from creator accounts. This
release also covers 1.18.0 to 1.20.0, which were not published separately. The
native app is unchanged from 1.17.0 apart from its version number.

There is no database migration in this release.

## Added

- **Booru preview keys.** **↑/↓** move the highlight through a post's media
  files, **←/→** change post, **E** downloads the highlighted file and **Q**
  ignores the post. Ignoring moves straight to the next post.
- **Artist pictures from Pawchive and Kemono.** On an Artist page, open
  **Identities**: each connected Pawchive or Kemono creator account has
  **Use … profile picture**, which sets the Artist image from that creator's
  avatar. You are asked before an existing image is replaced.
- **Choose the order of Sites for MD5 lookup.** Ticked sites are numbered in
  the order they are asked; the arrows move a site earlier or later.
- **Filename tokens in Image details.** When an Image's filename matches the
  filename layout in tagging setup, the details list what each token read
  (`%artist%`, `%copyright%`, `%character%`, `%md5%`, `%ext%`).
- **Resize or hide the Image details panel.** On wide screens, drag the edge
  between the panel and the Image (or focus it and use the arrow keys;
  double-click resets it). The arrow button or the **,** key hides and shows
  the panel; on phones it is **Hide details** below the Image. Both are
  remembered.

## Changed

- **One media file open at a time** in Booru previews. Files start collapsed
  except the first; moving the highlight opens that file and collapses the
  previous one. Collapsed rows load no preview.
- **Clearer highlight**: a tinted background and an accent bar instead of a
  thin outline.
- **Convert toggle is an icon** next to **Download to…**, highlighted when
  downloads will be converted with Media converter defaults.
- **Tagging a selection clears it** on Images, Videos and All, once an Image
  Tagging job starts or every reviewed item has had something applied.
  Cancelling keeps the selection.
- Checksums, paths and URLs in Image details wrap instead of being cut off.

## Fixed

- **Rule34 MD5 lookup works.** Rule34 and Safebooru name a post's checksum
  differently and answer "nothing found" with an empty reply; both were
  reported as an invalid response. Matches are now found, no match is
  reported as no match, and the site's own error message is shown when it
  rejects a request.

## Downloads

As in 1.17.0: the server downloads, `stashbooru-app-android-arm64-v8a.apk`
(most phones), `stashbooru-app-linux-x64.tar.gz` (needs `libmpv2`) and
`stashbooru-app-windows-x64.zip`. SHA-256 checksums are in `CHECKSUMS_SHA256`.

## Scope and limitations

Checked with unit tests and mounted browser tests on synthetic data, and the
Rule34-style MD5 lookup live against Safebooru.
- Rule34 itself was not checked with a real account API key.
- Kemono profile pictures were checked live; Pawchive's endpoint was checked
  directly, not from inside the app. Pixiv accounts have no profile picture
  option yet.
