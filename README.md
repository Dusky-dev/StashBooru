# StashBooru

[![Build](https://github.com/Dusky-dev/StashBooru/actions/workflows/build.yml/badge.svg?branch=develop)](https://github.com/Dusky-dev/StashBooru/actions/workflows/build.yml)
[![Development builds](https://img.shields.io/badge/builds-latest__develop-blue?logo=github)](https://github.com/Dusky-dev/StashBooru/releases/tag/latest_develop)
[![Docker](https://img.shields.io/badge/docker-GHCR-blue?logo=docker)](https://github.com/Dusky-dev/StashBooru/pkgs/container/stashbooru)
[![License: AGPL-3.0](https://img.shields.io/badge/license-AGPL--3.0-blue.svg)](LICENSE)

**StashBooru is an anime/cartoon-oriented fork of [Stash](https://github.com/stashapp/stash), focused on booru-style metadata, mixed image/video libraries, visual similarity, and assisted tagging.**

It keeps Stash's self-hosted Go backend, media scanning, player, galleries, plugins, scrapers, jobs, and general library-management foundation, while changing the UI and metadata model around anime/cartoon collections.

> [!IMPORTANT]
> StashBooru is an independent fork and is under active development. Fork-specific database migrations and metadata types may diverge from upstream Stash. Back up your database before testing new development builds or moving a library between StashBooru and upstream Stash.

## Terminology

StashBooru uses anime/booru-oriented names in the UI while some upstream-compatible API and internal names remain unchanged.

| StashBooru | Upstream / internal Stash term |
| --- | --- |
| **Video** | Scene |
| **Character** | Performer |
| **Artist** | Studio |
| **Collection** | Group |
| **Copyright** | Series / franchise / source work |

`Copyright` follows common booru tagging terminology: it means the work, series, franchise, or source a Character/media item belongs to. It is not a legal-rights-holder field.

## What StashBooru adds

### Native anime/booru metadata

- **Copyrights are a first-class metadata category**, separate from ordinary Tags.
- Copyrights have artwork, names, aliases, details, **Parent Series** and **Sub-series** relationships.
- Copyrights can be assigned directly to Images, Videos, and Characters.
- Characters can belong to one or more Copyrights, and Copyright Edit includes bulk Character assignment.
- Images and Videos can have **multiple Artists**. The first Artist is still mirrored to the legacy primary Studio field where upstream compatibility requires it.
- Character names such as `Darkness (Konosuba)` can use the suffix as real disambiguation instead of collapsing same-name Characters from different series.

### Unified media browsing

StashBooru is designed around mixed libraries rather than treating every media type as a separate island.

- Unified **All** media views combine relevant Images, Videos, Galleries, and Collections on supported entity pages.
- Grid, List, Wall, and Tagger-style views are available where supported.
- View mode, zoom, media tabs, and reference-comparison preferences are remembered per view.
- Image and Video file information is shown directly in **Details** instead of living in a redundant File Info tab.

### Image Tagging

The old Camie-only metadata workflow has grown into **Image Tagging**, where several sources can be reviewed together before anything is applied.

Supported sources include:

- **Local filename metadata** using a configurable filename layout. Supported tokens include `%artist%`, `%copyright%`, `%character%`, `%md5%`, and `%ext%`.
- **Booru MD5 lookup** against Danbooru, Gelbooru, Yande.re, Konachan, and Safebooru.
- Optional **Camie Tagger v2** inference for Character, Artist, Copyright, general, and meta predictions.

Image Tagging keeps source provenance visible (`local`, `booru`, `Camie`, and existing local entities), lets you choose exactly what to apply, can create missing metadata, and supports bulk tagging jobs. Local filename identity metadata is authoritative for Characters, Artists, and Copyrights when it is present, preventing conflicting Camie identity predictions from being mixed into the same category.

Filename-derived Copyright values can also represent multiple series with `+`, while names containing literal repeated/trailing plus signs such as `C++` are preserved.

### Visual similarity and reference comparison

Image similarity uses an optional **EVA02 embedding index** instead of relying only on pHash.

- Local ONNX inference or a separate remote inference worker.
- Embeddings and Stash metadata remain stored in StashBooru even when inference is remote.
- Image-card **Find similar** uses cosine similarity over the embedding index.
- Large libraries are supported beyond sqlite-vec's 4096-result KNN limit.
- Reference previews support **Selected image**, **Both images**, and a draggable **Slider** comparison mode.
- Comparison pan/zoom is synchronized and the reference remains stationary while browsing matches.
- File size, resolution, duration, and bitrate are shown in comparison views when applicable.

The visual-similarity model is opt-in and is never downloaded silently.

## Installation

### Development builds

StashBooru currently publishes development builds from `develop` to the [`latest_develop`](https://github.com/Dusky-dev/StashBooru/releases/tag/latest_develop) prerelease.

| Platform | Development build |
| --- | --- |
| Windows | [stash-win.exe](https://github.com/Dusky-dev/StashBooru/releases/download/latest_develop/stash-win.exe) |
| macOS | [Stash.app.zip](https://github.com/Dusky-dev/StashBooru/releases/download/latest_develop/Stash.app.zip) |
| Linux x86_64 | [stash-linux](https://github.com/Dusky-dev/StashBooru/releases/download/latest_develop/stash-linux) |
| Other Linux architectures / FreeBSD | [Development release assets](https://github.com/Dusky-dev/StashBooru/releases/tag/latest_develop) |

Once running, the web UI is available at `http://localhost:9999` by default, just like upstream Stash.

### Docker

The `develop` branch publishes an amd64 image to GitHub Container Registry:

```text
ghcr.io/dusky-dev/stashbooru:develop
```

For an existing Stash Docker setup, keep your normal mounts/configuration and replace the image with the StashBooru image. The upstream [Docker documentation](docker/production/README.md) is still useful for the base container layout, but use the StashBooru image above instead of `stashapp/stash`.

### FFmpeg

StashBooru retains Stash's FFmpeg dependency. Linux users should normally install FFmpeg from their distribution package manager; container builds include the runtime pieces expected by the fork.

## Optional tagging and similarity models

### EVA02 visual similarity

Open **Settings → System → Visual Similarity** to see worker/model/index status. The model can be installed explicitly from the UI and the image library can then be indexed. A remote embedding worker can also be configured if inference should run on another machine/GPU.

### Camie Tagger v2

Camie is optional and is **not bundled or automatically downloaded**. For local inference provide:

```text
camie-tagger-v2.onnx
camie-tagger-v2-metadata.json
```

With the standard Docker cache layout, the local worker expects these under:

```text
/cache/visual-embeddings/camie/
```

The Visual Similarity/Image Tagging UI reports the paths and readiness detected by the running server. Remote inference can be used instead when configured.

## Using StashBooru

The normal Stash workflow still applies: add your media directories, scan the library, then browse, curate, edit, tag, and organize it from the web UI. Stash scrapers, plugins, galleries, jobs, media playback, and most configuration concepts remain relevant.

Because the fork changes terminology and adds native Copyright/series relationships, upstream documentation may still use **Scene**, **Performer**, **Studio**, and **Group** where StashBooru displays **Video**, **Character**, **Artist**, and **Collection**.

## Support and documentation

For **StashBooru-specific bugs and feature work**, use this repository's [Issues](https://github.com/Dusky-dev/StashBooru/issues) and [Pull Requests](https://github.com/Dusky-dev/StashBooru/pulls).

For the underlying Stash server, configuration, scrapers, plugins, and general usage, the upstream resources remain valuable:

- [Stash documentation](https://docs.stashapp.cc/)
- [Stash repository](https://github.com/stashapp/stash)
- [Community scrapers](https://github.com/stashapp/CommunityScrapers)
- [Plugins documentation](https://docs.stashapp.cc/plugins/)

## Development

StashBooru follows the Stash codebase closely enough that the existing developer documentation remains the starting point:

- [Development setup](docs/DEVELOPMENT.md)
- [Architecture](docs/ARCHITECTURE.md)
- [Contributing](docs/CONTRIBUTING.md)

The fork intentionally preserves upstream/internal API names in a number of places for compatibility, so contributors should not mechanically rename every `scene`, `performer`, `studio`, or `group` symbol just because the UI uses different terminology.

## Upstream and license

StashBooru is forked from [stashapp/stash](https://github.com/stashapp/stash) and remains licensed under the **GNU Affero General Public License v3.0**. See [LICENSE](LICENSE).

Thanks to the Stash project and its contributors for the foundation this fork builds on.
