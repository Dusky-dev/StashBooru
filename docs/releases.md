# StashBooru release maintenance

The authoritative fork version is `internal/build/stashbooru-version.txt`.
Go embeds it and Vite reads the same file. Upstream Stash's `build.Version()` and
`VITE_APP_STASH_VERSION` keep their existing meaning; `git describe` only considers
upstream-style `v[0-9]*` tags. Never use a bare `v1.0.0` tag for a StashBooru release.

For a stable release:

1. Update the version file and `docs/releases/stashbooru-X.Y.Z.md` in a reviewed
   branch. Complete UI, Go, mounted browser and CI checks for that exact commit.
2. With the owner's release authorization, create `releases/stashbooru-X.Y.Z`
   at that tested commit. This deliberately triggers publication; it is not a
   general-purpose review branch. It does not merge `develop`.
3. The Build workflow validates release guards, UI and Go tests and builds every
   platform. Its release job verifies the branch/version/repository, commit and
   required assets, then creates `stashbooru-vX.Y.Z` at the exact commit.
4. The GitHub release stays a draft until all nine downloads and checksum files
   have uploaded, then becomes the latest stable release. Verify its commit,
   assets and `/releases/latest` response before announcing completion.

Existing tags/releases are never overwritten by this script. An interrupted
upload can leave a draft/tag; inspect and resolve it explicitly before retrying.
Do not move a published tag. Publish a new patch version for release corrections.
The legacy `latest_develop` updater pushes only its own tag ref.

The UI build cache includes the fork version, Vite config and commit SHA so it
cannot reuse stale version/hash metadata. `UPDATE_REPO` may still be set at build
time for another fork, which must use the same StashBooru stable tag convention.
The default is `Dusky-dev/StashBooru`. The update checker rejects prereleases,
drafts and upstream tag names, compares numeric stable versions, and retains
Stash's platform asset selection with a release-page fallback.
