# StashBooru 1.8.0

Web UI: "On this day" on the main page and dates for Images and Videos. These
notes also cover the Following and main-page changes that were already in the
1.6.2 and 1.7.0 downloads but missing from their notes. The native app is
unchanged from 1.7.0.

## Added

- **On this day** on the main page: Images or Videos from today's date in
  earlier years, newest first. Open **Edit main page** and add "On this day:
  Images" or "On this day: Videos". The row hides itself on days with nothing.
- **Images and Videos get a date.** New Booru downloads fill in an empty Date:
  when the photo was taken (EXIF, Images only), otherwise when it was posted on
  the site. A Date you already set is never changed.
- **Fill missing dates** (Settings → Tasks → Library) does the same for the
  existing library, using saved Booru download records. Items with neither
  date use the date they were added.
- **Followed posts** tab on Artists, Characters and Copyrights: **Check new
  posts** checks every site they're followed on and lists the posts, each
  marked with its site's icon.
- **Select and mark posts** seen or unseen together, or **Mark all seen**.
- **Mark followed posts seen once shown** (Following → Follows & settings, or
  Booru settings; off by default), including in the Booru browser.

## Changed

- **Following** opens on your follows and settings; **Posts from follows**
  lists posts from every follow.
- When the saved-post cache is full, the oldest posts you've already seen make
  room instead of stopping checks. Thumbnails are still loaded from the sites,
  not saved.
- Main-page rows saved before the renaming say Videos, Characters, Artists and
  Collections. **Edit main page** is a button at the top of the main page.
- Sorting by Date and the "Recently Released" rows change for items that get a
  Date.

## Downloads

As in 1.7.0: the server downloads, `stashbooru-app-android-arm64-v8a.apk`
(most phones), `stashbooru-app-linux-x64.tar.gz` (needs `libmpv2`) and
`stashbooru-app-windows-x64.zip`. SHA-256 checksums are in `CHECKSUMS_SHA256`.

## Scope and limitations

Checked with unit, integration and mounted browser tests on synthetic data.
Not checked: EXIF dates on real photos, the sites' real date strings, how long
Fill missing dates takes on a large library, and the new rows on a real main
page. "On this day" has no "View all" link yet.
