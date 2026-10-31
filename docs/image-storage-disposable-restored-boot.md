# Disposable restored-app boot acceptance (P12/P13)

The real-server P12/P13 Playwright acceptance now exercises a **separate
restored StashBooru instance** after its ordinary native-image and prepared
canary checks. It is an automated fixture-only rehearsal; it does not offer
a restore or activation command for a user's library.

The test creates two tiny deterministic images, indexes them through a real
StashBooru server, and builds both experimental candidate formats. After
verifying the image routes and disarming the trial, it stops the fixture
server, takes a SQLite backup through Python's SQLite backup API, and saves
the fixture configuration as backup evidence. It then verifies an existing
original-media backup snapshot and runs these offline operations in a newly
created private directory:

1. Create a saved DB/config/media backup-set receipt.
2. Recover the whole saved media snapshot into scratch.
3. Rebase primary-image folders in a separate SQLite clone.
4. Rebase all regular-file folders in another SQLite clone.
5. Prepare and independently audit the restart checkpoint.
6. Clone the all-file-rebased database again for the application's writable
   database, generate a **fresh allowlisted** scratch-only configuration,
   and boot a new real StashBooru process bound to `127.0.0.1`.
7. Query its actual GraphQL catalogue and ordinary image GET endpoints,
   checking the same IDs and exact image bytes.
8. Shut down the restored instance, confirm the source fixture images,
   saved database and saved config are unchanged, and re-audit the checkpoint.

The boot uses a separate working directory, isolated HOME/XDG paths, no
inherited STASH_* overrides, no saved credentials or plugin settings, private
cache/generated/blob/backup directories, and no configured remote sources.
Proxy variables are set to a dead loopback endpoint. It runs only inside
the CI-owned `stash-packed-native-e2e-*` temporary fixture and does **not**
start or reconfigure any user server.

The real-binary exercise is part of the Build workflow's
`packed-native-acceptance` job, on pull requests. The separate syntax check
is **not** a substitute for this live restored-app boot.

## What this does not prove

The test does not establish coherent live DB/config/media capture,
multi-process transactional rollback, original-file eviction, crash-safe
live restoration, full-library restore, external network isolation, or
compatibility with every media and ZIP-backed source. It deliberately
does not mark `canActivate`, `originalEvictionEnabled`,
`transactionalRollbackVerified`, or
`crashSafeLiveRollbackVerified` true. It does not activate packed-image
routing; the restored process serves retained original fixture bytes.

For a future real-user recovery tool, require explicit dry-run verification,
a containment boundary stronger than configuration/proxy scrubbing, and
cross-process coordination before any changes to a live installation.
