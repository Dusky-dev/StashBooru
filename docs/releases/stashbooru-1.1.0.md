# StashBooru 1.1

A consolidation release: clearer first use, per-change versioning and a set of
settings, identity and conversion fixes.

## Included

- **Introduction and update notes.** New installs open with an introduction to
  StashBooru. After every update, a short "What's new" note lists the changes;
  older notes stay in Settings → About.
- **Versioning.** Every user-visible change now gets its own StashBooru version,
  shown in Settings → About.
- **Organized flag off by default.** The upstream Organized button, filter and
  bulk-edit option are hidden, and nothing is skipped because of it. Turn it back
  on in Settings → Interface → *Use the Organized flag*.
- **Site icons.** Connected identities show the site's own icon and open the
  Booru browser on that site.
- **Convert again.** Media converted before can be converted again. While the
  original is still in the restore cache, the conversion starts from the
  original; skipped files say why.
- **Settings.** Library, Processing, Metadata Providers, Services and Plugins use
  collapsible groups with short descriptions. "Hierarchy inheritance" is now
  **Include parents**; "Association sources" is **Why these are attached**. The
  log console reads like a terminal.
- **Merges.** Artists and Copyrights merge like Characters and Tags, and booru
  downloads reuse Tags that were merged into another Tag.

## Before updating

Back up the database, configuration and media using the documented
fork-compatible procedure. Download the binary for your platform; SHA-256
checksums are included. Updates are not installed automatically.

## Scope and limitations

The limitations listed for 1.0 still apply (image restoration needs model/GPU
acceptance on the owner's setup; delta and Gallery block storage remain offline
prototypes).
