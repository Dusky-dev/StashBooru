# Agent instructions for StashBooru

These instructions apply to work in `Dusky-dev/StashBooru`. Check for more
specific instructions in the directories you edit. The user's current request
and explicit decisions take precedence over this file.

## Authority and delivery

- The fork owner authorizes agent-assisted implementation, local commits,
  feature-branch publication, and opening/updating PRs in this repository.
  Standing authorization for this and future PRs was given on 2026-10-01.
  Do not ask again for ordinary work, branch pushes, or PR creation within an
  assigned task. PR descriptions must disclose AI assistance accurately.
- Merging PRs, pushing directly to `develop`, deleting remote branches,
  destructive data changes, and production deployment require separate user
  authorization. Permission to create PRs does not authorize these actions.
- `docs/CONTRIBUTING.md` and `docs/AI_POLICY.md` contain inherited upstream
  policy. The owner's explicit authorization above governs agent work in this
  fork; it does not authorize contributions to `stashapp/stash` or other repos.
- Use `.github/pull_request_template.md`. An owner-requested fix can identify
  the request and related PR instead of inventing an issue or claiming human
  authorship/manual testing. Keep each PR focused on one coherent outcome.
- If an action is blocked, finish the unaffected implementation and checks,
  identify the exact blocker, and provide a reviewable branch or patch.

## Starting and resuming work

- Inspect the working tree, current branch, relevant open PRs, and fresh
  `origin/develop`. Record the baseline SHA. Preserve unrelated local changes.
- Use a feature branch or isolated worktree. Continue an existing task branch
  when appropriate; do not recreate features already implemented.
- Start with `docs/implementation-progress.md` and the relevant package notes.
  Handoff files are checkpoints; current code, Git refs and verified runtime
  behavior determine the actual state. Do not implement an entire roadmap when
  the user requests one package or correction.
- Search narrowly with `rg`. Batch independent reads/checks when safe; keep
  dependent operations and publication steps sequential.
- Prefer the connected GitHub tools for publication when shell Git credentials
  are unavailable. Publish the verified tree, check its SHA/content against the
  local result, and preserve commit ancestry. Never embed credentials in files.
- Keep the progress ledger current with baseline, branch/PR, implemented
  behavior, exact checks/results, remaining limitations and the next step.
  Put dated verification SHAs in the ledger. Keep this instruction file free
  of personal workspace paths, secrets and transient branch/commit IDs.

## Product conventions

- User-facing terminology: **Video** = Scene, **Character** = Performer,
  **Artist** = Studio. Preserve native internal identifiers and typed links.
  Copyrights are native entities, not ordinary Tags.
- Navbar order: All, Images, Videos, Characters, Artists, Copyrights, Tags,
  Galleries, Collections. Respect enabled-menu preferences and keep DOM,
  keyboard and visual order consistent. Use the capitalized All message.
- Character/Variant card images are portrait, Artist images square, and
  Copyright images landscape. Reuse existing card components; fit images
  without distortion. Copyrights and Variants share the Character detail row,
  stay close together, and wrap/scroll within viewport bounds.
- Global All uses native list/filter/selection/card/player infrastructure.
  Do not reintroduce duplicate All/Images/Videos tabs inside that page or
  recursive rewriting of native React trees. Keep typed media identity, such
  as `image:123` versus `scene:123`, through selection, URLs and actions.
- Reuse supported native extension points and UI components. Check working
  Images/Videos pages before introducing a parallel implementation. Shared
  viewers must retain their active caller and clean up on dismissal; Video
  previews advance only through explicit navigation.
- Preserve media IDs, native relationships, metadata, provenance, file activation
  and restore behavior. Inherited associations remain calculated unless the
  current task explicitly changes that contract; distinguish them from direct
  assignments and reject hierarchy cycles.
- StashBooru owns the database and jobs. Remote workers perform bounded
  processing and return results. Honor explicit remote-only/GPU-only choices;
  use configured fallbacks where supported. Do not download large models
  without authorization. Long jobs need cancellation, cleanup and item errors.

## Validation

- Match checks to the change. Documentation-only edits need review and
  `git diff --check`; avoid full builds or artificial tests for such edits.
- Backend changes: use focused tests first, then the applicable repository
  gates (`make test`, `make it`, `make lint`). The Makefile supplies SQLite
  include paths and tags; preserve them in direct Go commands.
- GraphQL/schema changes: use `make generate-backend` and `make generate-ui`
  where applicable. Do not hand-edit generated code. Allocate the next
  migration from current source; check fresh databases and upgrades.
- UI changes: use the existing scripts under `ui/v2.5` for tests, lint,
  TypeScript and formatting (`make validate-ui`). Verify production bundling
  with `make ui-only` when appropriate. Keep the locked package manager and
  dependency versions; avoid unrelated upgrades.
- Built-in UI changes: run `pnpm run prepare-builtins` in `ui/v2.5`, update the
  expected composed checksum when source changes, and check the generated
  `public/builtin/unifiedMedia.js` syntax. Edit `builtin-source`, not generated
  output. Production builds also compose the built-ins.
- Layout/player fixes need mounted-browser checks on desktop and mobile.
  Existing suites are in `ui/v2.5/tests/browser`; their documented fixture and
  environment requirements are in `docs/p07-global-media.md`. Check refreshes,
  repeated opens, selection, navigation and cleanup where relevant.
- Keep synthetic fixture results separate from real-media playback, codec
  checks, physical GPU inference and production behavior. State what ran and
  what remains unverified. Once relevant checks pass, rerun only for new changes
  or unresolved failures; do not repeatedly rebuild unchanged code.

## Upstream synchronization

- Upstream is `stashapp/stash`, normally its `develop` branch. Fetch fresh refs
  and compare exact SHAs before reporting whether it can merge.
- Use `git merge-tree --write-tree` or an isolated temporary merge to check
  conflicts without changing the user's branch. Include pending fork PRs when
  they touch the incoming code. A clean text merge is not a runtime guarantee.
- Inspect incoming files for migrations, API/dependency changes and overlap
  with fork behavior. Validate the temporary result as appropriate. Report
  conflicts and compatibility gaps; never replace fork files wholesale merely
  to match upstream, and do not merge into `develop` without authorization.

## Reporting

- Give concise progress updates during sustained work. Finish the assigned
  scope and relevant verification before returning to the user.
- Lead the final response with the result and PR link, then checks and material
  limitations. Include runnable commands when the user needs to apply or push
  something themselves. Avoid requiring repeated explanations or approvals.
