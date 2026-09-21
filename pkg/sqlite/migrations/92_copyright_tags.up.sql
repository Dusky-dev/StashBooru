CREATE TABLE `copyrights_tags` (
  `copyright_id` integer NOT NULL,
  `tag_id` integer NOT NULL,
  foreign key(`copyright_id`) references `copyrights`(`id`) on delete CASCADE,
  foreign key(`tag_id`) references `tags`(`id`) on delete CASCADE,
  PRIMARY KEY(`copyright_id`, `tag_id`)
);

CREATE INDEX `index_copyrights_tags_on_tag_id` on `copyrights_tags` (`tag_id`);
