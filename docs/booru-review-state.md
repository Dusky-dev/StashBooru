# Booru review state — B02

B02 adds library-wide review marks to the existing browser. It does not download
remote originals, import metadata/entities, change native media relationships or
create jobs. The browsing layout remains the B01 layout while broader browsing
revisions await owner testing.

## Review a provider page

Each post offers **Ignore** / **Unignore** and **Mark downloaded** / **Clear manual
mark**. These are independent choices. Unignoring never clears a download mark.
A manual download mark records a historical claim and the time it was marked;
it does not assert that any file currently exists or create a local media link.

**Select visible** or individual **Select** checkboxes prepare bounded bulk edits
on the current source page. Bulk ignore/unignore and manual mark/clear operations
are atomic: one stale version rejects the entire selection without partial edits.
**Undo last review change** reverses the last successful change on this source
page. Undo uses the returned versions, so another client's edits cannot be
silently overwritten. Undo is a new revision; reapplying a cleared flag records
the new mark time. Selection and the last undo action are temporary UI state.
The durable marks live in the server database.

Ignored posts are hidden by default. **Show ignored posts** restores them for
review or unignore; its `ignored=1` URL parameter survives refresh/back. Filtering
only changes the currently loaded provider page. The header reports provider
posts, shown posts and ignored posts hidden; the provider's totals, Previous/Next
and page numbers remain unchanged. A page with every post ignored still has its
provider pagination. There is no automatic page filling, remote query rewriting,
cross-page review selection or all-ignored-history listing in B02.

**Reload review status** fetches current marks and rechecks local evidence without
searching the provider again. Returning focus to the browser also refreshes review
status. A conflict leaves the edit unapplied; reload status before trying again.
If review storage cannot be read, the browser reports the failure, disables review
writes and keeps posts visible rather than treating unknown state as ignored.

## Current local matches

**In library · Image #…** / **In library · Video #…** links are separate from manual
marks. Each link identifies a native media kind/ID and its active primary file,
with evidence `current-file-md5` and a check time. Images and Videos with equal
numeric IDs remain distinct.

The server finds indexed native primary-file MD5 candidates, hashes their actual
current bytes and compares them with the remote provider's MD5 claim. It checks
that the file stayed stable while reading and that the native primary-file link
still exists after the read. Derived local links are not permanently stored:
file deletion, replacement, primary-file activation or native media deletion is
re-evaluated on the next status refresh. Manual history remains independent.
Filename hashes, `source_md5`, pHash similarity, inactive historical files and
manual marks do not become proof that an original is currently present.

This is conservative, bounded matching rather than an exhaustive catalogue scan:

| Display | Meaning |
| --- | --- |
| In library link | Current primary-file bytes match the MD5 supplied by the provider. This relies on that provider's hash metadata; no remote original is fetched. |
| Local match unknown · no remote MD5 | The provider supplied no supported content MD5. |
| No indexed primary-file MD5 match | No matching active candidate was found in the native MD5 index; this does not prove absence from the entire library. |
| Local check incomplete | Candidates could not all be verified, the indexed file changed/is missing, the database check failed or a resource limit was reached. |

A status request handles at most 50 remote posts and 100 native candidates. It
reads at most 64 MiB per regular file and 256 MiB in total, with a five-second
context budget and two concurrent verification requests. ZIP members, oversized
files and unavailable files remain unverified. No persistent checksum cache hides
file changes, and no large library rehash or model inference starts automatically.

## Identity and persistence

Marks are keyed by the normalized API engine + instance URL and the exact remote
post ID. Credentials, display names and editable source IDs are excluded from the
instance digest. Renaming a source or changing its API key keeps its marks.
Changing its URL/engine isolates its new posts from old history; returning to the
old instance restores the old history. Two configuration aliases of the same
normalized engine/URL share review state. Other providers' equal post IDs remain
separate; there is no cross-provider/hash-wide ignore.

Migration **97** adds `booru_review`, containing versioned marks, timestamps and
public source provenance. Cleared rows retain their versions so a clear/re-mark
cycle cannot make an old edit valid again. Native media/file deletion does not
cascade into manual review history. Normal database backups include these marks.
They are shared across this server's users, not private account-specific lists.

The normal server authentication applies to `POST /booru-browser/review/read`
and `POST /booru-browser/review/update`. Both require bounded strict JSON and
reject cross-origin browser requests. Clients provide the displayed instance
digest; a settings change returns 409 before review state can be applied to a
different instance. API keys are never stored in review records.

## Verification

The B02 integration suite covers a fresh database, upgrade from schema 96,
close/reopen persistence, cross-instance identity, independent marks, atomic stale
bulk edits, undo/tombstones, concurrent clients, current-file hashing, replacement
with the same size/mtime, missing files and primary-file removal. Browser fixtures
cover single/bulk changes, undo, URL filters, reload persistence, conflict recovery,
manual marks without invented local links and typed native links on desktop/mobile.
Fixtures use synthetic local bytes and mocked provider APIs; see the progress
ledger for the actual checks and remaining live-account limitations.
