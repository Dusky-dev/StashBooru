import { CriterionModifier } from "src/core/generated-graphql";
import {
  IHierarchicalLabeledIdCriterion,
  ModifierCriterionOption,
} from "./criterion";

// Native Copyright filters deliberately use the same hierarchical value shape
// as Tags while resolving Copyright IDs through the Copyright selector/graph.
export class CopyrightsCriterion extends IHierarchicalLabeledIdCriterion {}

export const CopyrightsCriterionOption: ModifierCriterionOption =
  new ModifierCriterionOption({
    messageID: "copyrights",
    type: "copyrights",
    inputType: "copyrights",
    modifierOptions: [
      CriterionModifier.IncludesAll,
      CriterionModifier.Includes,
      CriterionModifier.Equals,
      CriterionModifier.IsNull,
      CriterionModifier.NotNull,
    ],
    defaultModifier: CriterionModifier.IncludesAll,
    makeCriterion: (option) => new CopyrightsCriterion(option),
  });