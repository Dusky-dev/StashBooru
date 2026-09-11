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
import {
  applyCopyrightNamespaceFilter,
  fetchCopyrightRoot,
  isCopyrightTag,
} from "./copyrightFilter";

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

export type TagNamespace = "tags" | "copyrights" | "all";

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
    namespace?: TagNamespace;
  };

function matchesNamespace(tag: Tag, namespace: TagNamespace) {
  if (namespace === "all") return true;
  const copyright = isCopyrightTag(tag);
  return namespace === "copyrights" ? copyright : !copyright;
}

const _TagSelect: React.FC<TagSelectProps> = (props) => {
  const [createTag] = useTagCreate();

  const { configuration } = useConfigurationContext();
  const intl = useIntl();
  const maxOptionsShown =
    configuration?.ui.maxOptionsShown ?? defaultMaxOptionsShown;
  const defaultCreatable = !configuration?.interface.disableDropdownCreate.tag;
  const namespace = props.namespace ?? "tags";

  const exclude = useMemo(() => props.excludeIds ?? [], [props.excludeIds]);
  const visibleValues = useMemo(
    () => props.values?.filter((tag) => matchesNamespace(tag, namespace)),
    [namespace, props.values]
  );

  function filterResult(tag: Tag) {
    return (
      !exclude.includes(tag.id.toString()) && matchesNamespace(tag, namespace)
    );
  }

  async function makeFilter() {
    let filter = new ListFilterModel(GQL.FilterMode.Tags);
    filter.currentPage = 1;
    filter.itemsPerPage = maxOptionsShown;
    filter.sortBy = "name";
    filter.sortDirection = GQL.SortDirectionEnum.Asc;

    if (namespace !== "all") {
      const root = await fetchCopyrightRoot();
      if (root) {
        filter = applyCopyrightNamespaceFilter(
          filter,
          root,
          namespace === "copyrights"
        );
      }
    }

    return filter;
  }

  async function loadTags(input: string): Promise<Option[]> {
    if (isUUID(input)) {
      const filter = await makeFilter();
      filterByStashID(filter, input);

      const query = await queryFindTagsForSelect(filter);
      const matches = query.data.findTags.tags.filter(filterResult);

      if (matches.length > 0) {
        return matches.map(toOption);
      }
    }

    const filter = await makeFilter();
    filter.searchTerm = input;

    const query = await queryFindTagsForSelect(filter);
    const ret = query.data.findTags.tags.filter(filterResult);

    return tagSelectSort(input, ret).map(toOption);
  }

  const TagOption: React.FC<OptionProps<Option, boolean>> = (optionProps) => {
    let thisOptionProps = optionProps;

    const { object } = optionProps.data;

    const { name } = object;

    if (!matchesNamespace(object, namespace)) return null;

    // if name does not match the input value but an alias does, show the alias
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
    const root = namespace === "copyrights" ? await fetchCopyrightRoot() : null;
    if (namespace === "copyrights" && !root) {
      throw new Error("Unable to initialize the Copyright namespace");
    }

    const result = await createTag({
      variables: {
        input: {
          name,
          ...(root ? { parent_ids: [String(root.id)] } : {}),
        },
      },
    });
    return {
      value: result.data!.tagCreate!.id,
      item: result.data!.tagCreate!,
      message:
        namespace === "copyrights" ? "Created copyright" : "Created tag",
    };
  };

  const getNamedObject = (id: string, name: string) => {
    return {
      id,
      name,
      aliases: [],
      stash_ids: [],
      parents: [],
    };
  };

  const isValidNewOption = (inputValue: string, options: Tag[]) => {
    if (!inputValue) {
      return false;
    }

    if (
      options.some((o) => {
        return (
          o.name.toLowerCase() === inputValue.toLowerCase() ||
          o.aliases?.some((a) => a.toLowerCase() === inputValue.toLowerCase())
        );
      })
    ) {
      return false;
    }

    return true;
  };

  const entityLabel =
    namespace === "copyrights"
      ? props.isMulti
        ? "Copyrights"
        : "Copyright"
      : intl.formatMessage({ id: props.isMulti ? "tags" : "tag" });

  return (
    <FilterSelectComponent<Tag, boolean>
      {...props}
      values={visibleValues}
      className={cx(
        "tag-select",
        {
          "tag-select-active": props.active,
        },
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
          { entityType: entityLabel }
        )
      }
      closeMenuOnSelect={!props.isMulti}
    />
  );
};

export const TagSelect = PatchComponent("TagSelect", _TagSelect);

const _TagIDSelect: React.FC<
  IFilterProps & IFilterIDProps<Tag> & { namespace?: TagNamespace }
> = (props) => {
  const { ids, onSelect: onSelectValues } = props;
  const namespace = props.namespace ?? "tags";

  const [values, setValues] = useState<Tag[]>([]);
  const idsChanged = useCompare(ids);

  function onSelect(items: Tag[]) {
    setValues(items);
    onSelectValues?.(items);
  }

  useEffect(() => {
    async function loadObjectsByID(idsToLoad: string[]): Promise<Tag[]> {
      const query = await queryFindTagsByIDForSelect(idsToLoad);
      const { tags: loadedTags } = query.data.findTags;

      return loadedTags.filter((tag) => matchesNamespace(tag, namespace));
    }

    if (!idsChanged) {
      return;
    }

    if (!ids || ids?.length === 0) {
      setValues([]);
      return;
    }

    const filteredValues = values.filter((v) => ids.includes(v.id.toString()));
    if (filteredValues.length === ids.length) {
      return;
    }

    const load = async () => {
      const items = await loadObjectsByID(ids);

      const sortedItems = [...items];
      sortedItems.sort((a, b) => {
        const aName = a.sort_name || a.name;
        const bName = b.sort_name || b.name;

        if (aName && bName) {
          return aName.localeCompare(bName);
        }
        return 0;
      });

      setValues(sortedItems);
    };

    load();
  }, [ids, idsChanged, namespace, values]);

  return <TagSelect {...props} values={values} onSelect={onSelect} />;
};

export const TagIDSelect = PatchComponent("TagIDSelect", _TagIDSelect);

export const CopyrightSelect: React.FC<TagSelectProps> = (props) => (
  <TagSelect {...props} namespace="copyrights" />
);

export const CopyrightIDSelect: React.FC<
  IFilterProps & IFilterIDProps<Tag>
> = (props) => <TagIDSelect {...props} namespace="copyrights" />;
