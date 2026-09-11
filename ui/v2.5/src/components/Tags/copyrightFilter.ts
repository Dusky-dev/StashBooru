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

export interface TagWithParents {
  parents?: Array<{
    id: string;
    name?: string | null;
  }> | null;
}

let copyrightRootPromise: Promise<CopyrightRoot | null> | undefined;

export function fetchCopyrightRoot() {
  if (!copyrightRootPromise) {
    copyrightRootPromise = fetch(
      "image/visual-similarity/camie/copyright-root"
    )
      .then(async (response) => {
        if (!response.ok) throw new Error(await response.text());
        return (await response.json()) as CopyrightRoot;
      })
      .catch(() => null);
  }
  return copyrightRootPromise;
}

export function isCopyrightTag(
  tag: TagWithParents,
  root?: CopyrightRoot | null
) {
  return (tag.parents ?? []).some((parent) => {
    if (root) return parent.id === String(root.id);
    return parent.name?.trim().toLocaleLowerCase() === "copyright";
  });
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
