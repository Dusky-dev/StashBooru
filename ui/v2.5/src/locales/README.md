Use `en-GB.json` by default. This should have _all_ message IDs in it. Only add to other json files if the value is different to `en-GB` since it will fall back to use it if the message ID is not found in the chosen language.

Try to keep message IDs in alphabetical order for ease of reference.

# Merging translations from Codeberg Weblate

1. (**first time only**) Add remote for the Codeberg Weblate repository:
```bash
git remote add weblate_codeberg https://translate.codeberg.org/git/stash/stash/
```
2. (optional) Lock the Weblate repository.
3. Fetch the Weblate repository:
```bash
git fetch weblate_codeberg develop
```
4. Create and/or checkout a branch to hold the Weblate translations:
```bash
git checkout -b codeberg_weblate
```
5. Reset the branch to the Weblate repository's `develop` branch:
```bash
git reset --hard weblate_codeberg/develop
```
6. Push the branch to your github account:
```bash
git push origin codeberg_weblate
```
7. Create a pull request to merge the Weblate translations into the main repository.

# StashBooru display names

`booru-terminology.json` supplies the basic English Artist/Character/Video and
Collection names. Fork-specific nested English labels (for example Video covers
and Artist hierarchy/view headings) live in `en-GB.json`; `en-US.json` remains a
sparse spelling override and falls back to those messages. Preserve message IDs
and native Studio/Performer/Scene API fields. Non-English messages and user custom
locales keep their existing precedence; changing English does not translate every
language. Avoid changing label text used as a DOM selector: native Artist hierarchy
fields now remain in their native editor instead of the built-in Advanced mover.
