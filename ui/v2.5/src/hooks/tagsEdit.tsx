import * as GQL from "src/core/generated-graphql";
import { useTagCreate } from "src/core/StashService";
import { useEffect, useState } from "react";
import { Tag, TagSelect, TagSelectProps } from "src/components/Tags/TagSelect";
import { useToast } from "src/hooks/Toast";
import { useIntl } from "react-intl";
import { Badge, Button } from "react-bootstrap";
import { Icon } from "src/components/Shared/Icon";
import { faPlus } from "@fortawesome/free-solid-svg-icons";
import { CollapseButton } from "src/components/Shared/CollapseButton";

export function useTagsEdit(
  srcTags: Tag[] | undefined,
  setFieldValue: (ids: string[]) => void
) {
  const intl = useIntl();
  const Toast = useToast();
  const [createTag] = useTagCreate();
  const [tags, setTags] = useState<Tag[]>(srcTags ?? []);
  const [newTags, setNewTags] = useState<GQL.ScrapedTag[]>();

  function onSetTags(items: Tag[]) {
    setTags(items);
    setFieldValue(items.map((item) => item.id.toString()));
  }

  function resetTagsState() {
    setTags(srcTags ?? []);
    setNewTags(undefined);
  }

  useEffect(() => {
    setTags(srcTags ?? []);
  }, [srcTags]);

  async function createNewTag(toCreate: GQL.ScrapedTag) {
    const tagInput: GQL.TagCreateInput = { name: toCreate.name ?? "" };
    try {
      const result = await createTag({ variables: { input: tagInput } });

      if (!result.data?.tagCreate) {
        Toast.error(new Error("Failed to create tag"));
        return;
      }

      onSetTags(
        tags.concat([
          {
            id: result.data.tagCreate.id,
            name: toCreate.name ?? "",
            aliases: [],
            stash_ids: result.data.tagCreate.stash_ids,
            parents: [],
          },
        ])
      );

      const next = newTags!.concat();
      next.splice(next.indexOf(toCreate), 1);
      setNewTags(next);

      Toast.success(
        intl.formatMessage(
          { id: "toast.created_entity" },
          {
            entity: intl.formatMessage({ id: "tag" }).toLocaleLowerCase(),
            entity_name: toCreate.name,
          }
        )
      );
    } catch (e) {
      Toast.error(e);
    }
  }

  function updateTagsStateFromScraper(
    scrapedTags?: Pick<GQL.ScrapedTag, "name" | "stored_id">[]
  ) {
    if (!scrapedTags) return;

    const idTags = scrapedTags.filter(
      (tag) => tag.stored_id !== undefined && tag.stored_id !== null
    );
    setNewTags(scrapedTags.filter((tag) => !tag.stored_id));
    onSetTags(
      idTags.map((tag) => ({
        id: tag.stored_id!,
        name: tag.name ?? "",
        aliases: [],
        stash_ids: [],
        parents: [],
      }))
    );
  }

  function renderNewTags() {
    if (!newTags || newTags.length === 0) return;

    const content = (
      <>
        {newTags.map((tag) => (
          <Badge
            className="tag-item"
            variant="secondary"
            key={tag.name}
            onClick={() => createNewTag(tag)}
          >
            {tag.name}
            <Button className="minimal ml-2">
              <Icon className="fa-fw" icon={faPlus} />
            </Button>
          </Badge>
        ))}
      </>
    );

    if (newTags.length >= 10) {
      return (
        <CollapseButton text={`Missing (${newTags.length})`}>
          {content}
        </CollapseButton>
      );
    }
    return content;
  }

  function tagsControl(props?: TagSelectProps) {
    return (
      <>
        <TagSelect isMulti onSelect={onSetTags} values={tags} {...props} />
        {renderNewTags()}
      </>
    );
  }

  return {
    tags,
    onSetTags,
    resetTagsState,
    tagsControl,
    updateTagsStateFromScraper,
  };
}
