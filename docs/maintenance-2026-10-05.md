# StashBooru 1.0 maintenance pass — 2026-10-05

Baseline: `develop` at `5162463028e69420f32c7f5ff629bb36b9057008`, after PR #139.
Branch: `maintenance/features-ui-settings-20261005`.
Owner scope: clean up the added features, clarify their operation, unify settings
and styling, then publish StashBooru 1.0 with independent update checking.

## Settings and feature cleanup

- Processing owns worker, similarity/tagging, conversion and upscaling settings.
  Inheritance lives in Library; System contains native server configuration.
  Legacy System bookmarks redirect with query parameters and section anchors intact.
  Initial asynchronous layout follows an anchor until the user starts interacting.
- Settings panels load on first visit and preserve drafts across tab changes.
  Merely opening another settings category no longer contacts processing workers.
- One tagging Save covers thresholds, category limits and filename metadata.
  Status refresh preserves edits; failed initial configuration remains retryable.
  Filename extraction is clearly independent of optional Camie tagging.
- Browser upscaling defaults and server model paths save independently. A failed
  server configuration read cannot silently overwrite paths or block browser defaults.
- Worker URL/token fields have visible labels and native dark-theme inputs.
  Optional model details, re-downloads and trial-cache details use disclosures.
  Shared Run on choices use the same backend values/labels across processing tools.
- Native theme colors replace hardcoded stack, overlap and restoration colors.
  Responsive actions wrap and settings/error content stays within the viewport.
- Corrected help: pHash is the default Find similar mode; EVA02 is the related-content
  mode. Upscaling creates copies by default; restore applies to explicit replacements.
  [Feature guide](features.md) documents workflow, save scope, originals and limitations.

## Removed and deferred

| Item | Decision | Reason |
| --- | --- | --- |
| Duplicate tagging / filename Save controls | Removed | They saved the same configuration and implied separate operations. |
| Prominent repeated model download controls | Collapsed under model details | Setup/re-download is occasional maintenance; status and job actions stay visible. |
| Outdated EVA02-default and Camie-only filename wording | Replaced | It contradicted the current two-mode search and filename behavior. |
| `structural_role` metadata | Recommend staged retirement review | No direct non-generated UI consumer was found; persisted data and GraphQL compatibility require a migration/deprecation decision. |
| `primary_copyright` | Retained | The backend still uses it to order associations; it is not dead data. |
| Built-in Unified Media | Retained | Entity pages still use it; removing it would regress working views. |
| P12/P13 storage prototypes | Retained as offline tools | Production activation gates remain deliberate, documented limitations. |
| Presets and CSV/JSON reviews | Deferred | Feature freeze after the approved three-item QoL batch. |

No metadata deletion, database migration, media rewrite or production storage
activation is part of this pass.

## Independent release identity

One version file supplies Go and Vite with **StashBooru 1.0.0**. About shows the
upstream Stash build separately. Startup and About check stable fork releases,
compare numeric versions, reject unrelated/prerelease tags and offer a platform
download (or release page). Failure remains retryable and is never reported as
up to date. Installation is manual.

Stable tags use `stashbooru-vX.Y.Z`; upstream `v*` tags keep their meaning. The
release workflow builds and tests before publishing, uploads all assets into a
draft, supplies SHA-256 checksums and refuses existing tags or uncertain tag lookup.
Its release branch must exactly match the version file. UI cache identity includes
the version/config/commit to prevent stale release labels or hashes.
See [release procedure](releases.md) and [1.0 notes](releases/stashbooru-1.0.0.md).

## Verification

- Node 20.20.1 / pnpm 10.33.0: `make validate-ui` passed **47 tests**, JS/CSS lint,
  TypeScript and formatting. `make ui-only` passed production bundling and built-in
  preparation. The final About label received an additional mounted verification.
- Go 1.25.13: backend generation passed; `go test ./internal/build` and the focused
  `TestStashBooruReleaseVersion` API test passed with the repository's SQLite include
  paths. GitHub's full Go integration/lint and platform matrix remain release gates.
- Five isolated release-script tests cover asset checksums, draft-before-publication,
  existing tag/network failure, incorrect repository/branch/commit, missing binary
  and upload failure. No test contacts GitHub or publishes a release.
- Mounted Chromium 143.0.7499.0, desktop 1440×1000 and mobile 390×844: actual Settings
  with mocked Apollo/REST, save scopes, status refresh, drafts, lazy initialization,
  bookmark redirects/scroll, local setup failures, numeric update states/retry,
  stale response cleanup and horizontal overflow. Fixture TypeScript passes separately.
- Existing mounted Jobs/stacks, Video overlap and image restoration suites pass
  at both viewport sizes. Overlap uses synthetic H.264 streams; restoration uses
  synthetic PNGs and mocked inference. These are automated fixtures, not human
  manual tests or acceptance of real model/GPU output.
- Native Bootstrap/Sass deprecation notices remain existing dependency debt.
  This pass does not update the UI framework or dependency locks.

## Screenshots

Fixtures use synthetic settings and worker responses.

| View | Before | After |
| --- | --- | --- |
| Desktop settings | [System](screenshots/maintenance-1.0/before-settings-desktop.png) | [Processing](screenshots/maintenance-1.0/processing-settings-desktop.png) |
| Mobile settings | [System](screenshots/maintenance-1.0/before-settings-mobile.png) | [Processing](screenshots/maintenance-1.0/processing-settings-mobile.png) |

[About desktop](screenshots/maintenance-1.0/about-desktop.png) ·
[About mobile](screenshots/maintenance-1.0/about-mobile.png) ·
[Upscaling mobile](screenshots/maintenance-1.0/upscaling-settings-mobile.png)
