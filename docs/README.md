# StashBooru documentation

Start with the [feature guide](features.md) for the additions to upstream Stash,
or [installation](../README.md#installation) for a new server. The guides below
describe the current code; a development build can contain changes newer than
the latest stable release.

## User guides

| Guide | Covers |
| --- | --- |
| [Feature guide](features.md) | Navigation, mixed browsing, similarity, stacks, processing, diagnostics, animation, local HTTPS and backups. |
| [Metadata and tagging](metadata-and-tagging.md) | Native Copyrights, multiple Artists, Character variants, linked disambiguation, inheritance, filenames, booru lookups and reviewed model predictions. |
| [Booru browser](booru-browser.md) | Read-only multi-source search, five engine families, remote previews/tags, independent paging and server-side source/credential configuration. |
| [Booru review state](booru-review-state.md) | Durable ignored/manual-download marks, bulk undo, provider-page filtering and current-file MD5 evidence. |
| [Media converter](media-converter.md) | Formats, quality/effort/decode speed, animation timing, worker setup, replacement, restore/unrestore and recovery. |
| [Conversion review](p09-conversion-review.md) | Estimates, verified trials, savings minimums, cache expiry and reviewed apply. |
| [Visual stacks](p08-visual-stacks.md) | Create/manage/split/merge, representatives, filtering, ordered filmstrips and reviewed proposals. |
| [Video overlap](p10-video-overlap.md) | Timestamped indexing, duplicate/contained/partial/compilation evidence and segment playback. |
| [Mask restoration](p11-image-restoration.md) | Still-image mask editor, explicit model setup, preview, generated derivatives and limits. |
| [Character cleanup](character-dedup.md) | Conservative same-Copyright matching, conflict handling, preview and cancellable apply. |
| [Worker setup](../scripts/visual_embedding_server.md) | Remote inference, conversion, upscaling, overlap sampling and restoration capabilities. |
| [Jobs and stack shortcuts](qol-jobs-stacks.md) | Failed-job retention, Copy error, representative comparison and draft reordering. |
| [Versions and releases](releases.md) | Independent fork versions, update checks and release maintenance. |

Copyright means a work, series or franchise in booru terminology. It is a native
metadata entity, separate from ordinary Tags. Internally, Videos, Characters,
Artists and Collections still use Stash's Scene, Performer, Studio and Group
names. See the [terminology table](../README.md#terminology).

## Delivery status and experimental tools

The [progress ledger](implementation-progress.md) records P01–P13 and later
maintenance. Their delivered scopes include owner-approved reductions from the
original handoff. In particular:

- P01 supplies backup warnings; additional backup-manifest/restore-contract work
  was explicitly deferred by the owner.
- P02 supplies separate pHash and EVA02 modes. A Combined mode is not present.
- P03 supplies bounded still-image comparison, without automatic geometric
  alignment or whole-animation equality checking.
- P11 is merged; actual model/GPU acceptance remains a separate validation gate.
- [P12 image delta storage](p12-image-delta-storage.md) and
  [P13 Gallery block sharing](p13-gallery-block-storage.md) are offline,
  non-destructive benchmark/pack/verify/extract prototypes. They retain originals;
  they do not transparently replace the application's media storage.

The [remote browser](booru-browser.md) reads provider content;
[B02 review state](booru-review-state.md) persists local ignored/manual-download
marks and checks current indexed files. Disfavored-tag comparison, follows and
Artist × Copyright selection remain planned in the [roadmap](booru-browser-roadmap.md). Existing MD5 metadata
lookup and local entity favorites remain separate workflows.

## Implementation references

| Reference | Purpose |
| --- | --- |
| [Effective associations](p06-effective-associations.md) | Direct/effective relationship contract, provenance, native searches and reviewed inheritance activation. |
| [Global All page](p07-global-media.md) | Native query/list/player architecture and browser fixture instructions. |
| [Progress ledger](implementation-progress.md) | Dated baselines, PRs, checks, limitations and next work package. |
| [October maintenance](maintenance-2026-10-05.md) | Settings scopes, retained compatibility fields and release decisions. |
| [September maintenance](maintenance-2026-09-24.md) | Earlier processing, provenance, HTTPS and gallery-editor review. |
| [Development](DEVELOPMENT.md) / [architecture](ARCHITECTURE.md) | Underlying Stash development and build conventions. |
| [Repository instructions](../AGENTS.md) | Fork-specific implementation, validation and contribution authority. |

Stash's [user documentation](https://docs.stashapp.cc/) remains useful for the
inherited scanning, playback, Galleries, scrapers and plugins. Fork-specific
guides take precedence for the features listed here. StashBooru database backups
are incompatible with vanilla Stash; keep a separate vanilla backup to return to
it, and back up media bytes independently of database-only exports.
