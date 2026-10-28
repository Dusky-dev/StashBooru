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

The follow-up task in **Settings → Tasks → Inspect media → Experimental
image storage estimate** accepts the numeric ID of an *existing* Visual Stack.
It resolves image paths from StashBooru's catalogue on the server, never
from user-supplied filesystem paths. The selected stack must contain 2–16
images with regular primary files; video/mixed stacks are rejected.

Choose **Byte-exact** (recommended) or explicitly select **Pixel-exact**,
acknowledge the warning, and click **Estimate savings (read-only)**. The job
queue tracks completion and errors. **Refresh report** shows the most recent
comparison of P12 and P13 candidates, expected sizes and hypothetical savings.
The task requires the Python estimator and its P12/P13 scripts installed
beside the server (or an explicit `STASH_IMAGE_STORAGE_ESTIMATE_SCRIPT` path).

This first integration handles one Visual Stack at a time; it does **not**
automatically scan or cluster a whole library, publish archive packs,
delete original files, or serve reconstructed image data.

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
