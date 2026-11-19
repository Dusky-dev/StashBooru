# StashBooru 1.4.0

Following grows up: more sites, Follow buttons where you already work, and
automatic checks. Also clearer Identities and Pawchive creator names. Includes
everything from 1.1.2, 1.1.3, 1.2.0 and 1.3.0.

## Included

- **Automatic follow checks (1.4.0).** In Following → Manage, choose to check
  follows every 1, 3, 6, 12 or 24 hours (off until you turn it on), or press
  **Check all now**. The navbar Following link and each follow show how many
  new posts are waiting. A site that asks to slow down waits for the next run;
  one failing follow does not stop the others.
- **Follow from anywhere (1.3.0).** **Follow** / **Following** on Identities
  rows, Artist/Character/Copyright profiles and the Booru browser tag menu.
  Each follow shows the record it is connected to and opens in the Booru
  browser.
- **More sites to follow (1.2.0).** Gelbooru, Rule34 and Paheal tags and
  Pawchive creators (`creator:service:id`), as well as Danbooru. Rule34 and
  Gelbooru need your account key in Source settings.
- **Pawchive creator names (1.1.3).** Pawchive and Kemono creators show their
  real name and can be found by name in Identities and Artist searches. Account
  chips open the creator in the Booru browser. Fixed a crash when ticking rows
  on the Identities page.
- **Clearer Identities page (1.1.2).** Rows show the site's own name, which
  site it is from and whether it is connected, with one Connect/Change button.

## Before updating

Back up the database, configuration and media using the documented
fork-compatible procedure. Download the binary for your platform; SHA-256
checksums are included. Updates are not installed automatically.

## Scope and limitations

No database migration since 1.1.0. Automatic checks store their setting in
`following-schedule.json` beside the configuration. Gelbooru and Rule34 feed
checks were tested with fixtures only (they need account credentials); Paheal,
Safebooru and a Pawchive creator were checked live. Kemono, Moebooru, e621,
Philomena and Pixiv follows remain unsupported. The limitations listed for 1.0
still apply.
