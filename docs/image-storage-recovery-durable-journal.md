# Durable latest-attempt journal for scratch-only P12/P13 recovery

This milestone extends the **private missing-original recovery rehearsal**; it is not live-library restoration.

## Persisted record

Immediately before the isolated Python recovery writes any scratch bytes, the native Go job saves a bounded `missing-original-latest-journal.json` inside the protected candidate directory. It uses a same-directory temporary file, file synchronization, an atomic rename and directory synchronization where supported. The previous journal must be a safe regular file with a valid schema before replacement.

The record contains the trusted server-generated candidate identity, backup snapshot, new private scratch output path, start/update timestamps, the last recorded state, and—only after verified completion—the recovered file count and SHA-256 of the native recovery report.

- `running`: a native recovery attempt was recorded before writing scratch files.
- `failed`: the native recovery script returned an error; no completed proof is retained in the journal.
- `scratch_complete`: the native process finished and its byte-exact scratch receipt was verified *at that time*. This is not a live restoration.
- `interrupted_or_unconfirmed`: display-only interpretation of a persisted `running` record when no corresponding in-process job is running after a restart.

Settings → Tasks displays the last saved record and clearly notes that **any** persisted state requires a fresh scratch audit. No persisted success is promoted into the current session's live validated result. Only the latest recovery attempt is journaled in this initial milestone.

## Deliberate limitations

This journal does **not** replay/resume a failed job, clean partial files, check historical image-ID-to-backup-path mapping, rehash the archive or backup on startup, restore the database/configuration, replace any active media routes, authorize eviction, or prove crash-safe rollback. It is a small restart-awareness step before a full transactional recovery coordinator. An unexpected power loss can leave scratch output whose journal state is `running`; inspect it using the separate read-only audit from PR #272.

Malformed, symlinked or oversized journal files prevent further writes rather than being silently overwritten. Synthetic Go tests exercise interrupted and completed states, restart interpretation, unsafe paths, forbidden activation flags, symlinks and corrupted records.
