# Media converter

Open **Convert media…** from an image or video's operations menu. In an image or
video list, select files and choose **Convert selected media…**. The same dialog
shows conversion progress, cache settings, statistics and paginated restore
history. Jobs continue after the dialog closes. A batch continues past individual
failures; each failed item reports its reason.

## Formats and controls

| Output | Controls | Processor |
| --- | --- | --- |
| JPEG XL / animated JPEG XL | Distance 0–15; effort 1–9 | CPU |
| AV1 in MP4, MKV or WebM | Quality 0–100; effort 1–9 | CPU; supported NVENC, QSV or VAAPI hardware |
| H.264 MP4/MOV, HEVC MP4, VP9 WebM | Quality; effort | CPU or supported hardware |
| JPEG, PNG, WebP, AVIF, TIFF, BMP | Controls appropriate to the encoder; WebP lossless option | CPU |
| GIF, animated PNG, animated WebP | Animation-preserving conversion; applicable quality/effort controls | CPU |

Animated JPEG XL uses the normal **`.jxl` extension**, labeled AJXL in the format
selector. JXL distance 0 requests lossless compression. JPEG input at distance 0
uses `cjxl`'s JPEG reconstruction path when installed. Higher effort trades time
for compression. Quality is a normalized slider mapped to codec-specific
CRF/quantizer settings, and is not comparable across codecs. Distance is the JXL
quality control; WebP has a separate lossless switch.

Inputs are read by the worker's FFmpeg build, with Pillow for animated WebP and
`djxl` for JPEG XL. Mainstream still, animation and video containers are supported.
Availability depends on the installed codecs. GPU choices require a successful
test encode on the worker, not just a compiled encoder. An RTX 3060 can use NVENC
for H.264/HEVC, but **AV1 encoding runs on its CPU**. JXL has no GPU option here.
Explicit GPU requests fail when unavailable; the **Prefer GPU, otherwise CPU**
mode permits CPU fallback.

The converter fully decodes outputs and checks dimensions, frame count, duration,
individual animation delays and loop counts when available, and audio stream
count. It refuses to silently flatten animation or discard audio/transparency.
Audio/transparency loss requires the corresponding option. Outputs that are not
smaller are skipped by default; enable **Keep outputs even when they are larger**
to keep them.

Video entries stay videos, with their existing timeline metadata. Image entries
can become animated images or video clips. Files inside archives, interactive
video with funscripts, shared files, separate APNG poster frames, and embedded
subtitle/attachment/data tracks are rejected without replacing the source. Extract
or remux those inputs first. Network playlists are not accepted. Some encoder
versions cannot retain particular animation properties: for example libjxl 0.7
loses finite APNG loop counts. Verification rejects these outputs. Animated WebP
decoding has a 512 MiB decoded-frame memory limit.

Playback still depends on the browser's codec support. JPEG XL, especially
animated JXL, requires a compatible viewer/browser. MP4/WebM usually offer broader
browser support. The converter preserves StashBooru metadata; embedded EXIF,
container metadata, HDR side data and ICC profiles have format/encoder-dependent
support and are not guaranteed to survive every cross-format conversion.

## Identity, originals and recovery

Conversion changes the primary **file on the same media entry**. IDs, title,
description, source URLs, tags, characters, artists, copyrights, collections,
ratings, markers and other database relationships remain attached. It does not
recreate or retag entries.

Original fingerprints are retained as `source_md5`, `source_phash`,
`source_oshash`, etc. The active file receives its actual MD5/OSHash, keeping
scanning and duplicate detection accurate. Source fingerprints survive cache
eviction and subsequent conversions. Booru matching uses the original MD5.
History displays 64-bit pHashes as exact hexadecimal strings. New thumbnails,
previews and pHashes are queued through the existing Generate job after successful
conversion/restoration; existing custom covers are not overwritten.

The encoder writes to a staging directory beside the source. After validation,
the original is copied to the restore cache and verified, a fresh output filename
is published without overwriting a sibling, the file record is switched in a
database transaction, and the old active path is removed. The output filename
includes `.converted-<id>` to prevent collisions. Failed/cancelled encoding keeps
the source. Activation is journaled and completed or rolled back after an
interruption. Once activation begins, cancellation waits for that short critical
section to leave recoverable state.

