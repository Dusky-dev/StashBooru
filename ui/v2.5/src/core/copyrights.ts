import * as GQL from "src/core/generated-graphql";
import {
  ILabeledIdCriterion,
  ILabeledIdCriterionOption,
} from "src/models/list-filter/criteria/criterion";
import { ListFilterModel } from "src/models/list-filter/filter";
import { CriterionType } from "src/models/list-filter/types";

const CopyrightsCriterionOption = new ILabeledIdCriterionOption(
  "copyrights",
  "copyrights" as CriterionType,
  true,
  undefined
);

export const useCopyrightFilterHook = (
  copyright: Pick<GQL.CopyrightDataFragment, "id" | "name">
) => {
  return (filter: ListFilterModel) => {
    const criterion = new ILabeledIdCriterion(CopyrightsCriterionOption, [
      {
        id: copyright.id,
        label: copyright.name,
      },
    ]);
    criterion.modifier = GQL.CriterionModifier.IncludesAll;

    filter.criteria = filter.criteria.filter(
      (item) => item.criterionOption.type !== ("copyrights" as CriterionType)
    );
    filter.criteria.push(criterion);
    return filter;
  };
};
