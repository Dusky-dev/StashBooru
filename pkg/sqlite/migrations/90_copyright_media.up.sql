ALTER TABLE `copyrights` ADD COLUMN `image_blob` VARCHAR(32) REFERENCES `blobs` (`checksum`);

CREATE TABLE `performers_copyrights` (
    `performer_id` INTEGER NOT NULL,
    `copyright_id` INTEGER NOT NULL,
    PRIMARY KEY (`performer_id`, `copyright_id`),
    FOREIGN KEY (`performer_id`) REFERENCES `performers` (`id`) ON DELETE CASCADE,
    FOREIGN KEY (`copyright_id`) REFERENCES `copyrights` (`id`) ON DELETE CASCADE
);
CREATE INDEX `index_performers_copyrights_copyright` ON `performers_copyrights` (`copyright_id`);
