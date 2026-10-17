# Metadata and tagging

This guide covers StashBooru's metadata additions and reviewed tagging workflows.
For browsing and processing tools, see the [feature guide](features.md).

## Native entities and relationships

| Entity | Behavior |
| --- | --- |
| Copyright | A series, franchise or source work with its own ID, artwork, aliases, details, favorites, profile Tags and Main/Sub hierarchy. Assign it directly to Images, Videos and Characters. |
| Character | A native Performer with its own profile, Copyright associations, optional base Character and optional linked disambiguation context. |
| Artist | A native Studio; Images and Videos support several Artists. The first Artist is mirrored to the legacy primary Studio where required. Existing Artist parent links remain available. |
| Tag | An ordinary descriptive/rating Tag with native hierarchy links; it does not become a Copyright because the names happen to match. |

Copyright Edit can assign Characters in bulk. Main/Sub selectors edit the native
Copyright relationships, with multiple parents supported and cycles rejected.
Main and Sub detail tabs appear for the corresponding relationships. Sub ordering
belongs to the parent/child relationship. **Hierarchy** is the Copyright
directory's default sort; flat name/sort-name, count and date sorts remain
available. The optional sort name changes alphabetical placement without changing
the displayed name.

Media filters can target one Copyright or include descendants. Counts de-duplicate
media that belongs through more than one branch. Image/Video details present
Copyrights in separate hierarchy branches with their shared path. Current UI uses
native Grid/List controls and Main/Sub navigation; there is no separate taxonomy
tree editor. The retained `structural_role` and `primary_copyright` API fields are
compatibility fields, not active category/grouping controls.

Character variants remain separate searchable Characters, each with its own ID,
aliases, artwork and media. Set the base or select child variants in the Character
editor. The parent and Variants cards link to the related records; variant media
is available through descendant-aware Character searches. Reparenting does not
merge profiles.

The Character **Disambiguation** selector supports Copyright, Artist or Custom.
A linked context follows the target's displayed name and links to its page;
Custom remains text. Two same-name Characters can have different contexts.
An Artist used as disambiguation does not automatically become the Artist of
every media item. Ambiguous name/alias matches require resolution instead of
silently picking the first record.

## Direct and calculated associations

In **Settings → Library**, four independent switches control Character, Artist,
Copyright and Tag ancestors. Direct selections remain editable native links.
Ancestors and profile Tags are calculated from the current relationships rather
than copied into the direct-link tables. Direct profile Tags still apply when
that entity's ancestor switch is off; Tag ancestor inclusion has its own switch.

For example, selecting a Character variant can supply its base Character and the
enabled profile Tags. Removing that variant removes only unsupported calculated
memberships. An explicitly selected base, or a Tag still supplied by another
profile, remains. Hierarchy/profile edits affect subsequent reads and supported
hierarchical searches without rewriting every media record.

Use **Association sources** on Image/Video details to inspect where a membership
came from. **Settings → Tasks → Review library inheritance** previews proposed
defaults across existing media, shows affected memberships and origins, and
rechecks the preview before applying the shared defaults. It activates a live
projection; it does not insert calculated parents as permanent direct links.
Restarting the server requires a new review. Exact-set/null and some maintenance
filters keep direct-link semantics; see the [full contract](p06-effective-associations.md).

## Image Tagging

Open **Image Tagging** from an Image's operations. Opening the dialog loads local
filename metadata only. Fetching online metadata, running inference and asking
for library suggestions are separate explicit actions.

| Source | What it supplies | Requirement |
| --- | --- | --- |
| Local | Parsed filename identities and provenance. | Enable filename metadata and configure its layout. No model required. |
| Booru | Metadata for a matching post MD5, with source/post link and categorized tags where supplied/resolved. | Network access to a supported provider. No model required. |
| Camie | Character, Artist, Copyright, general/meta predictions. | Explicitly installed Camie v2 ONNX model and metadata on the selected worker. |
| EVA02 | Additional tag predictions from the installed EVA02 tagging data. | Compatible model/tag data on the selected worker; independent threshold. |
| Library | Suggestions drawn from visually similar indexed local Images. | Existing visual index and local catalogue metadata. Suggestions need review. |

The displayed merge priority is **Local → Booru → Camie → EVA02 → Library**.
The current button says **Fetch from Danbooru**, but the lookup code supports
Danbooru, Gelbooru, Yande.re, Konachan and Safebooru in that priority order. The
returned provider badge/post link identifies the actual source. This action looks
up metadata for an existing local file; it is not a remote-post search or downloader.

