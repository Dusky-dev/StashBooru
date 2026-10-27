#!/usr/bin/env python3
"""Compatibility entry point for the renamed StashBooru remote worker.

Prefer remote_worker_server.py. Imports resolve to the new module, preserving
shared server state and public names for existing integrations.
"""
import sys
import remote_worker_server as _worker

if __name__ == "__main__":
    raise SystemExit(_worker.main())

sys.modules[__name__] = _worker
