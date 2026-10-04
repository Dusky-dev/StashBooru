# P13 bounded Gallery block sharing

P13 implements the roadmap's first measured, non-destructive Gallery storage
prototype. It compares original byte chunks and exact decoded pixel tiles,
counts the complete archive, verifies every candidate, and exports ordinary
files without an application database. Original files remain intact. There is
no native file activation, catalogue write, schema migration, model download,
global deduplication or application worker change.

P12 PR [#136](https://github.com/Dusky-dev/StashBooru/pull/136) supplies the
strict pixel/metadata contract, portable PNG exporter and bounded recovery
helpers. Its final head `7023c9f812f1ecb9184193b3480a5c8790f61357` passed all
GitHub checks on 2026-10-04. P13 starts from that exact commit; P12 was still
open when work started. The P13 review therefore targets the P12 feature branch.
After P12 merges, retarget P13 to `develop` before its separate merge.

## Fidelity and identifiers

| Mode | Verified reconstruction | Portable export |
| --- | --- | --- |
| `byte-exact` (default) | Original complete file bytes, length and SHA-256; recorded historical MD5 is retained. | Original file/container bytes. Animation and unsupported pixel codecs can be archived without decoding. |
| `pixel-exact` (explicit opt-in) | Stored-raster RGBA8 or uint16 grayscale, including straight alpha, hidden transparent RGB and one-level changes, plus required rendering metadata. | Deterministic-policy PNG plus the metadata/provenance sidecar. This does not reproduce the original compressed file bytes. |

P12's still-image PNG/JPEG/WebP decoder is reused. Pixel mode rejects animation,
unsupported precision/modes and 16-bit multichannel PNG rather than silently
truncating. It does not rotate for Exif, transform ICC colors, threshold
differences or discard borders. Rendering chunks are embedded where supported;
opaque ancillary/JPEG APP/COM bytes remain in `extraction.json`. Metadata is
per image, so sharing pixel bytes does not replace that image's color/profile,
orientation or provenance. Source and exported SHA-256/MD5 remain distinct.

Gallery/member IDs are opaque identifiers. Input array order is reading order;
the optional representative selects display provenance only. The original
basename is recorded as `sourceName`; a portable `name` alias can be supplied.
Exports prefix the alias with its zero-based ordinal, preserving order and
avoiding collisions. Pixel exports change the suffix to `.png`. Native IDs,
relationships, primary file paths and fingerprints are never changed by this
offline tool.

## Estimate, review and pack

Dependencies are Python 3, Pillow and NumPy, explicitly installed by the operator
in a dedicated environment. The tool never downloads dependencies or models.
The repository CI installs the Ubuntu image/array packages explicitly.

Create a source specification:

```json
{
  "galleries": [
    {
      "id": "gallery:17/group:0",
      "representative": "image:203",
      "members": [
        {"id": "image:201", "path": "pages/01.png"},
        {"id": "image:203", "path": "pages/02.jpg", "name": "02.jpg"}
      ]
    }
  ]
}
```

Relative paths resolve beside the specification, independently of the shell's
working directory. Split larger Galleries into explicitly ordered groups of
at most 16 members; this prototype does not automatically partition or join
the groups into native Gallery membership.

```sh
python3 scripts/gallery_block_storage.py estimate \
  --galleries galleries.json --gallery gallery:17/group:0 \
  --min-bytes 4096 --min-percent 5 --output review.json

# Choose pixel-exact explicitly when its changed container contract is wanted.
python3 scripts/gallery_block_storage.py estimate \
  --galleries galleries.json --gallery gallery:17/group:0 \
  --fidelity pixel-exact --output pixel-review.json

python3 scripts/gallery_block_storage.py pack \
  --galleries galleries.json --gallery gallery:17/group:0 \
  --min-bytes 4096 --min-percent 5 --output gallery-17-r1.p13

python3 scripts/gallery_block_storage.py benchmark \
  --galleries galleries.json --output measured-galleries.json
```

`estimate` reconstructs and verifies candidates in memory and reports their
complete predicted ZIP sizes. `pack` writes, fsyncs and reopens the selected
archive, independently verifies it and asserts the measured file size equals
the estimate before publication. All outputs are exclusive: select a new path
for a changed report or archive. A losing shared layout writes an independent
recovery archive and recommends keeping the ordinary sources; it does not
activate a less efficient packed source.

## Strategies, savings and physical accounting

| Candidate | Boundaries | Shared unit |
| --- | --- | --- |
| Independent competitor | One full representation per image; smallest of raw/zlib or lossless PNG where applicable. | Identical complete encoded representations also share once. |
| Fixed byte chunks | 4, 16 and 64 KiB. | Exactly equal original byte slices. |
| Content-defined byte chunks | Gear-64 v1, target 8, 32 and 64 KiB; minimum target/4, maximum target×4; state resets per boundary. | Equal original byte slices despite many insertion-induced offsets. |
| Pixel tiles | 32×32, 64×64 and 128×128; row-major grid and exact smaller edge tiles. | Exactly equal canonical tile bytes. No approximate or base/residual tiles are used. |

Each chunk chooses raw or zlib level 9, whichever is smaller. Gear constants
derive from the first eight SHA-256 bytes of `p13-gear-v1:<0..255>`, interpreted
little-endian. The recurrence, mask, reset and bounds are fixed in the versioned
format and checked against an independent arithmetic oracle.

Sharing is selected only if the complete archive beats the independent archive
by a positive amount meeting **both inclusive configured minima**, also beats
the original source total by both minima, and is smaller than complete ordinary
exports plus the pixel metadata receipt where required. Defaults are 0 bytes
and 5 percent. Every block size is reconstructed and timed. Unusable candidates
record a bounded error; an independent representation remains available.

Reports include unique encoded and decoded chunk bytes, all per-image manifests,
the Gallery/chunk index, SHA file, ZIP local/central headers and footer. ZIP is
stored without extra fields, comments, ZIP64 or alignment padding, so reported
padding is zero. Dimensions, profiles and metadata count inside the manifests.
`randomAccessInitializationBytes` counts the central directory, footer and all
manifest reads including their local headers; selected-member chunk reads and
encoded bytes appear separately. This reader eagerly validates the bounded
manifest set, then reads/decodes only the requested member's chunks. Repeated
references within that member read each encoded chunk once.

`archiveBytes` counts physical payloads once. `allocatedBytes` is explicitly an
**allocation**, assigning each exclusive member manifest to its image and equal
shares of common metadata and each chunk to referencing images. Allocations sum
to the archive but are not additional physical storage. Reports separately list
retained source bytes, sources-plus-prototype bytes, zero persistent cache and
zero additional backup copies. An incremental revision also reports the retained
previous archive and its combined source/revision total. These retention totals
use recorded source sizes; externally deleted sources are not rediscovered by
the offline recovery reader.

## Immutable format and ownership

Each self-contained `.p13` is a bounded ZIP_STORED archive:

| Path | Contents |
| --- | --- |
| `gallery.json` | Product/version, typed Gallery ID, order/representative, fidelity and algorithm, runtime/codec policy, revision provenance, member manifest references and unique chunk descriptors. |
| `gallery.sha256` | SHA-256 of the exact Gallery index bytes. |
| `members/<SHA256>.json` | Independent per-image manifest: ID, order, filenames, source and representation hashes/lengths, dimensions/precision/alpha, metadata checksum and direct ordered chunk references. |
| `chunks/<SHA256>` | Immutable encoded chunk payload. |

Member addresses hash their JSON bytes. A chunk address hashes
`P13-chunk-v1\0` + canonical descriptor JSON + `\0` + encoded payload. The
descriptor includes codec, encoded/decoded lengths and decoded SHA-256. This
distinguishes raw bytes that happen to equal another chunk's zlib stream;
both encoded and decoded integrity are verified. There are no cross-image
dependencies, cyclic references or previous-page decode requirements.

Every revision owns all referenced bytes. Marking the new revision's member
references drops unreferenced chunks from that new archive only. Old archives,
sources and in-progress staging remain separate; this tool never sweeps or
deletes them. Within-Gallery sharing is the only ownership scope. Global sharing,
catalogue reference counts, live-reader/job/restore tracing and production GC
remain future work.

## Independent access, diagnostics and repair

```sh
python3 scripts/gallery_block_storage.py verify gallery-17-r1.p13
python3 scripts/gallery_block_storage.py verify gallery-17-r1.p13 --entry image:203
python3 scripts/gallery_block_storage.py extract gallery-17-r1.p13 \
  --entry image:203 --output recovered-one
python3 scripts/gallery_block_storage.py extract gallery-17-r1.p13 --output recovered-all
python3 scripts/gallery_block_storage.py diagnose gallery-17-r1.p13
```

Extraction uses only pack bytes and its embedded manifests. It writes ordinary
files and a final, fsynced `extraction.json` completion marker containing IDs,
order, original names, source/export fingerprints and metadata. A backup must
include the entire pack; a database, manifest or chunk index alone is insufficient.
Tests restore a copied archive in an empty directory after removing source paths.

`diagnose` checks each chunk and names all members that reference missing or
corrupt bytes, then independently verifies members. Unaffected members remain
extractable even if another image's exclusive chunk is damaged. Structural or
manifest damage rejects the pack rather than trusting an unverified index.

For repair, list **explicit available original sources** in a specification and
pass that Gallery/group. They must match the recorded original length, SHA-256
and MD5. The tool regenerates exactly the recorded chunk sequence, repairs only
covered missing/corrupt chunks, copies other payloads, and verifies a new archive.
It leaves the damaged input intact. Uncovered chunks, mismatched sources or a
runtime unable to reproduce encoded bytes reject repair. Metadata cannot invent
lost data; there is no automatic search/download or silent approximate repair.

```sh
python3 scripts/gallery_block_storage.py repair damaged.p13 \
  --galleries available-originals.json --gallery gallery:17/group:0 \
  --output repaired.p13
```

## Incremental changes and interrupted operations

```sh
# The chosen input group contains only new members, appended in its array order.
python3 scripts/gallery_block_storage.py append gallery-17-r1.p13 \
  --galleries new-pages.json --gallery new-pages --output gallery-17-r2.p13
python3 scripts/gallery_block_storage.py remove gallery-17-r2.p13 \
  --entry image:201 --output gallery-17-r3.p13
```

Append prepares only new sources and copies existing compressed chunks unchanged.
Remove traces surviving direct references, preserves shared chunks needed by
survivors and compacts their order. Removing the representative selects the first
survivor. Empty or oversized revisions and duplicate/reused IDs reject. Neither
operation needs old source paths or rebuilds any unrelated Gallery. Both verify
old and new archives, retain the old revision, and write a generation with the
parent index checksum. ZIP revision publication rewrites that one bounded pack;
it is not an in-place ZIP edit or global store update.

The established layout stays fixed during a revision. Its report rechecks net
source/export savings; it does not re-search all layouts or claim a fresh
independent-archive optimum without processing the existing images. A revision
that misses its gate recommends independent sources. Re-run `estimate` on the
whole source specification when a new optimum is needed. This prototype never
activates either layout in the application.

Writers use private sibling `.p13-*.staging` files, fsync archive bytes and their
directory, reopen/verify the pack, then publish using an atomic no-overwrite hard
link and fsync its directory. The filesystem must support hard links. Ordinary
cancel/error paths clean staging; a process death can leave it for recovery.

```sh
python3 scripts/gallery_block_storage.py recover .p13-valid.staging \
  --output recovered-revision.p13
```

Recovery fully verifies a regular staging file and refuses partial/corrupt output.
The staging input remains available. Hard process-exit tests cover creation,
entry writes, durable close, verification and publication, plus revision writes.
Cancelled extraction removes its newly created output directory. A hard-killed
extraction has no completion marker and can restart into a new directory from
the intact archive; no application reference ever points at its partial output.

## Bounds and measured fixtures

Limits are 32 groups per input, 16 images per pack, 64 MiB per source/decoded
plane, 256 MiB prepared sources-plus-planes, 384 MiB per pack, 16 megapixels and
16384 pixels per side, 1 MiB rendering metadata per image, 8192 unique chunks
and total references, 2 MiB member manifests, 4 MiB Gallery index and 15 minutes
per operation. ZIP directory count/size is bounded before `zipfile` allocation;
paths, duplicate entries, codecs, checksums, grid coverage, manifests and exact
decoded lengths are validated. Zlib rejects oversized, unfinished or trailing
streams. FIFO inputs reject without blocking. Working memory includes prepared
planes, bounded candidate payloads and selected-member data; these limits are
not a process-RSS quota. Cancellation/deadlines are cooperative around bounded
codec work. No persistent or concurrent reconstruction cache is implemented.

The [checked JSON report](benchmarks/p13-gallery-block-20261004.json) contains
26 verified Gallery/fidelity runs, all compared sizes, timing, random-access
accounting and Python/Pillow/NumPy/zlib versions. Fixtures reuse P12's repository
artwork derivatives, seeded noise, alpha/uint16 and metadata cases, and add
synthetic line pages, uncompressed PNG prefix insertion and mixed canvases.
These are **labelled fixtures, not real owner Galleries or scanned-page
calibration**. The deliberately uncompressed PNG case is an insertion control,
not evidence that ordinarily compressed images usually share byte chunks.

| Fixture | Original files | Byte-exact pack | Pixel-exact pack | Selected sharing |
| --- | ---: | ---: | ---: | --- |
| exact-copies | 644,331 B | 218,311 B | 256,369 B | Independent full representations share once. |
| localized-edits | 644,143 B | 647,573 B | 288,139 B | Pixel tiles 128; about 55% vs originals. |
| translation-borders | 643,861 B | 647,001 B | 748,408 B | Independent fallback. |
| different-resolutions | 607,419 B | 611,030 B | 730,070 B | Independent fallback. |
| recompressed-jpegs | 187,253 B | 174,988 B | 881,464 B | Independent fallback. |
| unrelated-noise | 1,928,603 B | 1,932,124 B | 1,930,287 B | Independent fallback. |
| alpha-hidden-rgb | 643,558 B | 560,482 B | 314,484 B | CDC 8 KiB / tiles 64. |
| high-bit-depth | 241,348 B | 155,831 B | 141,494 B | Fixed 4 KiB / tiles 64. |
| flat-page-edits | 9,520 B | 8,506 B | 15,994 B | Independent fallback; tile metadata erases the advantage. |
| uncompressed-png-prefix-shift | 590,917 B | 237,305 B | 203,459 B | CDC 32 KiB / independent full pixel dedup. |
| mixed-gallery | 678,107 B | 678,757 B | 996,765 B | Independent fallback. |

For localized edits, the tile archive includes 268,626 B unique chunks, 14,891 B
manifests and 4,622 B ZIP indexes. Boundary shifts and resizing erode tile sharing
even where P12's explicit predictor can help. Small, highly compressible pages
lose tile benefits to bookkeeping. These fallback results are part of acceptance.

```sh
make test-gallery-block
python3 -m unittest discover -s scripts/tests -p 'test_*storage.py' -v
python3 scripts/tests/gallery_block_fixtures.py --output p13-benchmark.json
```

On 2026-10-04, **25 P13 acceptance tests**, **22 P12 regression tests** and
**26 measured fixture contracts** passed. Tests include real process exits,
explicit-source repair, empty-environment restore, selected-image I/O, exact
alpha/high-bit-depth/metadata, shared ownership after removal, append without
old source paths, physical allocation and threshold boundaries, hostile archives,
bounded decompress and independent CLI item errors. The dedicated CI workflow
publishes its benchmark artifact. No Go/UI source or dependency lock changed;
their application checks run on GitHub. This container has no Go executable.

Before original eviction, retain the P12
[consumer inventory and production gate](p12-image-delta-storage.md#existing-file-consumers-to-integrate-before-activation):
actual owner Gallery/codec measurements, cross-platform/version conformance,
native logical-media materialization for serve/scan/conversion/inference/export,
journalled catalogue activation and rollback, live dependency/GC protection,
bounded concurrent cache, complete native backup/restore and user-visible review
of the chosen contract and measured savings. P13 completes the last listed
roadmap **prototype** package; production activation remains a separate milestone.
