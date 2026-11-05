# Tasks

This page allows you to direct the stash server to perform a variety of tasks.

## Scanning

The scan function walks through the stash directories you have configured for new and moved files. 

Stash currently identifies files by performing a quick file hash. This means that if the file is renamed for moved elsewhere within your configured stash directories, then the scan will detect this and update its database accordingly.

Stash currently ignores duplicate files. If two files contain identical content, only the first one it comes across is used.

### Ignoring files with `.stashignore`

You can create `.stashignore` files to exclude specific files or directories from being scanned. These files use gitignore-style pattern matching syntax.

Place a `.stashignore` file in any directory within your library. The patterns in that file will apply to all files and subdirectories within that directory. You can have multiple `.stashignore` files at different levels of your directory hierarchy - patterns from parent directories cascade down to child directories.

`.stashignore` files are not read inside zip files.

**Supported patterns:**

| Pattern | Description |
|---------|-------------|
| `filename.mp4` | Ignore a specific file. |
| `*.tmp` | Ignore all files with a specific extension. |
| `temp/` | Ignore a directory and all its contents. |
| `**/cache/` | Ignore directories named "cache" at any level. |
| `!important.mp4` | Negation - do not ignore this file even if it matches a previous pattern. |
| `# comment` | Lines starting with # are comments. |
| `\#filename` | Use backslash to match a literal # character. |

**Example `.stashignore` file:**

```
# Ignore temporary files
*.tmp
*.log

# Ignore specific directories
temp/
.thumbnails/

# But keep this specific file
!important.tmp
```

The scan task accepts the following options:

| Option | Description |
|--------|-------------|
| Generate scene covers | Generates scene covers for video files. |
| Generate previews | Generates video previews (mp4) which play when hovering over a scene. |
| Generate animated image previews | *Accessible in Advanced mode* - Also generate animated (webp) previews, only required when Scene/Marker Wall Preview type is set to Animated image. When browsing they use less CPU than the video previews, but are generated in addition to them and are larger files.|
| Generate scrubber sprites | The set of images displayed below the video player for easy navigation. |
| Generate video perceptual hashes | Generates perceptual hashes for scene deduplication and identification. |
| Generate thumbnails for images | Generates thumbnails for image files. | 
| Generate image perceptual hashes | Generates perceptual hashes for image deduplication and identification. |
| Generate previews for image clips | Generates a gif/looping video as thumbnail for image clips/gifs. |
| Rescan files | By default, Stash will only rescan existing files if the file's modified date has been updated since its previous scan. Stash will rescan files in the path when this option is enabled, regardless of the file modification time. Only required if Stash needs to recalculate video/image metadata, or to rescan gallery zips. |

## Merge duplicate Characters

**Settings → Tasks → Review profiles & associations → Merge duplicate Characters**

This tool proposes possible duplicates among native Character profiles. It compares normalized names (including reversed first/last names and unambiguous shortened names) and requires the **same nonempty, directly assigned Copyrights**. Characters without Copyrights, variants, ambiguous names, or conflicting profile details are skipped. Matching names alone are not proof of identity.

1. Click **Preview Character merges** to inspect proposed survivors, source profiles and their Copyrights. This preview changes nothing.
2. Select individual groups, or **Select all**. Nothing is preselected.
3. Choose **Merge selected groups** to queue only those groups, or **Mark selected as not duplicates** to prevent those matches from being proposed again. There is also a per-row **Not duplicate** action.
4. Expand **Not duplicates** to review saved rejected pairs. **Suggest again** removes an exclusion if you change your mind.

Not-duplicate decisions are stored in the StashBooru database and survive rescans and restarts. They apply to the exact Character IDs, not globally to similar names; a rejected pair also blocks its larger suggested group, preventing an indirect merge through a third profile. If either Character is deleted, the decision is discarded. Back up your database if you need to preserve these decisions through restoration.

Merging combines compatible metadata and Image, Video and Gallery links into a surviving Character; the removed Character profile records are deleted, **not the media files**. Merges are not undoable through this task. A merge job validates the preview again and runs transactionally: cancellation or failure before commit rolls back its changes. Make a database backup first, and review each group before merging.

