#!/usr/bin/env python3
"""Native-job candidate archive writer: verified media backup, byte-exact pack.

The *server* owns all paths supplied to this command. There is no native resume,
catalogue activation, original replacement, or original-file eviction.
"""
from __future__ import annotations

import argparse
import json
from pathlib import Path
import sys

import image_delta_storage as delta
import image_storage_backup_preflight as backups
import image_storage_pack_proof as proof
import media_library_backup as backup


def candidate(group, store, snapshot, method, output):
    if method not in proof.METHODS:
        raise ValueError("only byte-exact P12/P13 candidates are supported")
    # Check the entire media snapshot and the selected original bytes *before*
    # planning or creating the candidate. Publication rechecks them below.
    initial = backups.preflight(group, Path(store), snapshot)
    if not (initial["verified"] and initial["mediaBackupVerified"]):
        raise ValueError("original-media backup is not verified")

    def checkpoint(phase, staged):
        if phase == "verified":
            fresh = backups.preflight(group, Path(store), snapshot)
            if fresh["imageSHA256"] != initial["imageSHA256"]:
                raise ValueError("source/backup mismatch before candidate publication")

    report = proof.pack_proof(
        group, method, Path(output), 4096, 5.0, checkpoint=checkpoint
    )
    if (not report["verified"] or report["sourceSHA256"] != initial["imageSHA256"]
            or not report["sourcesRetained"] or report["originalEvictionEnabled"]
            or report["nativeServing"]):
        # These invariants are checked before publication inside pack_proof.
        # A mismatch here is unexpected and must never trigger activation.
        raise ValueError("candidate did not satisfy the byte-exact proof")
    return {
        **report,
        "group": group["id"],
        "backupVerified": True,
        "mediaBackupSnapshot": snapshot,
        "nativePackingCandidateOnly": True,
        "nativeServingVerified": False,
        "databaseBackupVerified": False,
        "originalEvictionEnabled": False,
        "archiveActivated": False,
    }


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--stacks", required=True)
    parser.add_argument("--group", required=True)
    parser.add_argument("--store", required=True)
    parser.add_argument("--snapshot", required=True)
    parser.add_argument("--method", choices=proof.METHODS, required=True)
    parser.add_argument("--output", required=True)
    args = parser.parse_args(argv)
    try:
        delta.require_dependencies()
        groups = delta.read_stacks(args.stacks)
        selected = next((g for g in groups if g["id"] == args.group), None)
        if selected is None:
            raise ValueError("unknown selected Visual Stack")
        result = candidate(selected, args.store, args.snapshot, args.method, args.output)
        print(json.dumps(result, sort_keys=True, allow_nan=False))
        return 0
    except (backup.BackupError, ValueError, OSError, KeyError, TypeError) as error:
        print("native candidate proof: " + str(error), file=sys.stderr)
        return 1
    except KeyboardInterrupt:
        print("Candidate job interrupted; originals remain authoritative.", file=sys.stderr)
        return 130


if __name__ == "__main__":
    raise SystemExit(main())
