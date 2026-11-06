import { IntlShape } from "react-intl";
import { CriterionModifier } from "src/core/generated-graphql";
import { ModifierCriterionOption, MultiStringCriterion } from "./criterion";

/** All-category kind selection; absence of the criterion means both kinds. */
export const MediaKindCriterionOption = new ModifierCriterionOption({
  messageID: "media_types",
  type: "media_types",
  options: ["IMAGE", "VIDEO"],
  modifierOptions: [CriterionModifier.Equals],
  defaultModifier: CriterionModifier.Equals,
  makeCriterion: () => new MediaKindCriterion(),
});

export class MediaKindCriterion extends MultiStringCriterion {
  constructor() {
    super(MediaKindCriterionOption);
  }

  public toCriterionInput(): string[] {
    return this.value;
  }

  protected getLabelValue(_intl: IntlShape): string {
    return this.value
      .map((kind) => (kind === "IMAGE" ? "Images" : "Videos"))
      .join(", ");
  }

  public isValid(): boolean {
    return (
      this.value.length > 0 &&
      this.value.every((kind) => kind === "IMAGE" || kind === "VIDEO")
    );
  }
}
