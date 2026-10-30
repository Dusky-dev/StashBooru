# Offline rollback evidence checkpoint (scratch only)

This milestone binds the existing immutable saved database/config/media backup
receipt to the independently verified **whole-media scratch recovery**,
**image-primary path rebasing** and **all-file SQLite path rebasing**. It
creates a private restart-auditable checkpoint. It **does not perform an
application rollback** and is not permission to delete originals.

After the existing #285–#287 offline rehearsals succeed, run from the
repository root:

```shell
python scripts/image_storage_disposable_rollback_checkpoint.py prepare \
  --backup-set-receipt /private/saved-backup-set.json \
  --recovered-output /private/recovered-snapshot \
  --image-rebase-output /private/rebased-images \
  --all-file-rebase-output /private/rebased-all-files \
  --output /private/new-rollback-checkpoint

python scripts/image_storage_disposable_rollback_checkpoint.py audit \
  --backup-set-receipt /private/saved-backup-set.json \
  --recovered-output /private/recovered-snapshot \
  --image-rebase-output /private/rebased-images \
  --all-file-rebase-output /private/rebased-all-files \
  --output /private/new-rollback-checkpoint
```

All inputs must be canonical absolute paths. The output parent must already
be private mode 0700, outside saved inputs, backup store and historic media
roots. The output must **not already exist**. The tool is offline and does
not connect to a running StashBooru process.

`prepare` checks all backup-set artifacts and independently audits the
prior scratch recovery and rebased SQLite files *before* creating a new
private 0700 output directory. It then re-runs all audits to reject evidence
that changed during the operation. A single mode-0600, fsynced checkpoint
marker is written with exclusive creation, followed by directory fsync.
This marker contains exact trusted paths, snapshot identifiers, database,
configuration, media manifest and rebased SQLite digests, and the explicit
false safety claims.

After a crash or restart, `audit` opens the saved files again and independently
recomputes the evidence and exact marker. `no_output` means nothing was
created; `incomplete` means the private directory exists but completion
marker was not published; `complete_verified` means every saved artifact
and scratch result matches now. Unknown extra files, symlinks, nonprivate
markers, corrupted claims, missing backed-up objects or modified SQLite
cause a fail-closed error. **Interrupted directories are never resumed or
overwritten.** A new attempt must use another new output directory.

This is only proof that a saved complete-media copy and rebased catalogue
can be audited after restarting the *rehearsal*. It does **not** boot the
application, atomically capture DB/config/media, prove live rollback,
handle cross-process concurrent writes, restore ZIP blobs, or establish
native-media compatibility across all consumer routes. The following
remain **false**: `coherentSnapshotVerified`,
`restoredApplicationBootVerified`, `fullLibraryRecovered`,
`transactionalRollbackVerified`, `crashSafeLiveRollbackVerified`,
`canActivate`, and `originalEvictionEnabled`.

The next milestone is isolated application boot with a strictly rewritten
scratch-only configuration and protected environment; no production database,
source media or credentials may be exposed to that test.
