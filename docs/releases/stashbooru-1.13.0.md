# StashBooru 1.13.0

Web UI: Image Tagging and Booru browser improvements. This release also covers
1.9.0 to 1.12.0, which were not published separately. The native app is
unchanged from 1.8.0 apart from its version number.

## Added

- **Tag many Images at once.** Select two or more Images and choose **Image
  Tagging…** to tag the whole selection in one confirmed job. Only local
  filename metadata starts ticked and it needs no other setting; every other
  source is an explicit choice for each job.
- **MD5 lookup in Image Tagging jobs.** A job can tag a selection from the
  posts it finds by MD5 and add each post's link to the Image's URLs. To spare
  the sites, a job asks one site at a time, about once a second each, and
  stops at the first match. It skips Images that already link to a chosen
  site, and stops asking a site that rate limits or keeps failing. Tags a site
  gives no category for are not applied.
- **PixAI in Image Tagging jobs**, alongside Camie and EVA02. All three need
  **Allow PixAI, Camie and EVA02 to tag several selected Images in one job**
  in tagging setup.
- **Sites for MD5 lookup** (Settings → Processing → Similarity and tagging):
  choose which Booru browser sources Image and Video Tagging ask, and in what
  order. Any source that can search by MD5 can be used, including e621, and
  saved credentials are used. A match must have the exact MD5.
- **Save the post link.** An MD5 match adds the post's link to the media's
  URLs on Apply; untick **Add the post link** to skip it.
- **Duplicate Character warnings.** Image Tagging warns when the selection
  holds the same Character with the name reversed, or several Characters
  while the tags (`solo`, `1girl`, …) say the Image shows one person. Nothing
  is merged or deleted.
- **Booru browser: collapsible ZIP archives and media groups.** A ZIP in a
  post has a header that collapses its files, and a post with more than four
  media files gets a "N media files" header.
- **Keyboard selection in Booru posts.** **Shift+↓** and **Shift+↑** move a
  highlighted selection through every media file, including files inside a
  ZIP. **↑** downloads the highlighted file; **←/→** still browse posts.

## Changed

- **Review metadata is now Image Tagging** (Video Tagging for Videos; Image
  & Video Tagging for mixed selections).
- **Characters match in either name order.** A suggested "Miku Hatsune" finds
  your existing "Hatsune Miku" when only one Character has that name.
- **One-person Images in jobs.** When the tags say an Image shows one person
  but the models suggest several Characters, a job (and automatic tagging
  after a Booru download) skips those model Characters and logs it.
  Characters from the filename or an MD5 match are kept.
- The tagging dialog shows how many local filename items it found.

## Downloads

As in 1.8.0: the server downloads, `stashbooru-app-android-arm64-v8a.apk`
(most phones), `stashbooru-app-linux-x64.tar.gz` (needs `libmpv2`) and
`stashbooru-app-windows-x64.zip`. SHA-256 checksums are in `CHECKSUMS_SHA256`.

## Scope and limitations

Checked with unit, integration and mounted browser tests on synthetic data.
Not checked: MD5 lookups against the real sites, a real PixAI worker, large
real ZIP archives, and libraries with many same-name Characters. MD5 lookup
counts for a job are only in the log.
