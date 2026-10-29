# Durable experimental image candidate receipts

Experimental P12/P13 candidate archives now write a separate **private immutable
receipt** after byte-exact packing and original-media backup verification.
Each receipt is stored next to its archive under the server-managed
`image-storage-candidates` directory, outside the active image library and
backup store. The receipt contains a versioned copy of the verified candidate
report and a streaming SHA-256 fingerprint of the published archive itself.

Candidate identifiers are server-generated names like
`stack-17-delta-0123456789abcdef01234567`. The API never accepts media paths
or archive paths. Saved receipts are enumerated with bounds, and the Settings
→ Tasks isolated-recovery control offers a selectable candidate, including
ones created before a server restart. Only 48 candidates are displayed
per status response; further receipts are retained but are not directly
selectable by the current UI. Untrusted receipt IDs, malformed JSON,
symlinks, missing archives and unexpected path nesting are rejected.

**Every recovery job** re-reads the receipt, recomputes the full archive
SHA-256, rechecks the stored candidate contract and source identity against
the live Visual Stack, then independently reconstructs all P12/P13 members
into a new private directory and rehashes every restored file. Enumeration
is not itself a verification certificate: an archive is checked again when
a user actually queues recovery.

## Crash and failure behavior

Proof receipts are written to a temporary file, fsynced and published through
a same-directory no-overwrite hard link so an interrupted write cannot expose
an incomplete final receipt or overwrite another. If writing the receipt
fails after the archive has been verified, the job reports failure and the
unregistered archive remains for administrator inspection; it is not
activated or automatically removed. Server restarts preserve published
archives and receipts. Any temporary receipt files are ignored and require
manual cleanup after confirming no job uses them.

The old one-job in-memory candidate result is still shown for convenience,
but saved receipts—not that transient result—are the authority for selecting
a candidate to reconstruct.

**Still not implemented:** catalogue/media serving from archives, any
in-place reconstruction or rollback, original eviction, database/config
recovery, resumable native packing or full lifecycle/retention cleanup.
No storage savings are realized. Live originals remain authoritative.
A saved receipt and archive checksum alone do not prove a production
restore; the separate byte-exact reconstruction step is still mandatory.
Synthetic CI and Go unit tests do not replace owner-library trials.
