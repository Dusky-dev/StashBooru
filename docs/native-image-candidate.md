# Native experimental image candidate packing

Adds an **opt-in native job** for one image-only Visual Stack (2–16 images).
Only byte-exact P12 delta or P13 shared-tile archives are supported.

## Native UI and inputs

Settings → Tasks → Inspect media → Experimental image candidate packing.
Select one numeric Visual Stack ID, an existing original-media backup
snapshot and the method; check the explicit disk-usage acknowledgement.
Follow the queued job and refresh the report.

The browser passes only stack ID, snapshot ID, method and acknowledgement.
No browser-supplied source paths, archive paths, output directories, or
original-media eviction options are accepted. The native server resolves
original primary-image paths from the catalogue and the backup store from
the persisted media-backup configuration.

## Verification

Candidates are written only to the application-config directory
`image-storage-candidates`, never into media roots or the backup
store. The server rejects nested or overlapping directories, symlinks,
non-private candidate folders and reused names. The output file has a
cryptographically random name and is published without overwriting.

The native wrapper:
1. Verifies the entire original-media snapshot and all selected current
   image SHA-256 hashes before packing.
2. Writes an experimental P12/P13 candidate in private temporary staging.
3. Independently reconstructs and hashes each archive member, comparing
   every byte against the original.
4. Re-verifies the whole backup and current sources just before publication,
   then checks source identity and SHA-256 again.
5. Atomically publishes a new file without removing any original.

The native job validates the proof result: exact stack/method/snapshot,
byte-exact fidelity, hashes, member counts, archive size, original retention,
and explicit absence of serving, activation and original eviction.

## Limits and recovery

**No disk space is saved yet.** The experimental candidate is extra storage.
It is not served by the native image routes, registered as active media,
or substituted for an original. A media-file backup does not contain the
database or configuration. Backup and restore of those remain independent.

The successful result lives in memory and is lost on restart. The published
archive persists. An interrupted pack can leave hidden
`.stash-pack-proof-*` staging directories; originals remain
authoritative. Existing offline P12/P13 verify and extract tools can reconstruct
copies to **new separate directories**. Native archive resume, rollback
and the always-available Reconstruct Library recovery action are still later
milestones. No cleanup or eviction of originals is implemented.

The new synthetic CI suite covers both byte-exact formats, corrupted and stale
backups, corruption before publication, no-overwrite, symlink/overlap checks,
and native result validation. No production library, actual restore, or
mounted-browser deployment test is claimed.
