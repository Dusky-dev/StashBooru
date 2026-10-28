#!/usr/bin/env python3
"""Read-only comparison of P12 image deltas and P13 shared-tile/chunk storage.

Nothing is packed, published, activated, removed or written to the source
directories. This is a candidate-size estimate, NOT a promise of future disk
savings: retaining originals makes physical usage increase until separately
verified application-level activation and eviction exists.
"""

from __future__ import annotations

import argparse
import json
import math
from pathlib import Path
import sys

import image_delta_storage as delta
import gallery_block_storage as tiles


def evaluate_group(group: dict, fidelity: str, minimum_bytes: int,
                   minimum_percent: float) -> dict:
    """Compare verified candidate reconstructions under identical fidelity."""
    choices = []
    errors = []
    for method in ("delta", "shared-tiles"):
        try:
            if method == "delta":
                _manifest, _blobs, report = delta.plan_group(
                    group, fidelity, minimum_bytes, minimum_percent)
                eligible = bool(report["eligibleDelta"])
            else:
                _index, _members, _blobs, report = tiles.plan_gallery(
                    group, fidelity, minimum_bytes, minimum_percent)
                eligible = bool(report["sharingSelected"])
            original = report["originalFileBytes"]
            projected = report["archiveBytes"]
            # A selected layout must beat original separate files, including
            # all archive overhead; reporting intra-archive-only savings
            # as future disk savings would be misleading.
            eligible = eligible and projected < original
            choices.append({
                "method": method,
                "layout": report["layout"],
                "verifiedDuringPlanning": True,
                "eligible": eligible,
                "originalBytes": original,
                "packedBytes": projected,
                "potentialSavedBytes": original - projected if eligible else 0,
                "reason": report.get("reason") or report.get("fallback") or "",
            })
        except (ValueError, OSError, KeyError, TypeError, OverflowError,
                RuntimeError, ImportError) as error:
            errors.append({"method": method, "error": str(error)})
    if not choices:
        return {
            "group": group.get("id", ""),
            "members": len(group.get("members", [])),
            "fidelity": fidelity,
            "error": "no method could evaluate this group",
            "methodErrors": errors,
        }
    baseline = choices[0]["originalBytes"]
    if any(choice["originalBytes"] != baseline for choice in choices):
        return {
            "group": group.get("id", ""), "fidelity": fidelity,
            "error": "storage methods disagreed on original-byte accounting",
            "methodErrors": errors,
        }
    eligible = [choice for choice in choices if choice["eligible"]]
    best = min(eligible, key=lambda x: (x["packedBytes"], x["method"])) if eligible else None
    return {
        "group": group["id"],
        "members": len(group["members"]),
        "fidelity": fidelity,
        "sourceBytes": baseline,
        "recommendedMethod": best["method"] if best else "keep-originals",
        "potentialPackedBytes": best["packedBytes"] if best else baseline,
        "potentialSavedBytes": best["potentialSavedBytes"] if best else 0,
        "physicalBytesIfOriginalsRetained": baseline + best["packedBytes"] if best else baseline,
        "methods": choices,
        "methodErrors": errors,
    }


def estimate(groups: list, fidelity: str = "byte-exact", minimum_bytes: int = 4096,
             minimum_percent: float = 5.0) -> dict:
    if fidelity not in ("byte-exact", "pixel-exact"):
        raise ValueError("unsupported fidelity")
    delta.integer(minimum_bytes, 0, delta.MAX_PACK, "minimum bytes")
    if not math.isfinite(minimum_percent) or not 0 <= minimum_percent <= 100:
        raise ValueError("minimum percent must be finite between 0 and 100")
    results = [evaluate_group(group, fidelity, minimum_bytes, minimum_percent)
               for group in groups]
    total_original = sum(row.get("sourceBytes", 0) for row in results)
    possible_savings = sum(row.get("potentialSavedBytes", 0) for row in results)
    error_count = sum("error" in row for row in results)
    return {
        "version": 1,
        "readOnly": True,
        "experimental": True,
        "sourcesRetained": True,
        "fidelity": fidelity,
        "sourceBytes": total_original,
        "hypotheticalBytesAfterFullActivation": total_original - possible_savings,
        "potentialSavedBytes": possible_savings,
        "physicalBytesBeforeEvictionWithAllCandidatePacks":
            sum(row.get("physicalBytesIfOriginalsRetained", 0) for row in results),
        "groupsChecked": len(results),
        "groupsFailed": error_count,
        "disclaimer": "Hypothetical candidate sizes only. No backup, original eviction, "
                      "catalogue activation, live serving or reconstruction cache included. "
                      "No files were altered; source retention increases physical storage.",
        "results": results,
    }


def main(argv=None) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--stacks", required=True, help="P12 stacks JSON specification")
    parser.add_argument("--fidelity", choices=("byte-exact", "pixel-exact"), default="byte-exact")
    parser.add_argument("--min-bytes", type=int, default=4096)
    parser.add_argument("--min-percent", type=float, default=5.0)
    parser.add_argument("--output", help="optional new report JSON file, never overwrite")
    args = parser.parse_args(argv)
    try:
        delta.require_dependencies()
        groups = delta.read_stacks(args.stacks)
        result = estimate(groups, args.fidelity, args.min_bytes, args.min_percent)
        encoded = json.dumps(result, sort_keys=True, indent=2, allow_nan=False) + "\n"
        if args.output:
            with Path(args.output).open("x", encoding="utf-8") as out:
                out.write(encoded)
        print(encoded, end="")
        return 1 if result["groupsFailed"] else 0
    except (ValueError, OSError, TypeError, ImportError) as error:
        print(f"storage estimator: {error}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
