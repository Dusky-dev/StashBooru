# Experimental byte-exact image packing proof (offline only)

This is the **next, non-destructive** step after read-only storage estimates.
It exercises actual P12/P13 archive writing and persisted reconstruction against
the SHA-256 of every selected original. It does **not** activate packed media,
change database records, change any original image, or free disk space.

## Requirements

Use Python 3 with Pillow and NumPy in a dedicated environment. The explicit
input is the same JSON stacks format as [image-storage-estimates.md](image-storage-estimates.md).
Choose one existing image-only group containing 2–16 images. Only **byte-exact**
fidelity is supported; pixel-exact representations cannot restore original
container bytes.

Choose a new **absolute** archive filename in an existing, separate directory
outside the original image directory. The destination must not exist; it is
published using atomic no-overwrite hard-link semantics.

For P12 deltas:

    python3 scripts/image_storage_pack_proof.py pack --stacks /path/stacks.json --group stack:17 --method delta --output /separate/proof.p12

For P13 shared blocks:

    python3 scripts/image_storage_pack_proof.py pack --stacks /path/stacks.json --group stack:17 --method shared-tiles --output /separate/proof.p13

The probe fingerprints every original file, plans a bounded candidate, writes
it to a private staging directory, verifies the persisted archive, reconstructs
every member, compares reconstructed bytes against the original SHA-256, checks
that source files are unchanged, and **only then publishes the new archive**.
The report shows retained-originals plus archive bytes. This is additional
disk usage, not realized savings.

## Interrupted operation and recovery

After an interruption, a hidden .stash-pack-proof-* directory may remain beside
the requested archive. If it contains candidate.pack, it can be inspected and
resumed only after the archive's own byte-exact verification succeeds:

    python3 scripts/image_storage_pack_proof.py resume --staged /separate/.stash-pack-proof-EXAMPLE/candidate.pack --method delta --output /separate/recovered.p12

Choose shared-tiles for a P13 candidate. Incomplete or corrupt candidates must
not be published. The original P12/P13 tools also provide verify and extract
commands to reconstruct copies into a **new** directory without changing
originals. Recovery does not require source images; the pack is self-contained.

Unneeded proof packs or abandoned staging directories can be cleaned up only
after confirming that no process or workflow still uses them. Never treat these
experimental archives as authoritative media.

## Explicit safety limitations

- No verified production backup prerequisite or native-serving proof is
  established by this experiment.
- This probe does not update catalogue records, media paths, file activation,
  metadata, native background jobs, or library serving.
- It does not replace a real incremental media backup.
- There is no original-file eviction or optimization toggle. **Do not delete
  originals**, even when estimated savings are positive.
- JSON-supplied paths are operator inputs for an offline command, not browser
  APIs. Future native integration must derive paths solely from StashBooru's
  catalogue.
- Tests use synthetic PNG fixtures, not a live library. Real-media calibration,
  native recovery UX, rollback, interrupted native jobs, and all media consumers
  remain future milestones.

The eventual Reconstruct Library recovery action must remain accessible even
when experimental optimization is disabled.
