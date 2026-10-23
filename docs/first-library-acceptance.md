# First-library acceptance

The real-server browser check in
[`first-library.mjs`](../ui/v2.5/tests/browser/first-library.mjs) walks the current
setup flow with an empty configuration and database. It creates its own temporary
folders and starts its own loopback server; it does not accept a running server
URL or an existing library. No HTTP, GraphQL, indexing or save responses are mocked.

## Current first-use path

1. Start StashBooru and choose where to keep its configuration.
2. In **Set up your paths**, use **Add directory** to choose a folder of Images
   and Videos. Review the database, generated, cache and binary-data locations.
   These hold different data from the media folder. Set credentials if needed,
   review the paths, then **Confirm**.
3. Read the StashBooru introduction and select **Finish**. This opens
   **Settings → Library**; it does not start a scan.
4. Open **Tasks → Update library → Scan** to index the configured folders.
   The task queue shows the running scan. Use **Selective scan** when only some
   folders should be indexed.
5. Open **All** to see Images and Videos together. Opening an Image thumbnail
   shows its viewer; its title at the bottom opens the native Image detail page.
   Use **Edit**, change the title and **Save**. Refresh to check the saved value.

## Run the isolated check

Build the application with the repository's locked toolchain first:

```sh
make pre-ui
make generate
make ui-only
make stash
```

Install Playwright 1.57.0 and its Chromium browser separately from the locked UI
dependencies. Provide its module path if it is outside normal module resolution.
FFmpeg and FFprobe must be on `PATH`; the suite deliberately refuses to use
Welcome's codec download option.

```sh
STASH_BROWSER_BINARY="$PWD/stash" \
PLAYWRIGHT_MODULE=/absolute/path/to/playwright/index.mjs \
STASH_BROWSER_SCREENSHOTS=/absolute/path/to/output \
node ui/v2.5/tests/browser/first-library.mjs
```

`CHROMIUM_EXECUTABLE` can select an installed Chromium executable. `FFMPEG` can
select the encoder used to create the sample Video; the server still needs to
find both codec tools on `PATH`. The binary must contain a production UI build.
No model download, provider key, external source or existing media is needed.

The suite creates a geometric PNG and a two-second H.264 MP4 for each of two
independent runs: 1440 × 1000 desktop and 390 × 844 mobile. It performs setup,
checks Finish's Library destination and persisted note acknowledgement, starts a
real scan from Tasks, checks one typed Image and one typed Video with decoded
thumbnails in All, opens the native Image editor through its viewer, saves a
title, reloads, restarts the server and checks the same native ID and title.
It also checks for page errors and editor page overflow.

Successful temporary instances are removed; a failed instance is retained at the
printed path for diagnosis. Requested output contains screenshots and server
logs. The main Build workflow reuses its Linux binary artifact for this check on
PRs and uploads `first-library-artifacts`; it does not compile a second binary.

## Scope and remaining acceptance

This is automated acceptance against actual SQLite, scanning, generated
thumbnails and metadata persistence. It is not owner usability testing or a
representative production library. It does not establish Video playback/codec
coverage, account security, upgrades, non-root hosting, Windows/macOS behavior,
provider/model integrations, relationship editing or backup recovery. Continue
E07/E12 with selection, viewer return navigation and native relationships in the
same isolated-library approach; keep those separate from first-use acceptance.
