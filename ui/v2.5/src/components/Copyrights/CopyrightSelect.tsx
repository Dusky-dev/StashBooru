import React, { useEffect, useMemo, useState } from "react";
import {
  MultiValueGenericProps,
  OptionProps,
  SingleValueProps,
  components as reactSelectComponents,
} from "react-select";
import cx from "classnames";

import * as GQL from "src/core/generated-graphql";
import { defaultMaxOptionsShown } from "src/core/config";
import { getClient } from "src/core/StashService";
import { useConfigurationContext } from "src/hooks/Config";
import { useCompare } from "src/hooks/state";
import {
  FilterSelectComponent,
  IFilterIDProps,
  IFilterProps,
  IFilterValueProps,
  Option as SelectOption,
} from "src/components/Shared/FilterSelect";
import { sortByRelevance } from "src/utils/query";

export type Copyright = Pick<
  GQL.Copyright,
  "id" | "name" | "sort_name" | "aliases" | "favorite"
>;

type Option = SelectOption<Copyright>;

export type CopyrightSelectProps = IFilterProps &
  IFilterValueProps<Copyright> & {
    excludeIds?: string[];
  };

function sortCopyrightsByRelevance(input: string, items: Copyright[]) {
  return sortByRelevance(
    input,
    items,
    (item) => item.name,
    (item) => item.aliases
  );
}

export const CopyrightSelect: React.FC<CopyrightSelectProps> = (props) => {
  const { configuration } = useConfigurationContext();
  const [createCopyright] = GQL.useCopyrightCreateMutation();
  const client = getClient();
  const maxOptionsShown =
    configuration?.ui.maxOptionsShown ?? defaultMaxOptionsShown;
  const defaultCreatable = !configuration?.interface.disableDropdownCreate.tag;
  const exclude = useMemo(() => props.excludeIds ?? [], [props.excludeIds]);

  async function loadCopyrights(input: string): Promise<Option[]> {
    const result = await client.query<GQL.FindCopyrightsForSelectQuery>({
      query: GQL.FindCopyrightsForSelectDocument,
      variables: {
        filter: {
          q: input || undefined,
          page: 1,
          per_page: maxOptionsShown,
          sort: "name",
          direction: GQL.SortDirectionEnum.Asc,
        },
      },
    });

    const items = result.data.findCopyrights.copyrights.filter(
      (item) => !exclude.includes(item.id.toString())
    );
    return sortCopyrightsByRelevance(input, items).map((object) => ({
      value: object.id,
      object,
    }));
  }

  const CopyrightOption: React.FC<OptionProps<Option, boolean>> = (
    optionProps
  ) => {
    const { object } = optionProps.data;
    const inputValue = optionProps.selectProps.inputValue;
    let alias: string | undefined;
    if (!object.name.toLowerCase().includes(inputValue.toLowerCase())) {
      alias = object.aliases.find((candidate) =>
        candidate.toLowerCase().includes(inputValue.toLowerCase())
      );
    }

    return (
      <reactSelectComponents.Option {...optionProps}>
        <span className="react-select-image-option">
          <span>{object.name}</span>
          {alias && <span className="alias">&nbsp;({alias})</span>}
        </span>
      </reactSelectComponents.Option>
    );
  };

  const CopyrightMultiValueLabel: React.FC<
    MultiValueGenericProps<Option, boolean>
  > = (optionProps) => (
    <reactSelectComponents.MultiValueLabel {...optionProps}>
      {optionProps.data.object.name}
    </reactSelectComponents.MultiValueLabel>
  );

  const CopyrightValueLabel: React.FC<SingleValueProps<Option, boolean>> = (
    optionProps
  ) => (
    <reactSelectComponents.SingleValue {...optionProps}>
      {optionProps.data.object.name}
    </reactSelectComponents.SingleValue>
  );

  const onCreate = async (name: string) => {
    const result = await createCopyright({
      variables: { input: { name } },
    });
    const created = result.data?.copyrightCreate;
    if (!created) throw new Error("Failed to create Copyright");
    return {
      value: created.id,
      item: created,
      message: "Created Copyright",
    };
  };

  const getNamedObject = (id: string, name: string): Copyright => ({
    id,
    name,
    sort_name: "",
    aliases: [],
    favorite: false,
  });

  const isValidNewOption = (inputValue: string, options: Copyright[]) => {
    const value = inputValue.trim().toLowerCase();
    if (!value) return false;
    return !options.some(
      (item) =>
        item.name.toLowerCase() === value ||
        item.aliases.some((alias) => alias.toLowerCase() === value)
    );
  };

  return (
    <FilterSelectComponent<Copyright, boolean>
      {...props}
      className={cx("copyright-select", props.className)}
      loadOptions={loadCopyrights}
      getNamedObject={getNamedObject}
      isValidNewOption={isValidNewOption}
      components={{
        Option: CopyrightOption,
        MultiValueLabel: CopyrightMultiValueLabel,
        SingleValue: CopyrightValueLabel,
      }}
      isMulti={props.isMulti ?? false}
      creatable={props.creatable ?? defaultCreatable}
      onCreate={onCreate}
      placeholder={props.noSelectionString ?? "Select Copyright"}
      closeMenuOnSelect={!props.isMulti}
    />
  );
};

export const CopyrightIDSelect: React.FC<
  IFilterProps & IFilterIDProps<Copyright>
> = (props) => {
  const { ids, onSelect: onSelectValues } = props;
  const [values, setValues] = useState<Copyright[]>([]);
  const idsChanged = useCompare(ids);
  const client = getClient();

  function onSelect(items: Copyright[]) {
    setValues(items);
    onSelectValues?.(items);
  }

  useEffect(() => {
    if (!idsChanged) return;
    if (!ids || ids.length === 0) {
      setValues([]);
      return;
    }

    const retained = values.filter((item) => ids.includes(item.id.toString()));
    if (retained.length === ids.length) return;

    void client
      .query<GQL.FindCopyrightsForSelectQuery>({
        query: GQL.FindCopyrightsForSelectDocument,
        variables: { ids },
      })
      .then((result) => {
        const items = [...result.data.findCopyrights.copyrights].sort((a, b) =>
          (a.sort_name || a.name).localeCompare(b.sort_name || b.name)
        );
        setValues(items);
      });
  }, [client, ids, idsChanged, values]);

  return <CopyrightSelect {...props} values={values} onSelect={onSelect} />;
};
