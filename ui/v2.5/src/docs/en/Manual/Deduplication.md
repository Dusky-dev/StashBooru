# Deduplication

## Finding similar images

The image card's **Find similar** action opens a reference search with two modes:

- **Same image / variants (pHash)** is the default for new searches. It compares current primary-file perceptual hashes, filters by the distance slider (0–8), and sorts by distance with image ID as the tie-breaker. Start with a low distance for close copies and increase it for more variants. A distance of zero means matching perceptual hashes, not necessarily identical files or pixels. Crops, rotations, borders, and large edits may not match.
- **Related content (EVA02)** finds related visual subjects using image embeddings. Different pictures of the same character can rank highly. Database tags do not determine this ranking, and the pHash distance slider does not apply. Generate embeddings in **Settings > System > Visual Similarity** if needed.

pHash mode works without embeddings. Images missing a current primary-file pHash are excluded; if the reference is missing one, the search explains that hashes must be generated in **Tasks**, or you can switch to Related content. Preserved `source_phash` values and secondary-file hashes are not used as substitutes for the displayed image's current hash.

Filters apply to the results; the reference itself does not have to pass those filters and is excluded from the results. Changing mode or distance returns to page one while preserving the reference and other filters. Mode and distance are saved in URLs and saved filters. Existing reference links without an explicit mode retain the previous EVA02 behavior; switch the mode to use pHash. Reference-free similarity grouping and video searches retain their existing behavior.

Review matches before deleting anything. This search does not verify byte/pixel identity or automatically delete or stack files.

## Video duplicate checker

[The dupe checker](/sceneDuplicateChecker) searches your collection for scenes that are perceptually similar. This means that the files don't need to be identical, and will be identified even with different bitrates, resolutions, and intros/outros.

To achieve this stash needs to generate what's called a phash, or perceptual hash. Similar to sprite generation stash will generate a set of 25 images from fixed points in the scene. These images will be stitched together, and then hashed using the phash algorithm. The phash can then be used to find scenes that are the same or similar to others in the database. Phash generation can be run during scan, or as a separate task. 

> **⚠️ Note:** Generation can take a while due to the work involved with extracting screenshots.

The dupe checker can be run with four different levels of accuracy. `Exact` looks for scenes that have exactly the same phash. This is a fast and accurate operation that should not yield any false positives except in very rare cases. The other accuracy levels look for duplicate files within a set distance of each other. This means the scenes don't have exactly the same phash, but are very similar. `High` and `Medium` should still yield very good results with few or no false positives. `Low` is likely to produce some false positives, but might still be useful for finding dupes.

> **⚠️ Note:** To generate a pHash Stash requires an uncorrupted file. If any errors are encountered during sprite generation the pHash will not be generated. This is to prevent false positives.

## Selecting duplicates

The `Select Options…` dropdown provides shortcuts for bulk-selecting files across every duplicate group on the current page. Each option selects every file in a group *except* the one to keep, so the selection can be reviewed and then deleted:

- **Largest file** and **highest resolution** keep the largest file, or the file with the highest resolution.
- **Oldest / youngest** keep the file with the oldest or youngest modification time.
- **Preferred codec** keeps the file matching the codec chosen in the `Preferred Video Codec` dropdown, which lists the codecs detected among the current duplicate results. Groups that contain no file with the preferred codec are left untouched, so nothing is selected when there is no matching file to keep.

The `Only select if all codecs match in the duplicate group` checkbox is a safety option for the size, resolution and age selections: when enabled, groups whose files use different codecs are skipped, so you don't accidentally select a file that was encoded with a different codec. It does not apply to the preferred-codec selection, which is codec-aware by design.
