# Consolidation roadmap

Working list for the consolidation goal in `AGENTS.md`. One row is one PR:
area — what the user sees — where to start — how to prove it. Update rows in
place and move finished ones to **Done**; the progress ledger keeps the detail.

## Next (agent can finish alone, in order)

1. **Settings consistency (owner, 2026-11-18).** The owner means how panels
   look, plus the measured layout fixes. Experimental behind Advanced mode:
   next PR. Then Processing on shared `SettingSection`/`Inputs` rows with one
   worker notice (before/after shots first), then Interface (6.7k px, 25 flat
   groups) and System (3.8k px) as titled disclosures like Tasks.
2. **Settings → Tasks English text.** `CharacterDedupTask.tsx`,
   `MediaBackupTask.tsx` — move to `react-intl`; `make validate-ui`, search
   `tests` and `builtin-source` for matching selectors.
3. **Expected request errors.** Processing converter 503 with no converter;
   Video `thumbs.vtt` 404 before generation — page sweep with no failed requests.
4. **Back during slow loads elsewhere.** Images, All and Booru (Booru has its
   own scroll memory in `BooruBrowser.tsx`) — reproduce with held scripts first.
   The Video list scroll-restore check in `first-library` failed once on #385
   and passed on re-run (intermittent).
5. **Image/Video bulk save and partial failure.** Listed as remaining in the
   audit — verify against the LIB ledger entries before starting.
6. **Finite-loop GIF → animated JPEG XL** fails the worker's loop-count check
   (source kept) with cjxl 0.7. Reproduce on libjxl 0.11 before changing.

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
14. Show a Tag button (count 0) when an item has only profile-implied
    Tags? (Codex P2 on #383, deferred.)
15. Animated JPEG XL plays choppily in Firefox; the file and a WebP copy play
    fine. AVIF and video inputs lost their frame timing (every frame 33 ms);
    fixed in #388, but existing files keep it until reconverted.
    Choppiness from GIF, APNG or WebP sources is undetermined: needs
    `jxlinfo -v` of one file, its source format and the worker's cjxl version.

## Needs real hardware or accounts (never claim verified)

Real provider accounts and Following, models/GPU/remote workers, upgrades
beyond the pinned 99→100 path, physical mobile/accessibility and large
libraries. Work here is limited to labels, errors and empty states found
through code or fixtures.

## Done

- Native app for Android, Linux and Windows in `app/` (owner request,
  2026-11-20, 1.5.0). Read-only v1. Follow-ups to propose: editing,
  release keystore for signed APKs, attaching app builds to releases.
- Copyright → Characters tab HTTP 400 (LIB-01, 2026-11-16).
- Fork labels (#376), toasts (#377), Metadata Health (#378), navbar fork links
  (#379), list scroll after a fast Back (#380), Identities keeps
  connected names together (downloads, Character merge, merge prompt; #381).
- Identities panel name; alias inspector and inheritance review removed (#382).
- Card popovers keep detail-page associations (#384).
- Auto JPEG XL effort on converter rules and one-off settings (#385).
- Identities: add the profile's own name; display names beside keys (#386).
- Experimental settings show only in Advanced mode (#387).
- Animated JPEG XL from AVIF and video inputs keeps per-frame timing (#388).
- Settings: Tools is a Tasks group; Logs and Changelog are part of About (#389).
- Processing: compact Camie/PixAI rows with manual help icon (#390).
- Copyright pages list Main above Sub instead of tabs (#391).
- Interface and System settings fold into remembered groups (#392).
- Settings help icons, translated search labels, remembered Tasks groups
  (#393).
- One JPEG XL and one PNG converter choice, still or animated (#394).
- Artist/Copyright merge dialog like Characters/Tags; merged Tags reused by
  downloads ([#395](https://github.com/Dusky-dev/StashBooru/pull/395)).
- Identities page: site names as row headings, one Connect/Change action,
  scan section with plain status (1.1.2,
  [#405](https://github.com/Dusky-dev/StashBooru/pull/405)).
- Pawchive/Kemono creator names on identities; account chips open the
  Booru browser; Identities checkbox crash (1.1.3).
- Following checks Gelbooru, Rule34, Paheal and Pawchive creators (1.2.0).
- Follow / Following on Identities, profiles and the Booru browser tag menu;
  follows show their connected record (1.3.0).
- Automatic follow checks with new-post counts in the navbar (1.4.0).
- **Followed posts** tab on Artists, Characters and Copyrights with per-site
  icons; post selection, Mark all seen and an optional mark-seen-once-shown
  setting (also in Booru settings); Following opens on follows & settings
  (1.6.0).
- Main page shows current names; Edit main page at the top (1.6.1, #416).
- Image/Video Date from EXIF capture date, else site post date, on download
  and via Tasks → Fill missing dates; "On this day" main-page row using
  Date, else date added (1.7.0).
- Parent Tags/Copyrights apply automatically (calculated; "Include parent
  Tags" is on by default). No separate Implies field (owner, 2026-11-18).
- Implied Tags and Copyrights on Image and Video cards (#383); detail pages,
  Tag search and Copyright filters already included parents.
