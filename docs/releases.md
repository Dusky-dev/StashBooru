# StashBooru release maintenance

The authoritative fork version is `internal/build/stashbooru-version.txt`.
Go embeds it and Vite reads the same file. Upstream Stash's `build.Version()` and
`VITE_APP_STASH_VERSION` keep their existing meaning; `git describe` only considers
upstream-style `v[0-9]*` tags. Never use a bare `v1.0.0` tag for a StashBooru release.

For a stable release:

1. Update the version file and `docs/releases/stashbooru-X.Y.Z.md` in a reviewed
   branch. Complete UI, Go, mounted browser and CI checks for that exact commit.
2. With the owner's release authorization (standing since 2026-11-19, see
   `AGENTS.md`), create `releases/stashbooru-X.Y.Z`
   at that tested commit. This deliberately triggers publication; it is not a
   general-purpose review branch. It does not merge `develop`.
3. The Build workflow validates release guards, UI and Go tests and builds every
   platform. Its release job verifies the branch/version/repository, commit and
   required assets, then creates `stashbooru-vX.Y.Z` at the exact commit.
4. The GitHub release stays a draft until every server download, the native
   app builds (from the `native-app` job of the same run) and the checksum files
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

## Versions, introduction and update notes

Owner decision (2026-11-19): every user-visible change is an update. Each such
PR bumps `internal/build/stashbooru-version.txt` (minor for features, patch for
fixes) and the native app's matching version (`app/pubspec.yaml`
`version:` and `kAppVersion` in `app/lib/state/app_settings.dart`; the app
tests check they match) and adds a newest-first entry under
`ui/v2.5/src/docs/en/Changelog/StashBooru/` (a `X.Y.Z.md` file plus its line in
`index.ts`). Settings → About lists every StashBooru version above the
retained upstream history.

On start the app decides what to show from two UI settings:

- **Nothing acknowledged yet** (`lastNoteSeen` and `lastVersionSeen` both
  unset: a fresh install, including a pre-written configuration): the
  StashBooru introduction, titled "Welcome to StashBooru". The setup wizard
  shows the same introduction on Welcome, and Finish acknowledges it.
- **Otherwise**: the changelog entries newer than `lastVersionSeen`, up to the
  running version. A missing or unreadable `lastVersionSeen` counts as 1.0.0,
  the version before notes were versioned.

Get started / Close records `lastVersionSeen` (running version) and
`lastNoteSeen` (newest dated note); Escape does not. If saving fails the dialog
closes for that session and returns on the next load. Upstream release notes
no longer open as a prompt; they stay in Settings → About.

Publishing a stable release still needs `docs/releases/stashbooru-X.Y.Z.md`
(the GitHub release text); copy it from the in-app entry.
