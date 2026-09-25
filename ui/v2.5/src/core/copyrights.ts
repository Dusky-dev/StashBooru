import * as GQL from "src/core/generated-graphql";
import {
  IHierarchicalLabeledIdCriterion,
  ModifierCriterionOption,
} from "src/models/list-filter/criteria/criterion";
import { ListFilterModel } from "src/models/list-filter/filter";
import { CriterionType } from "src/models/list-filter/types";

const CopyrightsCriterionOption: ModifierCriterionOption =
  new ModifierCriterionOption({
    messageID: "copyrights",
    type: "copyrights" as CriterionType,
    modifierOptions: [
      GQL.CriterionModifier.IncludesAll,
      GQL.CriterionModifier.Includes,
      GQL.CriterionModifier.Equals,
      GQL.CriterionModifier.IsNull,
      GQL.CriterionModifier.NotNull,
    ],
    defaultModifier: GQL.CriterionModifier.IncludesAll,
    makeCriterion: (option) =>
      new IHierarchicalLabeledIdCriterion(option as ModifierCriterionOption),
  });

export const useCopyrightFilterHook = (
  copyright: Pick<GQL.CopyrightDataFragment, "id" | "name">
) => {
  return (filter: ListFilterModel) => {
    const criterion = new IHierarchicalLabeledIdCriterion(
      CopyrightsCriterionOption,
      {
        items: [
          {
            id: copyright.id,
            label: copyright.name,
          },
        ],
        excluded: [],
        // P04 Copyright pages represent the whole taxonomy branch by default.
        depth: -1,
      }
    );
    criterion.modifier = GQL.CriterionModifier.IncludesAll;

    filter.criteria = filter.criteria.filter(
      (item) => item.criterionOption.type !== ("copyrights" as CriterionType)
    );
    filter.criteria.push(criterion);
    return filter;
  };
};
