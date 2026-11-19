import { useIntl } from "react-intl";
import { ExternalLink } from "../Shared/ExternalLink";
import { ManualLink } from "../Help/context";
import { InferenceWorkerSettings } from "./InferenceWorkerSettings";
import { VisualSimilaritySettings } from "./VisualSimilaritySettings";
import { MediaConversionSettings } from "./MediaConversionSettings";
import { MediaUpscalingSettings } from "./MediaUpscalingSettings";
import { SettingsDisclosure } from "./SettingsDisclosure";

const groups = ["worker", "similarity", "conversion", "upscaling"] as const;

export function SettingsProcessingPanel() {
  const intl = useIntl();
  const content = {
    worker: <InferenceWorkerSettings />,
    similarity: <VisualSimilaritySettings />,
    conversion: <MediaConversionSettings />,
    upscaling: <MediaUpscalingSettings />,
  };
  return (
    <div id="processing-settings" className="processing-settings">
      <p className="text-muted">
        Processing settings and defaults. Use each section’s Save button.
      </p>
      <nav
        className="processing-settings-nav"
        aria-label="Processing settings sections"
      >
        <ManualLink tab="Processing">Processing help</ManualLink>
        <ExternalLink
          href={`https://github.com/Dusky-dev/StashBooru/blob/stashbooru-v${import.meta.env.VITE_APP_STASHBOORU_VERSION}/docs/features.md`}
        >
          Feature guide
        </ExternalLink>
      </nav>
      {groups.map((group) => (
        <SettingsDisclosure
          key={group}
          id={`processing-group-${group}`}
          remember
          title={intl.formatMessage({
            id: `config.processing.groups.${group}`,
          })}
          description={intl.formatMessage({
            id: `config.processing.groups.${group}_help`,
          })}
        >
          {content[group]}
        </SettingsDisclosure>
      ))}
    </div>
  );
}
