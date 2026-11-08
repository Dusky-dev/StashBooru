import { useMemo } from "react";
import { FormattedMessage } from "react-intl";
import { useFindCopyrightsForSelectQuery } from "src/core/generated-graphql";
import { CriterionOption } from "src/models/list-filter/criteria/criterion";
import { CopyrightsCriterionOption } from "src/models/list-filter/criteria/copyrights";
import { ListFilterModel } from "src/models/list-filter/filter";
import { sortByRelevance } from "src/utils/query";
import {
  IUseQueryHookProps,
  makeQueryVariables,
  useLabeledIdFilterState,
} from "./LabeledIdFilter";
import { SidebarListFilter } from "./SidebarListFilter";

// Copyrights are native entities, not general Tags. The Copyright selector
// supports search but does not expose native per-result media counts, so the
// candidate search remains global while the applied criterion filters media.
function useCopyrightQueryFilter({ q, skip }: IUseQueryHookProps) {
  const { data, loading } = useFindCopyrightsForSelectQuery({
    variables: makeQueryVariables(q, {}),
    skip,
  });
  const results = useMemo(
    () =>
      sortByRelevance(
        q,
        data?.findCopyrights.copyrights ?? [],
        (item) => item.name,
        (item) => item.aliases
      ).map((item) => ({ id: item.id, label: item.name })),
    [data?.findCopyrights.copyrights, q]
  );
  return { results, loading };
}

export function SidebarCopyrightsFilter({
  filter,
  setFilter,
}: {
  filter: ListFilterModel;
  setFilter: (value: ListFilterModel) => void;
}) {
  const option: CriterionOption = CopyrightsCriterionOption;
  const state = useLabeledIdFilterState({
    filter,
    setFilter,
    option,
    useQuery: useCopyrightQueryFilter,
    hierarchical: true,
    // Hierarchical selectors render an include-descendants modifier on mount.
    // Without its translation key, react-intl throws and crashes global All.
    includeSubMessageID: "sub_tags",
  });
  return (
    <SidebarListFilter
      {...state}
      data-type={option.type}
      title={<FormattedMessage id="copyrights" />}
      sectionID="copyrights"
    />
  );
}