## Review library inheritance

**Settings → Tasks → Review profiles & associations → Review library inheritance**

Use this task to preview how the library's calculated associations would change if you include or exclude ancestor Characters, Artists, Copyrights, or Tags. These four switches control inherited associations; they **do not rewrite directly assigned Image or Video links**. The same hierarchy defaults are shared by System and Tagging settings.

1. Adjust the four inheritance switches and click **Preview library**. The preview runs as a background task; no settings change until you apply.
2. Review the counts for Images, Videos, affected media and added/removed inherited associations. Expand individual examples to see current and proposed memberships and their sources. The examples are a sample; the review covers the whole library.
3. Choose **Apply reviewed defaults** only after checking the report. A review with errors cannot be applied. You can cancel a running job or discard a finished review to adjust the switches and preview again.

The review can resume in the same browser session. While an existing review is active, its proposed settings are locked until you apply or discard it. If profiles or associations change in the meantime, preview again rather than relying on stale results.

## Animation inspection

**Settings → Tasks → Inspect media → Animation inspection**

Regular scans inspect frame counts and add the native **animated** Tag to animated Images without replacing other metadata. Click **Inspect existing images** to backfill Images imported before animation detection was available. This queues a background task; use the Task Queue to follow its progress. It can add missing Tags but does not replace original media files.

## Video overlap / containment

**Settings → Tasks → Inspect media → Video overlap / containment**

Use **Open video overlap review** to find Videos containing matching segments, including trimmed or re-encoded copies and clips within compilations. The review indexes timestamped video samples, then presents candidate matches and their intervals for manual comparison.

- Choose an indexing backend and sample interval, then index the library or the selected reference Video. Index checkpoints can resume after a cancellation or restart.
- Search for overlapping segments and examine the match evidence, coverage and unmatched intervals. Use **Compare matched segments** for synchronized playback before drawing conclusions.
- A similarity result is evidence to review, **not proof that two files are interchangeable**. Video records and source files remain in the native catalogue; this review does not automatically delete duplicates.

The comparison and indexing settings are available inside the review dialog. Some codec or worker configurations may require additional setup.

## Auto tagging

See the [Auto tagging](/help/AutoTagging.md) page.

## Scene filename parser

See the [Scene filename parser](/help/SceneFilenameParser.md) page.

## Generated content

The generated content provides the following:

* Scene covers - screenshot of the scene used as the cover image
* Video or image previews that are played when mousing over the scene card
* Video Perceptual hashes - helps match against StashDB, and feeds the duplicate finder
* Sprites (scene stills for parts of each scene) that are shown in the scene scrubber
* Marker video previews that are shown in the markers page
* Transcoded versions of scenes. See below
* Image thumbnails of galleries
* Image Perceptual hashes - can be used for identification and deduplication

The generate task accepts the following options:

| Option | Description |
|--------|-------------|
| Scene covers | Generates scene covers for video files. |
| Previews | Generates video previews (mp4) which play when hovering over a scene. |
| Animated image previews | *Accessible in Advanced mode* - Generates animated previews (webp). Only required if the Preview type is set to Animated image. Requires Generate previews to be enabled. |
| Scene scrubber sprites | The set of images displayed below the video player for easy navigation. |
| Marker previews | Generates video previews (mp4) which begin at the marker timecode. Markers with an explicit end time run for that interval (optionally capped by the `Max marker preview duration` setting); markers without one use the `Default marker preview duration` setting (20 seconds by default). Both are configurable under `Settings → System → Preview Generation`. |
| Marker animated image previews | *Accessible in Advanced mode* - Also generate animated (webp) previews, only required when Scene/Marker Wall Preview type is set to Animated image. When browsing they use less CPU than the video previews, but are generated in addition to them and are larger files. |
| Marker screenshots | Generates static JPG images for markers. Only required if Preview type is set to Static image. Requires marker previews to be enabled. | 
| Transcodes | *Accessible in Advanced mode* - MP4 conversions of unsupported video formats. Allows direct streaming instead of live transcoding. |
| Video perceptual hashes | Generates perceptual hashes for scene deduplication and identification. |
| Generate heatmaps and speeds for interactive scenes | Generates heatmaps and speeds for interactive scenes. |
| Image clip previews | Generates a gif/looping video as thumbnail for image clips/gifs. |
| Image thumbnails | Generates thumbnails for image files. |
| Image perceptual hashes | Generates perceptual hashes for image deduplication and identification. |
| Overwrite existing files | By default, where a generated file exists, it is not regenerated. When this flag is enabled, then the generated files are regenerated. |

