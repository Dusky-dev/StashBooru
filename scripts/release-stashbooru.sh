#!/usr/bin/env bash
# Run by the build workflow only after UI validation, tests and all platform builds.
set -euo pipefail

version=$(tr -d '\r\n' < internal/build/stashbooru-version.txt)
[[ "$version" =~ ^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$ ]] || { echo "Invalid stable version" >&2; exit 1; }
[[ "$GITHUB_REPOSITORY" == "Dusky-dev/StashBooru" ]] || exit 1
[[ "$GITHUB_REF" == "refs/heads/releases/stashbooru-$version" ]] || { echo "Release branch must match version file" >&2; exit 1; }
[[ "$(git rev-parse HEAD)" == "$GITHUB_SHA" ]] || exit 1
tag="stashbooru-v$version"
notes="docs/releases/stashbooru-$version.md"
[[ -s "$notes" ]] || { echo "Release notes missing" >&2; exit 1; }
# Never move an existing stable tag or replace a release's assets on a retry.
if git ls-remote --exit-code --tags origin "refs/tags/$tag" >/dev/null 2>&1; then
  echo "Release tag already exists; refusing to replace it" >&2
  exit 1
else
  status=$?
  [[ "$status" == 2 ]] || { echo "Could not verify release tag absence" >&2; exit "$status"; }
fi
assets=(Stash.app.zip stash-macos stash-win.exe stash-linux stash-linux-arm64v8 stash-linux-arm32v7 stash-linux-arm32v6 stash-freebsd stash-ui.zip)
for asset in "${assets[@]}"; do
  [[ -s "dist/$asset" ]] || { echo "Missing asset: $asset" >&2; exit 1; }
done
(cd dist && sha256sum "${assets[@]}") > CHECKSUMS_SHA256
# Keep the release a draft until every asset is uploaded successfully.
gh release create "$tag" --repo "$GITHUB_REPOSITORY" --target "$GITHUB_SHA" \
  --title "StashBooru $version" --notes-file "$notes" --draft \
  "${assets[@]/#/dist/}" CHECKSUMS_SHA1 CHECKSUMS_SHA256
gh release edit "$tag" --repo "$GITHUB_REPOSITORY" --draft=false --latest
