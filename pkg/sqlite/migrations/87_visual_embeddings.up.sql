CREATE TABLE `visual_embedding_sources` (
    `entity_type` TEXT NOT NULL,
    `entity_id` INTEGER NOT NULL,
    `model` TEXT NOT NULL,
    `source_key` TEXT NOT NULL DEFAULT '',
    `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (`entity_type`, `entity_id`)
);

CREATE VIRTUAL TABLE `image_embedding_vectors` USING vec0(
    embedding FLOAT[1024] DISTANCE_METRIC=cosine
);

CREATE VIRTUAL TABLE `scene_embedding_vectors` USING vec0(
    embedding FLOAT[1024] DISTANCE_METRIC=cosine
);