### Transcodes

Web browsers support a limited number of video and audio codecs and containers. Stash will directly stream video files where the browser supports the codecs and container. Originally, stash did not support viewing scene videos where the browser did not support the codecs/container, and generating transcodes was a way of viewing these files.

Stash has since implemented live transcoding, so transcodes are essentially unnecessary now. Further, transcodes use up a significant amount of disk space and are not guaranteed to be lossless.

### Image gallery thumbnails

These are generated when the gallery is first viewed, so generating them beforehand is not necessary.

## Cleaning

This task will walk through your configured media directories and remove any scene from the database that can no longer be found. It will also remove generated files for scenes that subsequently no longer exist.

Care should be taken with this task, especially where the configured media directories may be inaccessible due to network issues.

## Exporting and importing

The import and export tasks read and write JSON files to the configured metadata directory. Import from file will merge your database with a file.

> **⚠️ Note:** The full import task wipes the current database completely before importing.

See the [JSON Specification](/help/JSONSpec.md) page for details on the exported JSON format.

## Backing up

> **⚠️ Compatibility:** StashBooru database backups are incompatible with vanilla Stash. Restore them using a compatible StashBooru version. Keep a separate vanilla Stash backup if you need to return to vanilla Stash.

The backup task creates a backup of the StashBooru database and (optionally) blob files. The backup can either be downloaded or output into the backup directory (under `Settings > Paths`) or the database directory if the backup directory is not configured.

When including filesystem blob storage, the backup is stored as a zip file, with the database file at the root of the zip and the blob files in a `blobs` directory. Blobs are database-managed assets, such as profile images; they are not your library's media files. If blobs are stored in the database, they are already included in the database copy.

> **⚠️ Backup scope:** Neither backup option includes your media files, conversion/upscaling original-file restore caches, generated files, or configuration. Back up media, restore caches, and configuration separately. Generated files can be backed up or regenerated after restoring. A database backup alone cannot recover missing media or cached originals.

For an **experimental, manual original-media backup** before storage optimization, StashBooru also ships `scripts/media_library_backup.py`. It creates SHA-256-verified, reusable file snapshots and restores to a separate directory. This is a command-line safety tool, **not** a setting in the running app or a substitute for backing up the database, configuration, and conversion/upscaling caches. See the repository's [media library backup guide](https://github.com/Dusky-dev/StashBooru/blob/develop/docs/media-library-backup.md).

For database-only backups, only the database file is copied into the destination. This is useful for quick backups before performing risky operations, or for users who do not use filesystem blob storage.

## Restoring from backup

> **⚠️ Compatibility:** Restore StashBooru database backups only with a compatible StashBooru version, never vanilla Stash. A matching schema number alone does not establish compatibility between the two applications.

Restoring from backup is currently a manual process. Before restoring, create a backup of the current database and blob files using the backup task, and keep it separate from the backup being restored. Stop StashBooru before replacing any database or blob files. The database backup zip file must be unzipped, and the database file and blob files (if applicable) copied into the database and blob directories respectively. For database-only backups, copy the database file directly. StashBooru should then be restarted to load the restored database. Restore media, configuration, and original-file restore caches from their separate backups as needed.

> **⚠️ Note:** the filename for a database-only backup is not the same as the original database file, so the database file from the backup must be renamed to match the original database filename before copying it into the database directory. The original database filename can be found in `Settings > Paths > Database path`.
