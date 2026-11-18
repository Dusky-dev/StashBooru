# Consolidation roadmap

Working list for the consolidation goal in `AGENTS.md`. One row is one PR:
area — what the user sees — where to start — how to prove it. Update rows in
place and move finished ones to **Done**; the progress ledger keeps the detail.

## Next (agent can finish alone, in order)

1. **Implied parents everywhere (owner, 2026-11-18).** Fixture: Vigilantes
   with parent My Hero Academia, as Tags and as Copyrights, one Image with only
   Vigilantes. Check detail, cards, lists and a My Hero Academia search, also
   after a restart. Cards and lists show calculated parents only on detail
   pages today — fix each gap, keeping parents calculated, not written.
2. **Danbooru-style "Implies".** One "Implies" field on the child Tag or
   Copyright, the reverse list read-only on the parent; the four Library
   switches and Association sources stay out of the main path.
3. **Settings → Tasks English text.** `CharacterDedupTask.tsx`,
   `MediaBackupTask.tsx` — move to `react-intl`; `make validate-ui`, search
   `tests` and `builtin-source` for matching selectors.
4. **Expected request errors.** Processing converter 503 with no converter;
   Video `thumbs.vtt` 404 before generation — page sweep with no failed requests.
5. **Back during slow loads elsewhere.** Images, All and Booru (Booru has its
   own scroll memory in `BooruBrowser.tsx`) — reproduce with held scripts first.
6. **Image/Video bulk save and partial failure.** Listed as remaining in the
   audit — verify against the LIB ledger entries before starting.

## Plan first (propose to the owner)

7. **Navbar width.** Controls overflow 120–220px at 1200–1366px —
   `MainNavbar.tsx`, `index.scss`; `checkNavbar` at 1200/1280/1366/1440.
8. **Shared list rules (E07).** Selection, Back and Escape across All, Images,
   Videos, Following and Booru — one reproduced defect per PR.
9. **DOM-moving built-ins (E09).** Move into the owning component only while
   fixing that workflow.

## Owner decisions

10. Add All to custom navbar menus saved before it existed?
11. Refresh lists when a scan ends? (Upstream does not.)
12. Any proposal to hide or remove a feature.
13. Usability pass on the owner's real library (E11/E12).

## Needs real hardware or accounts (never claim verified)

Real provider accounts and Following, models/GPU/remote workers, upgrades
beyond the pinned 99→100 path, physical mobile/accessibility and large
libraries. Work here is limited to labels, errors and empty states found
through code or fixtures.

## Done

- Copyright → Characters tab HTTP 400 (LIB-01, 2026-11-16).
- Fork labels (#376), toasts (#377), Metadata Health (#378), navbar fork links
  (#379), list scroll after a fast Back (#380), Identities keeps
  connected names together (downloads, Character merge, merge prompt; #381).
- Owner decisions 2026-11-18: profile panel renamed Identities (matches the
  page); Alias Collision Inspector and Review library inheritance removed.
