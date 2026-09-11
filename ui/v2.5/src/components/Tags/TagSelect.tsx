import React, { useEffect, useMemo, useState } from "react";
import {
  OptionProps,
  components as reactSelectComponents,
  MultiValueGenericProps,
  SingleValueProps,
} from "react-select";
import cx from "classnames";

import * as GQL from "src/core/generated-graphql";
import {
  useTagCreate,
  queryFindTagsByIDForSelect,
  queryFindTagsForSelect,
} from "src/core/StashService";
import { useConfigurationContext } from "src/hooks/Config";
import { useIntl } from "react-intl";
import { defaultMaxOptionsShown } from "src/core/config";
import { ListFilterModel } from "src/models/list-filter/filter";
import {
  FilterSelectComponent,
  IFilterIDProps,
  IFilterProps,
  IFilterValueProps,
  Option as SelectOption,
  toOption,
} from "../Shared/FilterSelect";
import { useCompare } from "src/hooks/state";
import { TagPopover } from "./TagPopover";
import { Placement } from "react-bootstrap/esm/Overlay";
import { sortByRelevance } from "src/utils/query";
import { PatchComponent, PatchFunction } from "src/patch";
import { isUUID } from "src/utils/stashIds";
import { filterByStashID } from "src/models/list-filter/utils";

export type SelectObject = {
  id: string;
  name?: string | null;
  title?: string | null;
};

export type Tag = Pick<
  GQL.Tag,
  "id" | "name" | "sort_name" | "aliases" | "image_path" | "stash_ids"
> & {
  parents?: Array<Pick<GQL.Tag, "id" | "name" | "sort_name">> | null;
};
type Option = SelectOption<Tag>;

type FindTagsResult = Awaited<
  ReturnType<typeof queryFindTagsForSelect>
>["data"]["findTags"]["tags"];

function sortTagsByRelevance(input: string, tags: FindTagsResult) {
  return sortByRelevance(
    input,
    tags,
    (t) => t.name,
    (t) => t.aliases
  );
}

const tagSelectSort = PatchFunction("TagSelect.sort", sortTagsByRelevance);

export type TagSelectProps = IFilterProps &
  IFilterValueProps<Tag> & {
    hoverPlacement?: Placement;
    hoverPlacementLabel?: Placement;
    excludeIds?: string[];
  };

