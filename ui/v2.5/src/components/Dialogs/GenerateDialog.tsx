import React, { useState, useEffect, useMemo } from "react";
import { Form, Button } from "react-bootstrap";
import { mutateMetadataGenerate } from "src/core/StashService";
import { ModalComponent } from "../Shared/Modal";
import { Icon } from "src/components/Shared/Icon";
import { useToast } from "src/hooks/Toast";
import * as GQL from "src/core/generated-graphql";
import { FormattedMessage, useIntl } from "react-intl";
import { useConfigurationContext } from "src/hooks/Config";
import { Manual } from "../Help/Manual";
import { withoutTypename } from "src/utils/data";
import { GenerateOptions } from "../Settings/Tasks/GenerateOptions";
import { SettingSection } from "../Settings/SettingSection";
import { faCogs, faQuestionCircle } from "@fortawesome/free-solid-svg-icons";
import { SettingsContext } from "../Settings/context";

interface IGenerateDialog {
  selectedIds?: string[];
  onClose: () => void;
  type: "scene" | "image" | "gallery";
}

interface CamieBulkResponse {
  jobID: number;
}

interface CamieConfig {
  threshold: number;
  limit: number;
  filenameEnabled: boolean;
  filenameLayout: string;
}

export const GenerateDialog: React.FC<IGenerateDialog> = ({
  selectedIds,
  onClose,
  type,
}) => {
  const sceneIDs = type === "scene" ? selectedIds : undefined;
  const imageIDs = type === "image" ? selectedIds : undefined;
  const galleryIDs = type === "gallery" ? selectedIds : undefined;

  const { configuration } = useConfigurationContext();

  function getDefaultOptions(): GQL.GenerateMetadataInput {
    return {
      sprites: true,
      phashes: true,
      previews: true,
      markers: true,
      previewOptions: {
        previewSegments: 0,
        previewSegmentDuration: 0,
        previewPreset: GQL.PreviewPreset.Slow,
      },
    };
  }

  const [options, setOptions] = useState<GQL.GenerateMetadataInput>(
    getDefaultOptions()
  );
  const [configRead, setConfigRead] = useState(false);
  const [showManual, setShowManual] = useState(false);
  const [animation, setAnimation] = useState(true);
  const [camieThreshold, setCamieThreshold] = useState("0.492");
  const [camieLimit, setCamieLimit] = useState("50");
  const [camieCharacters, setCamieCharacters] = useState(true);
  const [camieArtist, setCamieArtist] = useState(true);
  const [camieCopyright, setCamieCopyright] = useState(true);
  const [camieTags, setCamieTags] = useState(true);
  const [camieReplaceArtist, setCamieReplaceArtist] = useState(false);
  const [camieStarting, setCamieStarting] = useState(false);

  const intl = useIntl();
  const Toast = useToast();

  useEffect(() => {
    if (configRead) {
      return;
    }

    // combine the defaults with the system preview generation settings
    if (configuration?.defaults.generate) {
      const { generate } = configuration.defaults;
      setOptions(withoutTypename(generate));
      setConfigRead(true);
    }

    if (configuration?.general) {
      const { general } = configuration;
      setOptions((existing) => ({
        ...existing,
        previewOptions: {
          ...existing.previewOptions,
          previewSegments:
            general.previewSegments ?? existing.previewOptions?.previewSegments,
          previewSegmentDuration:
            general.previewSegmentDuration ??
            existing.previewOptions?.previewSegmentDuration,
          previewExcludeStart:
            general.previewExcludeStart ??
            existing.previewOptions?.previewExcludeStart,
          previewExcludeEnd:
            general.previewExcludeEnd ??
            existing.previewOptions?.previewExcludeEnd,
          previewPreset:
            general.previewPreset ?? existing.previewOptions?.previewPreset,
        },
      }));
      setConfigRead(true);
    }
  }, [configuration, configRead]);

  useEffect(() => {
    if (type !== "image") return;
    void (async () => {
      try {
        const response = await fetch("image/visual-similarity/camie/config");
        if (!response.ok) return;
        const config = (await response.json()) as CamieConfig;
        setCamieThreshold(String(config.threshold));
        setCamieLimit(String(config.limit));
      } catch {
        // Keep built-in defaults if the optional config endpoint is unavailable.
      }
    })();
  }, [type]);

  const selectionStatus = useMemo(() => {
    const countableIds: Record<typeof type, string> = {
      scene: "countables.scenes",
      image: "countables.images",
      gallery: "countables.galleries",
    };
    const countableId = countableIds[type];

    if (selectedIds) {
      return (
        <Form.Group id="selected-generate-ids">
          <FormattedMessage
            id="config.tasks.generate.generating_scenes"
            values={{
              num: selectedIds.length,
              scene: intl.formatMessage(
                {
                  id: countableId,
                },
                {
                  count: selectedIds.length,
                }
              ),
            }}
          />
          .
        </Form.Group>
      );
    }
    const message = (
      <span>
        <FormattedMessage
          id="config.tasks.generate.generating_scenes"
          values={{
            num: intl.formatMessage({ id: "all" }),
            scene: intl.formatMessage(
              {
                id: countableId,
              },
              {
                count: 0,
              }
            ),
          }}
        />
        .
      </span>
    );

    return (
      <Form.Group className="dialog-selected-folders">
        <div>{message}</div>
      </Form.Group>
    );
  }, [selectedIds, intl, type]);

  async function onGenerate() {
    try {
      await mutateMetadataGenerate({
        ...options,
        sceneIDs,
        imageIDs,
        galleryIDs,
      });
      Toast.success(
        intl.formatMessage(
          { id: "config.tasks.added_job_to_queue" },
          { operation_name: intl.formatMessage({ id: "actions.generate" }) }
        )
      );
    } catch (e) {
      Toast.error(e);
    } finally {
      onClose();
    }
  }

  async function onCamieTag() {
    const threshold = Number.parseFloat(camieThreshold);
    const limit = Number.parseInt(camieLimit, 10);
    if (!(threshold > 0 && threshold < 1)) {
      Toast.error(
        new Error("Camie threshold must be greater than 0 and less than 1.")
      );
      return;
    }
    if (!(limit >= 1 && limit <= 200)) {
      Toast.error(
        new Error("Camie per-category limit must be between 1 and 200.")
      );
      return;
    }
    if (!camieCharacters && !camieArtist && !camieCopyright && !camieTags) {
      Toast.error(new Error("Enable at least one Camie metadata category."));
      return;
    }

    setCamieStarting(true);
    try {
      const response = await fetch("image/visual-similarity/camie/tag", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          imageIDs: imageIDs?.map((id) => Number.parseInt(id, 10)) ?? [],
          all: imageIDs === undefined,
          threshold,
          limit,
          applyCharacters: camieCharacters,
          applyArtist: camieArtist,
          applyCopyright: camieCopyright,
          applyTags: camieTags,
          replaceArtist: camieReplaceArtist,
        }),
      });
      if (!response.ok) {
        throw new Error((await response.text()) || response.statusText);
      }
      const result = (await response.json()) as CamieBulkResponse;
      Toast.success(
        `Started Camie tagging job #${result.jobID}. Threshold ${threshold} and limit ${limit} are now the manual Camie defaults.`
      );
      onClose();
    } catch (e) {
      Toast.error(e);
    } finally {
      setCamieStarting(false);
    }
  }

  function onShowManual() {
    setAnimation(false);
    setShowManual(true);
  }

  if (showManual) {
    return (
      <Manual
        animation={false}
        show
        onClose={() => setShowManual(false)}
        defaultActiveTab="Tasks.md"
      />
    );
  }

  return (
    <ModalComponent
      show
      modalProps={{ animation, size: "lg" }}
      icon={faCogs}
      header={intl.formatMessage({ id: "actions.generate" })}
      accept={{
        onClick: onGenerate,
        text: intl.formatMessage({ id: "actions.generate" }),
      }}
      cancel={{
        onClick: () => onClose(),
        text: intl.formatMessage({ id: "actions.cancel" }),
        variant: "secondary",
      }}
      leftFooterButtons={
        <Button
          title="Help"
          className="minimal help-button"
          onClick={() => onShowManual()}
        >
          <Icon icon={faQuestionCircle} />
        </Button>
      }
    >
      <Form>
        {selectionStatus}
        <SettingsContext>
          <SettingSection>
            <GenerateOptions
              type={type}
              options={options}
              setOptions={setOptions}
              selection
            />
          </SettingSection>
          {type === "image" ? (
            <SettingSection>
              <h4>Camie metadata</h4>
              <p className="text-muted">
                Run the optional Camie Tagger v2 on these images and apply its
                predictions as StashBooru metadata. This starts a separate
                background task and does not run the generation options above.
                The threshold and per-category limit used here are saved as the
                defaults for every manual Camie analysis.
              </p>
              <div className="d-flex flex-wrap align-items-end mb-3">
                <Form.Group className="mr-3 mb-2">
                  <Form.Label>Threshold</Form.Label>
                  <Form.Control
                    type="number"
                    min="0.001"
                    max="0.999"
                    step="0.01"
                    value={camieThreshold}
                    onChange={(event) =>
                      setCamieThreshold(event.currentTarget.value)
                    }
                    style={{ width: "8rem" }}
                  />
                </Form.Group>
                <Form.Group className="mb-2">
                  <Form.Label>Per-category limit</Form.Label>
                  <Form.Control
                    type="number"
                    min="1"
                    max="200"
                    value={camieLimit}
                    onChange={(event) =>
                      setCamieLimit(event.currentTarget.value)
                    }
                    style={{ width: "8rem" }}
                  />
                </Form.Group>
              </div>
              <Form.Check
                className="mb-2"
                type="checkbox"
                id="camie-generate-characters"
                checked={camieCharacters}
                onChange={(event) =>
                  setCamieCharacters(event.currentTarget.checked)
                }
                label="Apply character predictions as Characters"
              />
              <Form.Check
                className="mb-2"
                type="checkbox"
                id="camie-generate-artist"
                checked={camieArtist}
                onChange={(event) =>
                  setCamieArtist(event.currentTarget.checked)
                }
                label="Apply the highest-confidence artist as Artist"
              />
              <Form.Check
                className="mb-2"
                type="checkbox"
                id="camie-generate-copyright"
                checked={camieCopyright}
                onChange={(event) =>
                  setCamieCopyright(event.currentTarget.checked)
                }
                label="Apply copyright / series predictions as Copyright"
              />
              <Form.Check
                className="mb-2"
                type="checkbox"
                id="camie-generate-tags"
                checked={camieTags}
                onChange={(event) => setCamieTags(event.currentTarget.checked)}
                label="Apply general, meta and other predictions as Tags"
              />
              <Form.Check
                className="mb-3"
                type="checkbox"
                id="camie-generate-replace-artist"
                checked={camieReplaceArtist}
                disabled={!camieArtist}
                onChange={(event) =>
                  setCamieReplaceArtist(event.currentTarget.checked)
                }
                label="Replace an existing Artist when Camie predicts one"
              />
              <Button
                variant="primary"
                disabled={camieStarting}
                onClick={() => void onCamieTag()}
              >
                {camieStarting ? "Starting Camie…" : "Tag with Camie"}
              </Button>
            </SettingSection>
          ) : null}
        </SettingsContext>
      </Form>
    </ModalComponent>
  );
};

export default GenerateDialog;
