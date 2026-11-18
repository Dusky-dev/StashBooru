# Consolidation roadmap

Working list for the consolidation goal in `AGENTS.md`. One row is one PR:
area — what the user sees — where to start — how to prove it. Update rows in
place and move finished ones to **Done**; the progress ledger keeps the detail.

## Next (agent can finish alone, in order)

1. **Danbooru-style "Implies" (waits for owner go-ahead).** One "Implies"
   field on the child Tag or Copyright, the reverse list read-only on the
   parent; the four Library switches and Association sources stay out of the
   main path.
2. **Converter: Auto JPEG XL effort (owner, 2026-11-18).** Auto picks a
   concrete effort per file on the server; workers still receive 1–9.
3. **Identities: add the profile's own name (owner, 2026-11-18).** The
   add form offers the Artist's name; lists show display names beside keys.
4. **Settings → Tasks English text.** `CharacterDedupTask.tsx`,
   `MediaBackupTask.tsx` — move to `react-intl`; `make validate-ui`, search
   `tests` and `builtin-source` for matching selectors.
5. **Expected request errors.** Processing converter 503 with no converter;
   Video `thumbs.vtt` 404 before generation — page sweep with no failed requests.
6. **Back during slow loads elsewhere.** Images, All and Booru (Booru has its
   own scroll memory in `BooruBrowser.tsx`) — reproduce with held scripts first.
7. **Image/Video bulk save and partial failure.** Listed as remaining in the
   audit — verify against the LIB ledger entries before starting.

## Plan first (propose to the owner)

8. **Navbar width.** Controls overflow 120–220px at 1200–1366px —
   `MainNavbar.tsx`, `index.scss`; `checkNavbar` at 1200/1280/1366/1440.
9. **Shared list rules (E07).** Selection, Back and Escape across All, Images,
   Videos, Following and Booru — one reproduced defect per PR.
10. **DOM-moving built-ins (E09).** Move into the owning component only while
   fixing that workflow.

## Owner decisions

11. Add All to custom navbar menus saved before it existed?
12. Refresh lists when a scan ends? (Upstream does not.)
13. Any proposal to hide or remove a feature.
14. Usability pass on the owner's real library (E11/E12).
15. Show a Tag button (count 0) when an item has only profile-implied
    Tags? (Codex P2 on #383, deferred.)
16. Animated JPEG XL plays choppily in the browser (file and WebP fine).
    Images load the original, so this is browser decode cost. Raise the AJXL
    decode-speed default, or keep animations as WebP?

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
- Identities panel name; alias inspector and inheritance review removed (#382).
- Implied Tags and Copyrights on Image and Video cards (#383); detail pages,
  Tag search and Copyright filters already included parents.
