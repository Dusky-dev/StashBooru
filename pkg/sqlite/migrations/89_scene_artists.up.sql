-- Videos historically had a single studio_id. Keep that column as the primary
-- Artist for backwards compatibility and store the ordered multi-Artist set
-- here. Existing videos need no data migration: reads include studio_id and the
-- first Artist written here is mirrored back to studio_id.
CREATE TABLE `scenes_artists` (
    `scene_id` INTEGER NOT NULL,
    `studio_id` INTEGER NOT NULL,
    `position` INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (`scene_id`, `studio_id`),
    FOREIGN KEY (`scene_id`) REFERENCES `scenes` (`id`) ON DELETE CASCADE,
    FOREIGN KEY (`studio_id`) REFERENCES `studios` (`id`) ON DELETE CASCADE
);
CREATE INDEX `index_scenes_artists_studio` ON `scenes_artists` (`studio_id`);