Predictions retain their source badges even when several sources agree. Earlier
sources retain the displayed value/score for overlapping predictions. Local
filename Character/Artist/Copyright values are authoritative within their populated
identity categories; conflicting inferred identities are suppressed. Scores are
source-specific signals, not interchangeable probabilities of correctness.

Select what to apply, resolve ambiguous identities, then inspect the change plan:
reuse existing records, create missing records, unresolved review items and
suppressed items. Unresolved items block apply. The explicit Artist replacement
option is separate from add-only association changes. Calculated Tags are shown
with origins and remain inherited rather than being written as direct Tags.
Applying metadata preserves the media's native ID and file bytes.

Bulk Image model tagging is available through the Images **Generate** workflow
and runs as a job. It is not a batch version of every optional single-Image source;
inspect the chosen action and selected scope before starting it.

### Filename layout and MD5 provenance

Configure filename metadata in **Settings → Processing → Similarity and tagging**.
The default layout is:

```text
[%artist%](%copyright%).%character%_%md5%.%ext%
```

For example:

```text
[artist_a](series_one).character_a_0123456789abcdef0123456789abcdef.jpg
```

Supported identity tokens include `%artist%`, `%copyright%` and `%character%`;
`%md5%` and `%ext%` describe the naming layout. Copyright values separated by `+`
can supply several series; literal repeated/trailing plus signs such as `C++`
are preserved. A Character suffix such as `Name (Series)` can disambiguate the
native Character rather than collapsing it into an unrelated same-name record.

Booru lookup can use a filename MD5, the actual file MD5 or retained conversion
source identity. The response labels the hash source. A historical MD5 can find
the original post after conversion; it does not mean the current bytes are still
identical to the remote original. Source pHash also remains lineage evidence.

### Model setup

Camie requires both `camie-tagger-v2.onnx` and
`camie-tagger-v2-metadata.json`. The standard Docker local cache is
`/cache/visual-embeddings/camie/`; the status panel reports the paths actually used.
Remote paths belong to the worker machine. The defaults are Camie threshold
`0.492`, EVA02 threshold `0.35` and a per-category result limit of `50`; change
them deliberately for new analyses. These controls do not tune pHash distance.
Camie weights are not bundled or downloaded silently. See [worker setup](../scripts/visual_embedding_server.md).

## Video Tagging and native Auto Tag

**Video Tagging** loads Local metadata on opening. Booru lookup and frame analysis
are explicit actions, with **Local → Booru → Frames** priority. Frame analysis
samples timestamps and aggregates model results with frame support; it does not
prove that a Character appears throughout a Video. Single and batch review show
the intended native relationships before apply. Batch scope respects the selected
or filtered Videos. The same source-priority and ambiguity rules protect identity
assignments.

Native **Auto Tag** is a separate path/name matching task, without model inference.
In **Settings → Tasks → Advanced mode → Auto Tag**, Copyrights can be enabled for
all or selected native Copyrights. **Selective Auto Tag** restricts paths. Copyright
matching applies to Images/Videos; Galleries have no native Copyright relationship.
Existing saved options leave Copyright matching off until enabled. Character
canonical identities, disambiguation and aliases use strict matching; alias
collisions are not resolved by selecting the first candidate.

## Profile artwork and cleanup

**Set image → From gallery…** picks catalogue artwork using the current
Character/Artist/Tag/Copyright context, paged search and previews. The editor
offers crop proportions, rotation, drag panning and pointer-centred zoom before
applying the profile image. This edits profile artwork, not the source media file.
Where no explicit artwork exists, supported entity image routes can use a related
Image as a fallback; setting artwork gives a stable user-selected result.

**Settings → Tools** offers:

- **Metadata Health:** read-only findings for duplicate/same-name identities,
  legacy Copyright Tags, Artist-link drift, orphaned links and ambiguous legacy
  Tag migrations. A finding does not itself repair or merge data.
- **Alias Collision Inspector:** canonical/alias conflicts and an inspection of
  the ambiguous value and its native candidates.
- **Visual Similarity Triage:** EVA02 clusters and preferred-copy suggestions.
  Explicitly selecting targets can add missing Characters, Artists, Copyrights
  and Tags from the preferred Image while preserving existing target metadata.
  These are embedding clusters, not pHash groups or verified duplicate sets;
  connected membership does not establish equality between every pair.

**Settings → Tasks → Merge duplicate Characters** is the separate reviewed,
cancellable merge workflow. It requires identical direct Copyright sets, skips
ambiguous/context-free/variant/conflicting cases, and revalidates before applying.
See [Character cleanup rules](character-dedup.md). It does not turn the diagnostic
tools or similarity suggestions into automatic deletion.
