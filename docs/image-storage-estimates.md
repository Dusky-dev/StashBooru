# Image storage savings comparison (read-only)

This experimental command reports **estimated** physical sizes for selected
Visual Stacks. It reuses the existing P12 base/delta and P13 chunk/tile planning
and full in-memory reconstruction checks. It does **not** write pack archives,
modify original images, activate compressed media, modify StashBooru's database
or remove files.

## Usage

Install Python 3, Pillow and NumPy, then prepare a small explicit input file
on the StashBooru server. Paths must refer to existing original files. Groups
are limited by the existing P12/P13 upper bound (1–32 groups of 1–16 members).
Group members represent *image* files in the same Visual Stack; do not infer
duplicates from the pHash or EVA02 index alone.

```json
{
  "stacks": [
    {
      "id": "stack:17",
      "representative": "image:201",
      "members": [
        {"id": "image:201", "path": "/srv/media/original.png"},
        {"id": "image:202", "path": "/srv/media/variant.png"}
      ]
    }
  ]
}
```

For safe exact-original-file recovery, use the default **byte-exact** mode:

```sh
python3 scripts/image_storage_estimate.py --stacks /path/stacks.json \
  --min-bytes 4096 --min-percent 5 \
  --output /path/new-storage-estimate.json
```

The report evaluates P12 delta and P13 shared blocks on equivalent groups,
including full container metadata and archive overhead. Only candidates
meeting their own savings gates **and** smaller than the separate originals
can be recommended. It displays source bytes, candidate packed bytes,
potential savings, and storage required **while originals are still retained**.

`--fidelity pixel-exact` is available for experimentation, but produces a
different PNG/container representation and **cannot** reconstruct original
JPEG/WebP/PNG container bytes exactly. It must not be used as a default for
production original-file eviction.

Reports can be printed without `--output`. With an output path, it creates a
**new** JSON report and never overwrites an existing file. No pack is
published during estimation. Individual group failures are reported explicitly;
the command returns nonzero if any group fails. It requires Pillow/NumPy but
does not download them automatically.

## Native Settings task (experimental)

The default **Analyze sample automatically** action needs no IDs. It
takes up to four small eligible image groups from the existing image-only
Visual Stacks (2–16 files), then fills any remaining slots using existing
primary-file pHash clusters with Hamming distance ≤4. It skips unavailable,
symlinked and oversized sources, existing stack members in auto pHash samples,
and groups that exceed 32 MiB/file or 96 MiB/group. No hashing, media scan,
image grouping, file update, model download, or new persistent Visual Stack
is performed by this action. If no eligible groups exist, the task explains
how to create a Visual Stack or generate pHashes.

This is a **bounded convenience sample**, not a representative or full-library
estimate; the reported hypothetical savings apply only to files actually
sampled, and should never be extrapolated across the entire library.
pHash proximity is a candidate-selection hint, **not** proof of duplicate
files or guaranteed storage savings. Each P12/P13 proposal is still checked
using actual original bytes and strict reconstruction.

For a specific set, enable **Advanced: enter specific Visual Stack IDs** and
provide 1–8 distinct IDs separated by commas or spaces. The server resolves
current paths from its own catalogue; the browser cannot supply file paths.
Only image-only 2–16-member stacks are allowed in this manual mode.

Leave **Byte-exact** (recommended) selected, then choose automatic or manual
estimate. **Pixel-exact** remains an explicit experimental option and may
change container bytes. The job queue tracks completion and errors; **Refresh
report** shows the latest candidate comparison and sample scope.
The task requires the Python estimator and its P12/P13 scripts installed
beside the server (or an explicit `STASH_IMAGE_STORAGE_ESTIMATE_SCRIPT` path).

The batch extension evaluates up to eight existing Visual Stacks at a time
and reports per-stack recommendations plus hypothetical total savings and the
increased footprint if candidate packs were retained alongside originals.
It does **not** automatically scan or create Visual Stacks for the whole library, publish
archive packs, delete original files, or serve reconstructed image data.

## What this does not establish

The figures are **hypothetical pack sizes**, not real library savings. Until
StashBooru has tested native reconstruction, scanning, serving, backups and
recoverability, retaining full originals alongside new pack files **increases
actual disk usage**. No safety warning is lifted and no destructive switch is
enabled by this step. Run against real, representative groups before deciding
whether any P12/P13 format should become an application storage backend.

Next step: a native **Estimate savings** task that creates verified input
specifications from current Visual Stacks and reports progress/results without
making filesystem mutations. Only after independent restoration, backups and
all media consumers are integrated should any opt-in optimizer be considered.

## Docker installation (no manual Python script copying)

The StashBooru `develop` Docker image built with this change packages the
repository's Python storage scripts at `/scripts`, including the estimator,
P12/P13 modules, media-backup tools and candidate verification/recovery helpers.
The container already installs Python 3, NumPy and Pillow. Its
`STASH_IMAGE_STORAGE_ESTIMATE_SCRIPT` points at the packaged estimator.

After a successfully published updated Docker image, pull it and recreate the
StashBooru container using the same configuration and volume mounts. The
**Experimental image storage estimate** warning about a missing script should
disappear. An existing image is not changed by a git pull alone, and this does
not activate compression or touch any originals.

**Standalone executable releases** do not include these Python scripts
inside the `stash-linux` binary. For a standalone server, keep the matching
`scripts/` directory from the source tree beside the running binary's
working directory (or set `STASH_IMAGE_STORAGE_ESTIMATE_SCRIPT` to the absolute
path of `image_storage_estimate.py`); Python 3 with Pillow/NumPy must also
be available. An image-only Visual Stack of 2–16 files is still required
to run an estimate.
