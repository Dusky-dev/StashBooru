# P12 — image delta-storage benchmark and recovery prototype

Implemented 2026-10-04 on `feat/p12-image-delta-prototype-20261004`, from
`develop` at `03f3bd88969f8b04414dca7781ddd1c96bf53c93` (merged P11 PR #135).
This is the first **non-destructive, offline** P12 deliverable. It does not
activate packed media, change the catalogue, replace originals, or alter
scanning/serving/conversion. Production storage integration remains gated.

## P11 checkpoint

PR #135 is merged. Final head `a0273de1d65606426ee1b1846ee0cb3a3cc026ef`
was checked on 2026-10-04: Build (including generation, tests and all platform
builds), Go lint, Media converter and Image restoration checks all passed.
All nine restoration Python tests passed again locally. Current source retains
the reviewed-preview invalidation, exact compositing checks, distinct native
derivative registration and retained source/mask/reference provenance.
Actual diffusion and physical RTX 3060 inference/VRAM/seam testing remain
unverified; this checkpoint does not claim those runtime gates passed.

## Fidelity and storage choices

| Mode | Reconstruction contract | Export |
| --- | --- | --- |
| `byte-exact` (default) | Entire original container bytes; source SHA-256, MD5 and length agree after reconstruction. Works across formats and preserves animations and all embedded metadata. | Original bytes, with a portable ordinal prefix on the original name. |
| `pixel-exact` (explicit) | Versioned stored-raster RGBA8, or little-endian grayscale16, with exact alpha and every pixel value. No orientation bake, profile conversion or premultiplication. | Deterministic-policy PNG plus the full per-member metadata/provenance in `extraction.json`. It is a new container with a new export fingerprint. |

Pixel-exact supports still PNG/JPEG/WebP and grayscale16 PNG. It rejects
multichannel16 PNG, TIFF/other unsupported decoders, CMYK and animation instead
of silently truncating precision or comparing only the first frame. Byte-exact
retains these inputs without decoding their pixels. JPEG decoded pixels are the
already compressed source's values, not recovered pre-JPEG detail.

ICC, Exif/orientation and supported PNG ancillary chunks are retained verbatim
and embedded in PNG exports. Palette/transparency are expanded to exact canonical
RGBA; their original chunks and other ancillary bytes remain in the metadata
sidecar. JPEG APP/COM data, including other embedded application metadata, also
remains verbatim in the sidecar. The exporter embeds known rendering chunks
(`cHRM`, `gAMA`, `iCCP`, `sRGB`, `pHYs`, text, Exif and time); it does not reinterpret
arbitrary application metadata. Archive-wide byte equality is offered only by
byte-exact mode.

Each full member compares raw payload, zlib level 9 and, for pixel mode, a
lossless PNG representation. Identical payloads share one immutable SHA-256
blob even in the independent layout. Every possible base in the bounded stack
is measured; the visual representative is recorded separately. This is a
deterministic greedy per-member choice followed by comparison of complete group
sizes, not a claim of globally optimal compression.

Byte deltas XOR the entire target byte stream against the base, using zero
prediction after the base ends and retaining the target length. Pixel deltas
XOR **every** canonical target byte against an integer nearest-neighbor
prediction. Sampling is explicitly `floor((coordinate - offset) * base_size /
prediction_size)`; pixels outside the prediction canvas start at zero. Defaults
compare identity, centred identity and aspect-preserving fit. Up to four explicit
scale/offset hints per target/base can supply reviewed alignment. No heatmap
threshold removes residuals, alpha, borders or transparent RGB values. There are
no crop/rotation detectors or automatic P03 alignment claims.

Every candidate base reconstructs all its members and compares the full bytes or
planes before selection. Candidate reconstruction time is reported separately
from encode and complete archive-verification time. Size chooses the base with
stable input-order ties; timings inform review and do not make selection random.

Both inclusive minima must pass: default delta improvement is **5% and 0 bytes**
against the independently encoded archive. The candidate must additionally save
net bytes against independent source files (and against independent lossless
PNG files in pixel mode). Otherwise it retains the independent layout. A fallback
prototype archive may be larger than the original ordinary files because of its
manifest/index overhead; it is written only to exercise recovery, never substituted
for those files. This distinction also applies to independent exact-blob sharing.

## Use on actual stacks

Install Pillow and NumPy in the Python environment used for this offline tool.
No model, GPU, application server or database is required. It does not fetch media
or dependencies. Candidate membership should come from a reviewed P08 stack or
P02/P03 comparison; similarity evidence itself is not used as a storage checksum.

Create `stacks.json` with explicit accessible paths. Relative paths resolve beside
that JSON file. The Image IDs are preserved as opaque typed identities; they do
not authorize catalogue changes. A portable `name` can be supplied when a source
basename uses unsupported characters.
The original basename remains separate `sourceName` provenance even when a
portable export name is supplied.

```json
{
  "stacks": [
    {
      "id": "stack:12",
      "representative": "image:42",
      "members": [
        {"id": "image:42", "path": "/media/base.png"},
        {"id": "image:43", "path": "/media/variant.png"}
      ]
    }
  ]
}
```

P08's existing read-only `findVisualStack(id: ID!)` query provides typed members,
representative, position and labels. Read those when preparing this input and
resolve each Image's current primary file. This prototype does not add an API,
automatically enumerate stacks, or process mixed Video members. Split larger
stacks into explicitly bounded candidate groups while keeping their native IDs.

```sh
python3 scripts/image_delta_storage.py benchmark \
  --stacks stacks.json --output benchmark.json

python3 scripts/image_delta_storage.py pack \
  --stacks stacks.json --group stack:12 --fidelity byte-exact \
  --min-bytes 4096 --min-percent 5 --output stack-12.p12

python3 scripts/image_delta_storage.py verify stack-12.p12
python3 scripts/image_delta_storage.py extract stack-12.p12 --output recovered
python3 scripts/image_delta_storage.py extract stack-12.p12 \
  --entry image:43 --output recovered-one
```

For a separate pixel-exact measurement/archive, explicitly use `--fidelity
pixel-exact` with a new output path. Outputs are never overwritten. Benchmark
reports retain errors per group/contract, identify the failing member, and exit
unsuccessfully if any contract failed; successful rows remain reviewable.

## Archive, accounting and recovery

The experimental ZIP_STORED archive contains `manifest.json`, its SHA-256, and
`blobs/<SHA256>`. The manifest is also the per-member index: versioned algorithm,
decoder/export/compressor, IDs/order, display representative, source hashes,
canvas/precision/channels, metadata, encoded/raw lengths, reconstruction hashes,
codec and direct base references. Dependency depth is exactly zero or one; cycles
and delta-as-base references are rejected. A pack carries its own base bytes,
so deleting an external source file cannot break the prototype pack.

`archiveBytes` includes **every unique blob, manifest, checksum, filename, local
header, central index and archive trailer**. Tests compare the estimate with the
actual file size. `originalFileBytes` counts separately stored inputs;
`independentLosslessPNGBytes` counts full standalone canonical exports including
rendering metadata, and `independentLosslessSidecarBytes` counts the necessary
complete metadata/provenance index. Their sum is `independentLosslessFileBytes`.
The net-savings gate uses that complete independent representation, not just
its PNG pixel payloads. Copies contribute their actual separate-file sizes to this
baseline; content-addressed archive blobs count once. Caches and application
backups are separately reported as zero because the tool owns neither.
`retainedOriginalsPlusArchiveBytes` explicitly includes the additional prototype
archive: the current non-destructive run consumes more disk space overall.

Creation writes beside the destination, fsyncs a complete staging archive, verifies
it through the standalone reader, then atomically hard-links it to a **new**
destination and fsyncs the directory. The filesystem must support hard links;
there is no unsafe copy/overwrite fallback. Ctrl-C and caught failures clean up
staging. A killed process may leave `.p12-*.staging`: incomplete archives are
rejected; complete archives can be independently verified and recovered:

```sh
python3 scripts/image_delta_storage.py verify /path/.p12-example.staging
python3 scripts/image_delta_storage.py recover /path/.p12-example.staging \
  --output recovered-stack.p12
```

Recovery leaves its supplied input intact. A crash after publication leaves a
valid final archive; an extra staging hard link can be removed separately.
There is no catalogue activation step or only-source temporary file. Stage files
remain bounded by the single pack limit; repeated external kills can leave
multiple files, which are not an automatically managed cache.

Single-member extraction reads only its payload and at most one independent
base, not every earlier member. Full verification reports affected members and
their base dependency; missing/corrupt deltas do not block verified independent
members. An integrity failure never silently returns incomplete pixels. The
reader rejects traversal/duplicate names, unknown codecs/versions, chains,
oversized canvases/metadata, nonregular input, excessive ZIP directories, ZIP
compression/encryption/ZIP64, truncated/extra zlib data and decompression bombs.

Extraction exclusively creates a new directory. Each member is reconstructed,
checked and fsynced; `extraction.json` is its durable completion marker. Caught
failures clean up that new directory. A hard-killed extraction may leave a
partial directory without the marker: rerun from the intact archive into a new
directory. Existing directories/files are never overwritten. Byte-mode exports
have source fingerprints; pixel-mode export fingerprints are computed from the
new PNG bytes and historical source fingerprints remain separate provenance.

Default hard bounds: 16 members per stack, 32 stacks per benchmark, 64 MiB per
file/plane, 256 MiB source-plus-plane input per stack, 384 MiB per archive,
16 megapixels / 16384 pixels per side, 1 MiB metadata per member and 15 minutes
per operation. Encode working memory includes additional bounded predictors,
residuals and candidate payloads; the 256 MiB input bound is not a process-RSS
quota. Cancellation/deadline checks are cooperative between bounded codec work.

## Measured labelled fixtures

The checked [JSON report](benchmarks/p12-image-delta-20261004.json) records runtime
versions, exact bytes, every base choice, timing and successful reconstruction.
This corpus uses resized repository screenshot artwork plus generated variants,
seeded unrelated noise and full-range grayscale16. It is **not a user's real stack
corpus** and its savings do not predict general-library performance.

| Fixture | Original files | Byte-exact pack | Pixel-exact pack | Pixel delta selected |
| --- | ---: | ---: | ---: | --- |
| exact-copies | 644,331 B | 216,937 B | 251,814 B | No |
| localized-edits | 644,143 B | 645,764 B | 255,007 B | Yes |
| translation-borders | 643,861 B | 645,180 B | 250,713 B | Yes |
| different-resolutions | 607,419 B | 609,205 B | 725,037 B | No |
| recompressed-jpegs | 187,253 B | 173,176 B | 877,978 B | No |
| cross-format-same-pixels | 285,156 B | 281,092 B | 295,351 B | No |
| unrelated-noise | 1,928,603 B | 1,930,905 B | 1,928,048 B | No |
| alpha-hidden-rgb | 643,558 B | 572,578 B | 253,619 B | Yes |
| high-bit-depth | 241,348 B | 123,243 B | 123,496 B | Yes |
| metadata-orientation | 429,663 B | 430,964 B | 250,576 B | No |

Localized edits and known borders produced useful pixel-mode deltas. JPEG
recompression and Lanczos resizing retained independent storage. One decoded
JPEG/PNG pair had identical canonical pixels but its proposed pixel archive was
larger than the source files, so net-savings policy rejected that delta. Exact
copies save through ordinary blob sharing even when the delta improvement is
below the 5% threshold. These measured fallback cases are intentional results.

Reproduce without catalogue access or retained fixture files:

```sh
make test-image-delta
python3 scripts/tests/image_delta_fixtures.py --output p12-benchmark.json
```

## Validation and production gate

On 2026-10-04, all **22 Python contract/recovery tests** and **20 measured
group/contract fixture runs** passed. Tests cover unchanged source bytes, full
offline restore after removing the originals, exact low-level/hidden-RGB edits,
alpha, integer resize/offset/borders, uint16 grayscale, ICC/Exif/text/orientation,
hash/provenance distinction, actual overhead, independent/threshold fallbacks,
base choice, single-member access, corrupt/missing dependants, hostile inputs,
ordinary cancellation and actual subprocess exits at creation, entry writes,
durability, verification and publication. A killed extraction restarts from its
archive. The dedicated CI workflow publishes its measured JSON as an artifact.

This package changes Python scripts, its Makefile target, CI and documentation.
There is no native schema migration, GraphQL/UI change, locked dependency
upgrade or application processing worker protocol change. The local container
has no Go executable; application Go/UI gates were not rerun for this standalone
prototype. P11's verified final CI gates are recorded separately above.

Before any original eviction, the remaining gate requires actual owner-stack
measurements (including JXL/native codec cases), supported-version/platform
conformance, a shared logical-media read/materialization interface, protected
live-base references/GC, journaled catalogue activation and crash/restart tests
at each application transition, bounded concurrent reconstruction cache,
serving/scan/conversion/restore/backup integration and user-visible measured
savings. A database backup alone cannot restore these packs; a portable backup
must include the complete archive. Linux/Pillow behavior was verified here;
Windows/macOS runtime and cross-version decoding have not been tested.

## Existing file consumers to integrate before activation

| Consumer | Current entry points / constraint |
| --- | --- |
| Scan and animation/format probe | `internal/manager/task_scan.go`, `pkg/file/image/scan.go`, `pkg/image/scan.go`; native paths and fingerprints currently describe ordinary files. |
| Serving and download | `internal/api/routes_image.go` → primary `BaseFile.Serve` with `file.OsFS`; serve validated materializations before changing that contract. |
| Thumbnails and pHash | `internal/manager/task_generate_image_thumbnail.go`, `task_generate_image_phash.go`; processing currently reads the primary path. |
| Conversion, upscale and restoration | `internal/manager/media_conversion.go`, `internal/api/routes_media_conversion*.go`, `routes_image_upscale*.go`, `routes_image_restoration.go`; materialize inputs and retain source/active fingerprint distinction and native restore journals. |
| Embedding/inference and remote workers | `internal/api/routes_image_visual_similarity.go`, worker upload/transfer paths; workers cannot assume access to private packed-storage paths. |
| Metadata export and Gallery archives | `internal/manager/task_export.go`, `pkg/image/export.go`, `internal/api/routes_gallery.go`; retain IDs/order/names and export actual ordinary media bytes independently. |
| Delete/cleanup and external tools | `pkg/image/delete.go`, `pkg/file/clean.go`, `internal/manager/task_clean.go`; trace live pack dependencies before GC and offer checked portable files to tools. |
| Native backup/restore | `internal/api/resolver_mutation_metadata.go`; version manifests and referenced bytes are mandatory additions before claiming packed-media backup coverage. |

Next roadmap prototype: P13 bounded gallery chunk/tile sharing, using the
same honest byte-versus-pixel measurements and independent recovery requirements.
Production P12 activation is a separate milestone; P13 is not implemented here.

Published as [PR #136](https://github.com/Dusky-dev/StashBooru/pull/136).
Implementation `e85e1e6e44746e59afe4e95514255dead5ee9ce3` has Git tree
`0af2dec7e3e6c582a171e354cb0a69f510b8656f`, exactly matching locally verified
`89f477a7c2bb643753eec94780c00780cf85ca64`, with merged P11 ancestry retained.
The initial [P12 CI workflow](https://github.com/Dusky-dev/StashBooru/actions/runs/37219023641)
passed its 22 tests and 20 fixture contracts and published its benchmark artifact.
Final-head application CI is tracked on the PR. These subsequent publication
notes do not change the checked implementation.
