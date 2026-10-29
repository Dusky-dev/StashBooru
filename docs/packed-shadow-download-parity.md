# Read-only packed-image original-byte download compatibility

This milestone adds a separate, explicit native compatibility endpoint:

GET /image/storage-shadow-download?stackID=...&imageID=...&candidateID=...

The browser supplies only a numeric Visual Stack ID, numeric image ID and
server-generated candidate identifier. All original-file paths and archive
paths are resolved server-side, and unexpected query keys (such as filename,
output or source) are rejected.

It reuses the existing shadow image read pipeline, which:
- Revalidates current native image membership, configured private candidate
  directory, immutable receipt and full candidate archive SHA-256.
- Revalidates P12/P13 byte-exact source hashes against existing originals.
- Independently reconstructs the selected member and requires its bytes and
  SHA-256 to match the current original before returning any HTTP bytes.

The download handler returns the *unchanged original format* as
application/octet-stream, forces Content-Disposition attachment and uses a
server-generated name such as image-123.png. Only approved image extensions
are retained from the validated original path; the actual original name,
directory, and arbitrary extensions are not included. The handler supports
HTTP byte-range and HEAD semantics via net/http.ServeContent, with no-store,
nosniff and restrictive sandbox CSP. File payload remains limited by the
existing 64 MiB shadow source bound and 75-second Python reconstruction
timeout. An animated or otherwise browser-unsupported format can be
downloaded without any re-encoding.

This is distinct from the native image/{id}/image route; it does **not**
reroute downloads or reconstruct files on disk. The UI now offers the
explicit verified download in the side-by-side comparison.

## Preview compatibility boundary

Native image/{id}/preview currently serves a generated clip-preview
file from generated storage, not a still image dynamically decoded from the
original P12/P13 archive. The comparison UI offers a separate link to inspect
this *existing* clip-preview file (possibly 404) without generating a new
one. Its existence does not establish that a packed-source preview can be
regenerated, nor does it test playback or source replacement.

## Tests and limitations

Go synthetic HTTP tests cover complete original-byte download, byte-range
and HEAD responses, mandatory safe headers, extension sanitization, and
request rejection before any filesystem access. Existing Python P12/P13
shadow tests cover fidelity and corruption refusal. Full CI is required
before merging.

The endpoint remains read-only, requires originals to exist, and does not
enable packed serving for any existing media consumer, alter metadata,
activate candidates, delete originals, verify database/config recovery,
or provide crash-safe rollback. Owner-library/manual browser tests remain
outstanding.
