# StashBooru 1.17.0

Web UI and server: Ignored metadata, connecting uncategorised Booru tags, and
label and display fixes. This release also covers 1.14.0 to 1.16.0, which
were not published separately. The native app is unchanged from 1.13.0 apart
from its version number.

This release upgrades the database (migration 104 adds the Ignored flag).
Back up your database before upgrading, as with any migration.

## Added

- **Ignore Tags, Characters, Artists and Copyrights you don't want.** Tick
  **Ignored** on an edit page (also in the bulk edit dialogs for Tags,
  Characters and Artists). An ignored item is hidden from lists and
  selectors. Booru downloads, Image and Video Tagging, bulk tagging and Auto
  Tag never add or recreate it, and tagging review doesn't suggest it. The
  **Ignored** filter on each list finds it again.
- **Connect Booru tags that have no category.** Sites such as Paheal send
  every tag without a category. In the Booru browser such a tag now offers
  **Connect as Character / Artist / Copyright**. Once connected, it is listed
  in that section on every post from the site, and downloads link it to that
  record instead of creating a Tag.
- **Turn duplicate Character detection on or off.** Tagging setup has
  **Detect duplicate Characters** with a checkbox for each mode: reversed
  names, and several Characters on a solo Image.
- **Copyrights filter on Images and Videos**, as All already had.

## Changed

- **Fewer false alarms on solo Images.** The one-person check needs the
  `solo` tag (`1girl` alone is not enough), and is skipped when tags such as
  `cosplay`, `solo_focus`, `multiple_views` or a group count allow several
  Characters. Two names for the same existing Character count once.
- **Sort and filter names say what they do.** Images: **Similarity (pHash
  or EVA02)**; Videos: **Similarity (pHash)**. The pHash filter is
  **Perceptual Hash (pHash)**, and codes are labelled **Code** instead of
  "Artist Code". Saved filters and links keep working.
- Copyrights no longer list Created At and Updated At twice in Sort by.

## Fixed

- **Image and Video details list only their own Copyrights.** The details
  panel added every Copyright of each Character shown, so unrelated series
  appeared there. It now shows the media's own Copyrights and their parents,
  matching filters and Copyright pages. Character cards still list each
  Character's own Copyrights.

## Downloads

As in 1.13.0: the server downloads, `stashbooru-app-android-arm64-v8a.apk`
(most phones), `stashbooru-app-linux-x64.tar.gz` (needs `libmpv2`) and
`stashbooru-app-windows-x64.zip`. SHA-256 checksums are in `CHECKSUMS_SHA256`.

## Scope and limitations

Checked with unit, integration and mounted browser tests on synthetic data,
including a database upgrade test.
- Ignored: media that already has an ignored item keeps it; the Booru browser
  still shows ignored tags on posts; scrapers and Identify are not covered;
  Tags passed on through a Character, Artist or Copyright still apply.
- Connected tags: Images downloaded before connecting keep the Tag; Image
  Tagging's MD5 lookup does not use these connections yet.
- The one-person duplicate check is a heuristic; an untagged second person
  can still trigger it, which is why each mode can be turned off.
