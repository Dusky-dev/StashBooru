import { useCallback } from "react";
import * as GQL from "src/core/generated-graphql";
import { StudiosCriterion } from "src/models/list-filter/criteria/studios";
import { ListFilterModel } from "src/models/list-filter/filter";

export const useStudioFilterHook = (
  studio: GQL.StudioDataFragment,
  showChildStudioContent?: boolean
) => {
  const { id, name } = studio;
  return useCallback(
    (filter: ListFilterModel) => {
      const studioValue = { id, label: name };
      let studioCriterion = filter.criteria.find(
        (criterion) => criterion.criterionOption.type === "studios"
      ) as StudiosCriterion | undefined;
      if (studioCriterion) {
        studioCriterion.value.items = [studioValue];
        studioCriterion.value.depth = showChildStudioContent ? -1 : 0;
        studioCriterion.modifier = GQL.CriterionModifier.Includes;
      } else {
        studioCriterion = new StudiosCriterion();
        studioCriterion.value = {
          items: [studioValue],
          excluded: [],
          depth: showChildStudioContent ? -1 : 0,
        };
        studioCriterion.modifier = GQL.CriterionModifier.Includes;
        filter.criteria.push(studioCriterion);
      }
      return filter;
    },
    [id, name, showChildStudioContent]
  );
};
