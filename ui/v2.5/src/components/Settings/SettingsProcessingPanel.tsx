import { Link } from "react-router-dom";
import { ExternalLink } from "../Shared/ExternalLink";
import { ManualLink } from "../Help/context";
import { InferenceWorkerSettings } from "./InferenceWorkerSettings";
import { VisualSimilaritySettings } from "./VisualSimilaritySettings";
import { MediaConversionSettings } from "./MediaConversionSettings";
import { MediaUpscalingSettings } from "./MediaUpscalingSettings";

export function SettingsProcessingPanel() {
  return (
    <div id="processing-settings" className="processing-settings">
      <p className="text-muted">
        Processing settings and defaults. Use each section’s Save button.
      </p>
      <nav
        className="processing-settings-nav"
        aria-label="Processing settings sections"
      >
        <Link to="/settings?tab=processing#inference-worker">Worker</Link>
        <Link to="/settings?tab=processing#visual-similarity">
          Similarity and tagging
        </Link>
        <Link to="/settings?tab=processing#media-converter">Conversion</Link>
        <Link to="/settings?tab=processing#media-upscaling">Upscaling</Link>
        <ManualLink tab="Processing">Processing help</ManualLink>
        <ExternalLink
          href={`https://github.com/Dusky-dev/StashBooru/blob/stashbooru-v${import.meta.env.VITE_APP_STASHBOORU_VERSION}/docs/features.md`}
        >
          Feature guide
        </ExternalLink>
      </nav>
      <InferenceWorkerSettings />
      <VisualSimilaritySettings />
      <MediaConversionSettings />
      <MediaUpscalingSettings />
    </div>
  );
}