const _TagSelect: React.FC<TagSelectProps> = (props) => {
  const [createTag] = useTagCreate();

  const { configuration } = useConfigurationContext();
  const intl = useIntl();
  const maxOptionsShown =
    configuration?.ui.maxOptionsShown ?? defaultMaxOptionsShown;
  const defaultCreatable = !configuration?.interface.disableDropdownCreate.tag;

  const exclude = useMemo(() => props.excludeIds ?? [], [props.excludeIds]);

  function filterResult(tag: Tag) {
    return !exclude.includes(tag.id.toString());
  }

  function makeFilter() {
    const filter = new ListFilterModel(GQL.FilterMode.Tags);
    filter.currentPage = 1;
    filter.itemsPerPage = maxOptionsShown;
    filter.sortBy = "name";
    filter.sortDirection = GQL.SortDirectionEnum.Asc;
    return filter;
  }

  async function loadTags(input: string): Promise<Option[]> {
    if (isUUID(input)) {
      const filter = makeFilter();
      filterByStashID(filter, input);

      const query = await queryFindTagsForSelect(filter);
      const matches = query.data.findTags.tags.filter(filterResult);

      if (matches.length > 0) {
        return matches.map(toOption);
      }
    }

    const filter = makeFilter();
    filter.searchTerm = input;

    const query = await queryFindTagsForSelect(filter);
    const ret = query.data.findTags.tags.filter(filterResult);

    return tagSelectSort(input, ret).map(toOption);
  }

  const TagOption: React.FC<OptionProps<Option, boolean>> = (optionProps) => {
    let thisOptionProps = optionProps;
    const { object } = optionProps.data;
    const { name } = object;

    const { inputValue } = optionProps.selectProps;
    let alias: string | undefined = "";
    if (!name.toLowerCase().includes(inputValue.toLowerCase())) {
      alias = object.aliases?.find((a) =>
        a.toLowerCase().includes(inputValue.toLowerCase())
      );
    }

    thisOptionProps = {
      ...optionProps,
      children: (
        <TagPopover id={object.id} placement={props.hoverPlacement ?? "right"}>
          <span className="react-select-image-option">
            <span>{name}</span>
            {alias && <span className="alias">&nbsp;({alias})</span>}
          </span>
        </TagPopover>
      ),
    };

    return <reactSelectComponents.Option {...thisOptionProps} />;
  };

  const TagMultiValueLabel: React.FC<
    MultiValueGenericProps<Option, boolean>
  > = (optionProps) => {
    let thisOptionProps = optionProps;
    const { object } = optionProps.data;

    thisOptionProps = {
      ...optionProps,
      children: (
        <TagPopover
          id={object.id}
          placement={props.hoverPlacementLabel ?? "top"}
        >
          <span>{object.name}</span>
        </TagPopover>
      ),
    };

    return <reactSelectComponents.MultiValueLabel {...thisOptionProps} />;
  };

  const TagValueLabel: React.FC<SingleValueProps<Option, boolean>> = (
    optionProps
  ) => {
    let thisOptionProps = optionProps;
    const { object } = optionProps.data;

    thisOptionProps = {
      ...optionProps,
      children: <>{object.name}</>,
    };

    return <reactSelectComponents.SingleValue {...thisOptionProps} />;
  };

  const onCreate = async (name: string) => {
    const result = await createTag({ variables: { input: { name } } });
    return {
      value: result.data!.tagCreate!.id,
      item: result.data!.tagCreate!,
      message: "Created tag",
    };
  };

  const getNamedObject = (id: string, name: string) => ({
    id,
    name,
    aliases: [],
    stash_ids: [],
    parents: [],
  });

  const isValidNewOption = (inputValue: string, options: Tag[]) => {
    if (!inputValue) return false;
    return !options.some(
      (o) =>
        o.name.toLowerCase() === inputValue.toLowerCase() ||
        o.aliases?.some((a) => a.toLowerCase() === inputValue.toLowerCase())
    );
  };

  return (
    <FilterSelectComponent<Tag, boolean>
      {...props}
      className={cx(
        "tag-select",
        { "tag-select-active": props.active },
        props.className
      )}
      loadOptions={loadTags}
      getNamedObject={getNamedObject}
      isValidNewOption={isValidNewOption}
      components={{
        Option: TagOption,
        MultiValueLabel: TagMultiValueLabel,
        SingleValue: TagValueLabel,
      }}
      isMulti={props.isMulti ?? false}
      creatable={props.creatable ?? defaultCreatable}
      onCreate={onCreate}
      placeholder={
        props.noSelectionString ??
        intl.formatMessage(
          { id: "actions.select_entity" },
          {
            entityType: intl.formatMessage({
              id: props.isMulti ? "tags" : "tag",
            }),
          }
        )
      }
      closeMenuOnSelect={!props.isMulti}
    />
  );
};

export const TagSelect = PatchComponent("TagSelect", _TagSelect);

const _TagIDSelect: React.FC<IFilterProps & IFilterIDProps<Tag>> = (props) => {
  const { ids, onSelect: onSelectValues } = props;
  const [values, setValues] = useState<Tag[]>([]);
  const idsChanged = useCompare(ids);

  function onSelect(items: Tag[]) {
    setValues(items);
    onSelectValues?.(items);
  }

  useEffect(() => {
    if (!idsChanged) return;
    if (!ids || ids.length === 0) {
      setValues([]);
      return;
    }

    const filteredValues = values.filter((v) => ids.includes(v.id.toString()));
    if (filteredValues.length === ids.length) return;

    const load = async () => {
      const query = await queryFindTagsByIDForSelect(ids);
      const sortedItems = [...query.data.findTags.tags];
      sortedItems.sort((a, b) => {
        const aName = a.sort_name || a.name;
        const bName = b.sort_name || b.name;
        return aName && bName ? aName.localeCompare(bName) : 0;
      });
      setValues(sortedItems);
    };

    void load();
  }, [ids, idsChanged, values]);

  return <TagSelect {...props} values={values} onSelect={onSelect} />;
};

export const TagIDSelect = PatchComponent("TagIDSelect", _TagIDSelect);
