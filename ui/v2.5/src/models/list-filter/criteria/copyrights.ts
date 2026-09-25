import { CriterionModifier } from "src/core/generated-graphql";
import {
  IHierarchicalLabeledIdCriterion,
  ModifierCriterionOption,
} from "./criterion";

export class CopyrightsCriterion extends IHierarchicalLabeledIdCriterion {}

export const CopyrightsCriterionOption = new ModifierCriterionOption({
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
  makeCriterion: () => new CopyrightsCriterion(CopyrightsCriterionOption),
});
