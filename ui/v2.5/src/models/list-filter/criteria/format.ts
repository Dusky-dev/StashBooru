import { StringCriterion, StringCriterionOption } from "./criterion";
import {
  addFileFilterInput,
  imageFormatFileFilterInput,
  sceneFormatFileFilterInput,
} from "./file-filter";

export const ImageFormatCriterionOption = new StringCriterionOption({
  messageID: "format",
  type: "format",
  makeCriterion: () => new ImageFormatCriterion(),
});

export class ImageFormatCriterion extends StringCriterion {
  constructor() {
    super(ImageFormatCriterionOption);
  }

  public applyToCriterionInput(input: Record<string, unknown>) {
    addFileFilterInput(
      input,
      imageFormatFileFilterInput(this.toCriterionInput())
    );
  }
}

export const SceneFormatCriterionOption = new StringCriterionOption({
  messageID: "format",
  type: "format",
  makeCriterion: () => new SceneFormatCriterion(),
});

export class SceneFormatCriterion extends StringCriterion {
  constructor() {
    super(SceneFormatCriterionOption);
  }

  public applyToCriterionInput(input: Record<string, unknown>) {
    addFileFilterInput(
      input,
      sceneFormatFileFilterInput(this.toCriterionInput())
    );
  }
}
