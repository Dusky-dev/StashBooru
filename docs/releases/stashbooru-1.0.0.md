# StashBooru 1.0

The first stable StashBooru release brings together the merged media, metadata,
comparison and processing features, followed by a settings and UI maintenance pass.
StashBooru's version is independent of its upstream Stash base.

## Included

- Native All media browsing, Character variants, Copyright hierarchies and
  configurable inherited associations.
- pHash and EVA02 image similarity, visual comparison, visual stacks and filmstrip
  shortcuts. Stack reordering supports drag handles and keyboard buttons.
- Conversion estimates, saved verified trials, savings thresholds, reviewed apply
  and restore. Upscaling defaults and worker setup have separate save actions.
- Video overlap evidence and segment review, plus mask-based restoration tooling.
- Failed jobs stay visible in the current tab until dismissed, with Copy error.
- A dedicated Processing settings tab, clearer feature help, shared worker choices
  and consistent native theme styling. Existing settings bookmarks are redirected.
- Settings → About shows StashBooru 1.0.0, the upstream Stash build separately, and
  a stable StashBooru update check with manual download links.

## Before updating

Back up the database, configuration and media using the documented fork-compatible
procedure. Keep processing originals and restore caches until you have reviewed
outputs. This maintenance pass adds no database migration.

Download the binary for your platform; existing `stash-*` asset names are retained
for installation compatibility. SHA-256 checksums are included. The UI-only ZIP is
for the matching server build. Updates are not installed automatically.

## Scope and limitations

Image restoration still requires actual model/GPU acceptance on the owner's setup.
Image delta and Gallery shared-block storage remain offline prototypes; this release
does not activate them on production media. Job retention is browser-tab history,
not durable server history while disconnected. Processing presets and CSV/JSON
review exports remain deferred.

See [the feature guide](https://github.com/Dusky-dev/StashBooru/blob/stashbooru-v1.0.0/docs/features.md)
and [maintenance notes](https://github.com/Dusky-dev/StashBooru/blob/stashbooru-v1.0.0/docs/maintenance-2026-10-05.md).
