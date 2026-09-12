CREATE TABLE `copyrights` (
    `id` INTEGER PRIMARY KEY AUTOINCREMENT,
    `name` VARCHAR(255) NOT NULL COLLATE NOCASE UNIQUE,
    `sort_name` VARCHAR(255) NOT NULL DEFAULT '',
    `description` TEXT NOT NULL DEFAULT '',
    `favorite` BOOLEAN NOT NULL DEFAULT 0,
    `created_at` DATETIME NOT NULL,
    `updated_at` DATETIME NOT NULL
);

CREATE TABLE `copyright_aliases` (
    `copyright_id` INTEGER NOT NULL,
    `alias` VARCHAR(255) NOT NULL COLLATE NOCASE,
    PRIMARY KEY (`copyright_id`, `alias`),
    FOREIGN KEY (`copyright_id`) REFERENCES `copyrights` (`id`) ON DELETE CASCADE
);
CREATE INDEX `index_copyright_aliases_alias` ON `copyright_aliases` (`alias` COLLATE NOCASE);

CREATE TABLE `copyright_relations` (
    `parent_id` INTEGER NOT NULL,
    `child_id` INTEGER NOT NULL,
    PRIMARY KEY (`parent_id`, `child_id`),
    CHECK (`parent_id` != `child_id`),
    FOREIGN KEY (`parent_id`) REFERENCES `copyrights` (`id`) ON DELETE CASCADE,
    FOREIGN KEY (`child_id`) REFERENCES `copyrights` (`id`) ON DELETE CASCADE
);
CREATE INDEX `index_copyright_relations_child` ON `copyright_relations` (`child_id`);

CREATE TABLE `images_copyrights` (
    `image_id` INTEGER NOT NULL,
    `copyright_id` INTEGER NOT NULL,
    PRIMARY KEY (`image_id`, `copyright_id`),
    FOREIGN KEY (`image_id`) REFERENCES `images` (`id`) ON DELETE CASCADE,
    FOREIGN KEY (`copyright_id`) REFERENCES `copyrights` (`id`) ON DELETE CASCADE
);
CREATE INDEX `index_images_copyrights_copyright` ON `images_copyrights` (`copyright_id`);

CREATE TABLE `scenes_copyrights` (
    `scene_id` INTEGER NOT NULL,
    `copyright_id` INTEGER NOT NULL,
    PRIMARY KEY (`scene_id`, `copyright_id`),
    FOREIGN KEY (`scene_id`) REFERENCES `scenes` (`id`) ON DELETE CASCADE,
    FOREIGN KEY (`copyright_id`) REFERENCES `copyrights` (`id`) ON DELETE CASCADE
);
CREATE INDEX `index_scenes_copyrights_copyright` ON `scenes_copyrights` (`copyright_id`);

-- Images historically had a single studio_id. Keep that column as the primary
-- Artist for backwards compatibility and store the ordered multi-Artist set
-- here. Existing images need no data migration: reads include studio_id and the
-- first Artist written here is mirrored back to studio_id.
CREATE TABLE `images_artists` (
    `image_id` INTEGER NOT NULL,
    `studio_id` INTEGER NOT NULL,
    `position` INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (`image_id`, `studio_id`),
    FOREIGN KEY (`image_id`) REFERENCES `images` (`id`) ON DELETE CASCADE,
    FOREIGN KEY (`studio_id`) REFERENCES `studios` (`id`) ON DELETE CASCADE
);
CREATE INDEX `index_images_artists_studio` ON `images_artists` (`studio_id`);
