import { useState } from "react";
import { Alert, Form } from "react-bootstrap";
import { useIntl } from "react-intl";
import { faPencilAlt } from "@fortawesome/free-solid-svg-icons";
import * as GQL from "src/core/generated-graphql";
import { useBulkImageUpdate, useBulkSceneUpdate } from "src/core/StashService";
import { ModalComponent } from "../Shared/Modal";
import { BulkUpdateFormGroup, BulkUpdateTextInput } from "../Shared/BulkUpdate";
import { BulkUpdateDateInput } from "../Shared/DateInput";
import { MultiSet } from "../Shared/MultiSet";
import { StudioSelect } from "../Shared/Select";
import { RatingSystem } from "../Shared/Rating/RatingSystem";
import { getDateError } from "src/utils/yup";
import { dispatchMediaUpdates, splitMediaSelection } from "./mediaSelection";

type SharedUpdate = Pick<
  GQL.BulkImageUpdateInput,
  | "title"
  | "details"
  | "date"
  | "rating100"
  | "organized"
  | "studio_id"
  | "tag_ids"
  | "performer_ids"
>;

export function EditMediaDialog({
  selected,
  onClose,
}: {
  selected: GQL.MediaListItemFragment[];
  onClose: (applied: boolean) => void;
}) {
  const intl = useIntl();
  const [input, setInput] = useState<SharedUpdate>({});
  const [tags, setTags] = useState<GQL.BulkUpdateIds>({
    ids: [],
    mode: GQL.BulkUpdateIdMode.Add,
  });
  const [characters, setCharacters] = useState<GQL.BulkUpdateIds>({
    ids: [],
    mode: GQL.BulkUpdateIdMode.Add,
  });
  const [running, setRunning] = useState(false);
  const [errors, setErrors] = useState<string[]>([]);
  const [completed, setCompleted] = useState(new Set<string>());
  const [updateImages] = useBulkImageUpdate();
  const [updateScenes] = useBulkSceneUpdate();
  const { imageIDs, sceneIDs } = splitMediaSelection(selected);
  const dateError = getDateError(input.date ?? "", intl);
  const update = (value: SharedUpdate) =>
    setInput((old) => ({ ...old, ...value }));
  const relationshipInput = (value: GQL.BulkUpdateIds) =>
    value.ids?.length || value.mode === GQL.BulkUpdateIdMode.Set
      ? value
      : undefined;

  async function apply() {
    setRunning(true);
    setErrors([]);
    const common = {
      ...input,
      tag_ids: relationshipInput(tags),
      performer_ids: relationshipInput(characters),
    };
    const outcomes = await dispatchMediaUpdates(
      selected,
      common,
      (ids, fields) =>
        completed.has("Images")
          ? Promise.resolve()
          : updateImages({ variables: { input: { ...fields, ids } } }),
      (ids, fields) =>
        completed.has("Videos")
          ? Promise.resolve()
          : updateScenes({ variables: { input: { ...fields, ids } } })
    );
    const next = new Set(completed);
    const failures: string[] = [];
    outcomes.forEach((outcome, index) => {
      if (!(index === 0 ? imageIDs : sceneIDs).length) return;
      const name = index === 0 ? "Images" : "Videos";
      if (outcome.status === "fulfilled") next.add(name);
      else failures.push(`${name}: ${String(outcome.reason)}`);
    });
    setCompleted(next);
    setErrors(failures);
    setRunning(false);
    if (!failures.length) onClose(true);
  }

  return (
    <ModalComponent
      show
      icon={faPencilAlt}
      header="Edit selected media"
      accept={{
        onClick: apply,
        text: errors.length
          ? "Retry failed updates"
          : "Apply to selected media",
      }}
      cancel={{
        onClick: () => onClose(completed.size > 0),
        text: "Close",
        variant: "secondary",
      }}
      disabled={running || !!dateError}
      isRunning={running}
    >
      <p>
        {imageIDs.length} Images and {sceneIDs.length} Videos selected. Only
        changed fields are applied.
      </p>
      {errors.map((error) => (
        <Alert key={error} variant="danger">
          {error}
        </Alert>
      ))}
      {!!errors.length && (
        <Alert variant="info">
          Successful updates: {Array.from(completed).join(", ") || "none"}.
          Retry applies only to the failed type.
        </Alert>
      )}
      <Form>
        <fieldset disabled={running || completed.size > 0}>
          <BulkUpdateFormGroup name="title">
            <BulkUpdateTextInput
              value={input.title}
              valueChanged={(title) => update({ title })}
              unsetDisabled={false}
            />
          </BulkUpdateFormGroup>
          <BulkUpdateFormGroup name="details">
            <BulkUpdateTextInput
              value={input.details}
              valueChanged={(details) => update({ details })}
              unsetDisabled={false}
            />
          </BulkUpdateFormGroup>
          <BulkUpdateFormGroup name="date">
            <BulkUpdateDateInput
              value={input.date}
              valueChanged={(date) => update({ date })}
              unsetDisabled={false}
              error={dateError}
            />
          </BulkUpdateFormGroup>
          <BulkUpdateFormGroup name="rating">
            <RatingSystem
              value={input.rating100}
              onSetRating={(rating100) =>
                update({ rating100: rating100 ?? undefined })
              }
            />
          </BulkUpdateFormGroup>
          <Form.Group>
            <Form.Label>Organized</Form.Label>
            <Form.Control
              as="select"
              className="input-control"
              value={input.organized == null ? "" : String(input.organized)}
              onChange={(e) =>
                update({
                  organized:
                    e.target.value === ""
                      ? undefined
                      : e.target.value === "true",
                })
              }
            >
              <option value="">Keep current value</option>
              <option value="true">Yes</option>
              <option value="false">No</option>
            </Form.Control>
          </Form.Group>
          <BulkUpdateFormGroup name="studios">
            <StudioSelect
              ids={input.studio_id ? [input.studio_id] : []}
              onSelect={(items) =>
                update({ studio_id: items[0]?.id ?? undefined })
              }
            />
          </BulkUpdateFormGroup>
          <BulkUpdateFormGroup name="tags">
            <MultiSet
              type="tags"
              ids={tags.ids ?? []}
              mode={tags.mode}
              onUpdate={(ids) => setTags({ ...tags, ids })}
              onSetMode={(mode) => setTags({ ...tags, mode })}
            />
          </BulkUpdateFormGroup>
          <BulkUpdateFormGroup name="performers">
            <MultiSet
              type="performers"
              ids={characters.ids ?? []}
              mode={characters.mode}
              onUpdate={(ids) => setCharacters({ ...characters, ids })}
              onSetMode={(mode) => setCharacters({ ...characters, mode })}
            />
          </BulkUpdateFormGroup>
        </fieldset>
      </Form>
    </ModalComponent>
  );
}
