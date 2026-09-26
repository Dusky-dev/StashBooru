-- P05 keeps Characters as native Performer rows. Each variant has one
-- optional base Character and one optional typed disambiguation target.
ALTER TABLE `performers`
  ADD COLUMN `parent_performer_id` INTEGER
  REFERENCES `performers` (`id`) ON DELETE SET NULL
  CHECK (`parent_performer_id` IS NULL OR `parent_performer_id` != `id`);

ALTER TABLE `performers`
  ADD COLUMN `disambiguation_copyright_id` INTEGER
  REFERENCES `copyrights` (`id`) ON DELETE SET NULL;

ALTER TABLE `performers`
  ADD COLUMN `disambiguation_studio_id` INTEGER
  REFERENCES `studios` (`id`) ON DELETE SET NULL
  CHECK (`disambiguation_copyright_id` IS NULL OR `disambiguation_studio_id` IS NULL);

CREATE INDEX `index_performers_parent_performer_id`
  ON `performers` (`parent_performer_id`);
CREATE INDEX `index_performers_disambiguation_copyright_id`
  ON `performers` (`disambiguation_copyright_id`);
CREATE INDEX `index_performers_disambiguation_studio_id`
  ON `performers` (`disambiguation_studio_id`);
