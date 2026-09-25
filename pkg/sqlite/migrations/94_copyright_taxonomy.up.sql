-- P04 taxonomy metadata is deliberately layered on top of the existing
-- multi-parent Copyright graph. The graph remains the source of truth while
-- these tables add presentation/order semantics without imposing a fixed
-- taxonomy depth.

CREATE TABLE `copyright_relation_order` (
    `parent_id` INTEGER NOT NULL,
    `child_id` INTEGER NOT NULL,
    `position` INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (`parent_id`, `child_id`),
    FOREIGN KEY (`parent_id`, `child_id`) REFERENCES `copyright_relations` (`parent_id`, `child_id`) ON DELETE CASCADE
);
CREATE INDEX `index_copyright_relation_order_parent_position`
    ON `copyright_relation_order` (`parent_id`, `position`, `child_id`);

CREATE TABLE `copyright_structural_roles` (
    `copyright_id` INTEGER PRIMARY KEY,
    `role` TEXT NOT NULL DEFAULT '',
    FOREIGN KEY (`copyright_id`) REFERENCES `copyrights` (`id`) ON DELETE CASCADE
);

CREATE TABLE `tag_structural_roles` (
    `tag_id` INTEGER PRIMARY KEY,
    `role` TEXT NOT NULL DEFAULT '',
    FOREIGN KEY (`tag_id`) REFERENCES `tags` (`id`) ON DELETE CASCADE
);

-- A primary Copyright is optional and must also be part of the media item's
-- normal Copyright association set. Composite foreign keys guarantee that
-- invariant and automatically clear a primary when the association is removed.
CREATE TABLE `image_primary_copyrights` (
    `image_id` INTEGER PRIMARY KEY,
    `copyright_id` INTEGER NOT NULL,
    FOREIGN KEY (`image_id`, `copyright_id`) REFERENCES `images_copyrights` (`image_id`, `copyright_id`) ON DELETE CASCADE
);
CREATE INDEX `index_image_primary_copyrights_copyright`
    ON `image_primary_copyrights` (`copyright_id`);

CREATE TABLE `scene_primary_copyrights` (
    `scene_id` INTEGER PRIMARY KEY,
    `copyright_id` INTEGER NOT NULL,
    FOREIGN KEY (`scene_id`, `copyright_id`) REFERENCES `scenes_copyrights` (`scene_id`, `copyright_id`) ON DELETE CASCADE
);
CREATE INDEX `index_scene_primary_copyrights_copyright`
    ON `scene_primary_copyrights` (`copyright_id`);
