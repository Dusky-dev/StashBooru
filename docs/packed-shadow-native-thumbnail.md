# Isolated native thumbnail decoding from packed-image bytes

This milestone extends the existing opt-in Settings → Tasks original/shadow
comparison with a third preview: an isolated **native JPEG thumbnail** made
from the verified packed-image member.

The separate endpoint

`GET image/storage-shadow-thumbnail?stackID=...&imageID=...&candidateID=...`

uses the same catalogue selection, authenticated private candidate receipt,
full archive SHA-256 rehash, original-file SHA-256 and byte-for-byte P12/P13
verification as `image/storage-shadow`. The browser cannot supply a path,
width, codec, output filename or write instruction.

After validation, the backend passes the reconstructed bytes directly into
the **same vips/FFmpeg thumbnail backend** used by StashBooru's regular
native encoder. There is no temporary original, no generated cache lookup
or cache output write. The thumbnail is JPEG and is returned through the
existing no-store/nosniff/sandboxed shadow-response helper.

## Format and safety limits

- Static PNG, JPEG, WebP, BMP and TIFF are candidates for in-memory decoding.
  Availability depends on the installed native vips/FFmpeg codecs.
- GIF (animated or otherwise), animated WebP, JXL and AVIF are explicitly
  unsupported in this isolated path. The real native encoder uses a
  seekable filesystem path for some formats, especially AVIF. Rather than
  simulating this by creating a temporary source inside media roots, the
  diagnostic endpoint returns HTTP 415.
- The input is limited to 64 MiB, thumbnail target to 512 pixels, and
  resulting JPEG to 8 MiB. The isolated member decoder retains the 75-second
  process limit. This does **not** add an encoder time limit within the Go
  process, so operational testing is still needed before broad use.
- All native `/image/{id}/thumbnail` and `/image/{id}/preview` paths are
  unchanged. No preview clip, thumbnail cache, metadata or source image
  file is read/written by the encoder except the previously verified source
  needed for original-byte equality.

## Explicitly not proved

This is one opt-in static thumbnail path. It does **not** establish that
cache-generated thumbnails, animated previews, video clips, downloads,
all codecs, image editing and metadata viewers can consume packed sources.
It does not recover after originals disappear, replace native file serving
or authorize archive activation. Original-file eviction remains disabled.

Go regressions cover invalid request parameters, upper bounds and unsupported
format refusal. CI build and Go lint must pass. Real owner-library and
mounted-browser visual tests are still needed, as are selected-format
integration tests with actual native vips/FFmpeg before declaring broad
compatibility.
