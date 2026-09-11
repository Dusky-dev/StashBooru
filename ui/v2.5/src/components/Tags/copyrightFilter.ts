import { CriterionModifier } from "src/core/generated-graphql";
import { ListFilterModel } from "src/models/list-filter/filter";
import {
  createMandatoryStringCriterionOption,
  StringCriterion,
} from "src/models/list-filter/criteria/criterion";
import {
  ParentTagsCriterionOption,
  TagsCriterion,
} from "src/models/list-filter/criteria/tags";

export interface CopyrightRoot {
  id: number;
  name: string;
}

const nameCriterionOption = createMandatoryStringCriterionOption("name");

export function applyCopyrightNamespaceFilter(
  filter: ListFilterModel,
  root: CopyrightRoot,
  includeCopyright: boolean
) {
  const next = filter.clone();

  const parents = ParentTagsCriterionOption.makeCriterion() as TagsCriterion;
  parents.modifier = CriterionModifier.Includes;
  parents.value = includeCopyright
    ? {
        items: [{ id: String(root.id), label: root.name }],
        excluded: [],
        depth: 0,
      }
    : {
        items: [],
        excluded: [{ id: String(root.id), label: root.name }],
        depth: 0,
      };
  next.criteria = next.criteria.filter(
    (criterion) => criterion.criterionOption.type !== "parents"
  );
  next.criteria.push(parents);

  if (!includeCopyright) {
    const name = nameCriterionOption.makeCriterion() as StringCriterion;
    name.modifier = CriterionModifier.NotEquals;
    name.value = root.name;
    next.criteria.push(name);
  }

  return next;
}