Originals and journals live in **`media-conversions/` beside `config.yml`**. Keep
that directory with configuration/database backups. The default original cache
limit is **20 GiB**, configurable in the dialog. Oldest completed originals are
permanently deleted until the limit is met. A limit of zero evicts each original
after successful activation. An original larger than the limit will therefore
not remain restorable. Pending activation/recovery data is protected from
eviction. Journals and source fingerprints remain after eviction.

**Restore** restores the previous file version and file metadata while retaining
current user edits to tags and other media metadata. It refuses occupied original
paths, modified converted files and corrupted backups. For repeated conversions,
restore the latest version first. Evicted originals cannot be restored.

Recovery runs on startup and before scans, cleaning and further converter work.
The dialog's **Recover interrupted operations** also runs it. Ambiguous external
file changes retain the available files and report the journal ID; scans/cleaning
stop until that conflict is resolved. Do not manually edit a conversion journal
or discard its cache while an operation is pending.

Statistics show signed per-file savings, total savings, weighted percentage,
average savings per distinct file, larger outputs, retained-original bytes and net
media storage savings. Repeated conversions are combined and restored conversions
are excluded. Net savings subtract retained originals, so they can be negative
before cache eviction. These are recorded file sizes, excluding generated assets,
temporary files, filesystem compression, sparse allocation and manual external
file changes.

## Local and remote setup

The normal and CUDA Docker build images include the converter worker. Local
non-Docker installations need Python 3.10+, Pillow, FFmpeg/ffprobe and libjxl tools
(`cjxl` and `djxl`, strongly recommended). The worker defaults to
`scripts/media_conversion_worker.py`; set `STASH_MEDIA_CONVERSION_WORKER` to its
absolute path when launching StashBooru elsewhere. `STASH_PYTHON` overrides
`python3`. StashBooru's configured FFmpeg/ffprobe paths are passed to the local
worker.

Remote conversion reuses the **Visual Similarity remote tagging URL and bearer
token**. Update the remote checkout to the same branch/version and restart
`scripts/visual_embedding_server.py`; keep `media_conversion_worker.py` beside it.
Install FFmpeg, libjxl tools and Pillow on that machine. Existing model settings
are unchanged and conversion does not download models.

For large videos, increase the worker's upload limit explicitly, for example:

```sh
python scripts/visual_embedding_server.py --host 0.0.0.0 --port 8000 --max-upload-mb 8192
```

The existing default is 512 MiB. `STASH_EMBEDDING_SERVER_TEMP_DIR` selects its
temporary storage volume. Leave enough free space on both machines for input,
intermediate APNG frames, output and the local restore copy. The worker accepts
one conversion at a time and returns a clear busy error to other requests.
Uploads/downloads are streamed; response size and MD5 are verified before local
activation. Disconnecting/cancelling stops the remote encoder at its next process
poll. The database and restore cache remain on the StashBooru server.

Worker environment options:

| Variable | Default |
| --- | --- |
| `STASH_CONVERTER_THREADS` | `4` |
| `STASH_CONVERTER_TIMEOUT_SECONDS` | `86400` |
| `STASH_CONVERTER_FFMPEG` / `STASH_CONVERTER_FFPROBE` | `ffmpeg` / `ffprobe` |
| `STASH_CONVERTER_VAAPI_DEVICE` | `/dev/dri/renderD128` |

Remote endpoints, protected by the existing bearer authentication:

- `GET /v1/convert/capabilities`
- `POST /v1/convert`: binary input, JSON options in `X-Stash-Conversion-Options`;
  binary output, metadata in `X-Stash-Conversion`, checksum in `X-Stash-Content-MD5`.

References: [FFmpeg encoders](https://ffmpeg.org/ffmpeg-codecs.html),
[JPEG XL tools](https://github.com/libjxl/libjxl/tree/main/tools),
[NVIDIA encode/decode support](https://developer.nvidia.com/video-encode-decode-support-matrix).
