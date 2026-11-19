import { useRef, useState } from "react";
import { useOrganizedEnabled } from "src/hooks/Config";
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
import {
  dispatchMediaUpdates,
  splitMediaSelection,
  summarizeMediaUpdates,
} from "./mediaSelection";

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
  onClose: (confirmed: string[]) => void;
}) {
  const intl = useIntl();
  const organizedEnabled = useOrganizedEnabled();
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
  const writing = useRef(false);
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

  const common = {
    ...input,
    tag_ids: relationshipInput(tags),
    performer_ids: relationshipInput(characters),
  };
  const hasChanges = Object.values(common).some((value) => value !== undefined);
  const pending = selected.filter((item) => !completed.has(item.id));
  const pendingCounts = splitMediaSelection(pending);
  const clears = [
    tags.mode === GQL.BulkUpdateIdMode.Set && !tags.ids?.length ? "Tags" : "",
    characters.mode === GQL.BulkUpdateIdMode.Set && !characters.ids?.length
      ? "Characters"
      : "",
  ].filter(Boolean);

  async function apply() {
    if (writing.current || !hasChanges || !pending.length || dateError) return;
    writing.current = true;
    setRunning(true);
    setErrors([]);
    try {
      const outcomes = await dispatchMediaUpdates(
        pending,
        common,
        async (ids, fields) => {
          const result = await updateImages({
            variables: { input: { ...fields, ids } },
          });
          return result.data?.bulkImageUpdate?.map((image) => image.id) ?? [];
        },
        async (ids, fields) => {
          const result = await updateScenes({
            variables: { input: { ...fields, ids } },
          });
          return result.data?.bulkSceneUpdate?.map((scene) => scene.id) ?? [];
        }
      );
      const report = summarizeMediaUpdates(pending, outcomes);
      const next = new Set([...completed, ...report.confirmed]);
      setCompleted(next);
      setErrors(report.errors);
      if (next.size === selected.length) onClose([...next]);
    } finally {
      writing.current = false;
      setRunning(false);
    }
  }

  return (
    <ModalComponent
      show
      icon={faPencilAlt}
      header="Edit selected media"
      accept={{
        onClick: apply,
        text: errors.length
          ? "Retry remaining updates"
          : "Apply to selected media",
      }}
      cancel={{
        onClick: () => onClose([...completed]),
        text: completed.size ? "Close and keep remaining selected" : "Close",
        variant: "secondary",
      }}
      disabled={running || !!dateError || !hasChanges || !pending.length}
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
          Stash confirmed {completed.size} of {selected.length} updates. Retry
          sends only the remaining {pendingCounts.imageIDs.length} Images and{" "}
          {pendingCounts.sceneIDs.length} Videos. Closing keeps those items
          selected.
        </Alert>
      )}
      {!hasChanges && (
        <p className="text-muted">Choose a field or relationship to update.</p>
      )}
      {!!clears.length && (
        <Alert variant="warning">
          This will clear all {clears.join(" and ")} on the remaining items.
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
          {organizedEnabled && (
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
          )}
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
