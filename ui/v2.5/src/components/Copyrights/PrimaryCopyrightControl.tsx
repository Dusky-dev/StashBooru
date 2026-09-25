import React, { useEffect, useState } from "react";
import { Form } from "react-bootstrap";
import * as GQL from "src/core/generated-graphql";
import { useToast } from "src/hooks/Toast";

type MediaType = "image" | "scene";

type CopyrightValue = Pick<GQL.Copyright, "id" | "name" | "sort_name">;

interface IProps {
  mediaType: MediaType;
  mediaID: string;
  copyrights: CopyrightValue[];
  primary?: CopyrightValue | null;
}

export const PrimaryCopyrightControl: React.FC<IProps> = ({
  mediaType,
  mediaID,
  copyrights,
  primary,
}) => {
  const Toast = useToast();
  const [value, setValue] = useState(primary?.id ?? "");
  const [saving, setSaving] = useState(false);
  const [updateImagePrimary] = GQL.useImagePrimaryCopyrightUpdateMutation();
  const [updateScenePrimary] = GQL.useScenePrimaryCopyrightUpdateMutation();

  useEffect(() => setValue(primary?.id ?? ""), [primary?.id]);

  if (copyrights.length === 0) return null;

  async function update(next: string) {
    const previous = value;
    setValue(next);
    setSaving(true);
    try {
      const copyrightID = next || null;
      if (mediaType === "image") {
        await updateImagePrimary({
          variables: { imageID: mediaID, copyrightID },
        });
      } else {
        await updateScenePrimary({
          variables: { sceneID: mediaID, copyrightID },
        });
      }
    } catch (error) {
      setValue(previous);
      Toast.error(error);
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="primary-copyright-control mb-3">
      <Form.Label className="mb-1">Primary Copyright</Form.Label>
      <Form.Control
        as="select"
        value={value}
        disabled={saving}
        onChange={(event) => void update(event.currentTarget.value)}
      >
        <option value="">Automatic branch order</option>
        {copyrights.map((copyright) => (
          <option key={copyright.id} value={copyright.id}>
            {copyright.sort_name || copyright.name}
          </option>
        ))}
      </Form.Control>
      <Form.Text className="text-muted">
        Controls the first Copyright used for deterministic media grouping.
        Clear it to fall back to taxonomy branch order.
      </Form.Text>
    </div>
  );
};
