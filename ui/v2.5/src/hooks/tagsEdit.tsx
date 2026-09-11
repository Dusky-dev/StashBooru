import * as GQL from "src/core/generated-graphql";
import { useTagCreate } from "src/core/StashService";
import { useEffect, useState } from "react";
import {
  CopyrightSelect,
  Tag,
  TagSelect,
  TagSelectProps,
} from "src/components/Tags/TagSelect";
import { isCopyrightTag } from "src/components/Tags/copyrightFilter";
import { useToast } from "src/hooks/Toast";
import { useIntl } from "react-intl";
import { Badge, Button } from "react-bootstrap";
import { Icon } from "src/components/Shared/Icon";
import { faPlus } from "@fortawesome/free-solid-svg-icons";
import { CollapseButton } from "src/components/Shared/CollapseButton";

function splitTags(srcTags: Tag[] | undefined) {
  const tags: Tag[] = [];
  const copyrights: Tag[] = [];
  for (const tag of srcTags ?? []) {
    (isCopyrightTag(tag) ? copyrights : tags).push(tag);
  }
  return { tags, copyrights };
}

export function useTagsEdit(
  srcTags: Tag[] | undefined,
  setFieldValue: (ids: string[]) => void
) {
  const intl = useIntl();
  const Toast = useToast();
  const [createTag] = useTagCreate();

  const initial = splitTags(srcTags);
  const [tags, setTags] = useState<Tag[]>(initial.tags);
  const [copyrights, setCopyrights] = useState<Tag[]>(initial.copyrights);
  const [newTags, setNewTags] = useState<GQL.ScrapedTag[]>();

  function publish(nextTags: Tag[], nextCopyrights: Tag[]) {
    setFieldValue(
      [...nextTags, ...nextCopyrights].map((item) => item.id.toString())
    );
  }

  function onSetTags(items: Tag[]) {
    setTags(items);
    publish(items, copyrights);
  }

  function onSetCopyrights(items: Tag[]) {
    setCopyrights(items);
    publish(tags, items);
  }

  function resetTagsState() {
    const next = splitTags(srcTags);
    setTags(next.tags);
    setCopyrights(next.copyrights);
    setNewTags(undefined);
  }

  useEffect(() => {
    const next = splitTags(srcTags);
    setTags(next.tags);
    setCopyrights(next.copyrights);
  }, [srcTags]);

  async function createNewTag(toCreate: GQL.ScrapedTag) {
    const tagInput: GQL.TagCreateInput = { name: toCreate.name ?? "" };
    try {
      const result = await createTag({
        variables: {
          input: tagInput,
        },
      });

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

      const newTagsClone = newTags!.concat();
      const pIndex = newTagsClone.indexOf(toCreate);
      newTagsClone.splice(pIndex, 1);

      setNewTags(newTagsClone);

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
    if (!scrapedTags) {
      return;
    }

    const idTags = scrapedTags.filter(
      (t) => t.stored_id !== undefined && t.stored_id !== null
    );
    const newNewTags = scrapedTags.filter((t) => !t.stored_id);
    onSetTags(
      idTags.map((p) => {
        return {
          id: p.stored_id!,
          name: p.name ?? "",
          aliases: [],
          stash_ids: [],
          parents: [],
        };
      })
    );

    setNewTags(newNewTags);
  }

  function renderNewTags() {
    if (!newTags || newTags.length === 0) {
      return;
    }

    const ret = (
      <>
        {newTags.map((t) => (
          <Badge
            className="tag-item"
            variant="secondary"
            key={t.name}
            onClick={() => createNewTag(t)}
          >
            {t.name}
            <Button className="minimal ml-2">
              <Icon className="fa-fw" icon={faPlus} />
            </Button>
          </Badge>
        ))}
      </>
    );

    const minCollapseLength = 10;

    if (newTags.length >= minCollapseLength) {
      return (
        <CollapseButton text={`Missing (${newTags.length})`}>
          {ret}
        </CollapseButton>
      );
    }

    return ret;
  }

  function tagsControl(props?: TagSelectProps) {
    return (
      <>
        <TagSelect isMulti onSelect={onSetTags} values={tags} {...props} />
        {renderNewTags()}
      </>
    );
  }

  function copyrightsControl(props?: TagSelectProps) {
    return (
      <CopyrightSelect
        isMulti
        onSelect={onSetCopyrights}
        values={copyrights}
        {...props}
      />
    );
  }

  return {
    tags,
    copyrights,
    onSetTags,
    onSetCopyrights,
    resetTagsState,
    tagsControl,
    copyrightsControl,
    updateTagsStateFromScraper,
  };
}
