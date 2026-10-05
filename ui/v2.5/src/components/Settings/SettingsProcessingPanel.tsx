import { Link } from "react-router-dom";
import { ExternalLink } from "../Shared/ExternalLink";
import { InferenceWorkerSettings } from "./InferenceWorkerSettings";
import { VisualSimilaritySettings } from "./VisualSimilaritySettings";
import { MediaConversionSettings } from "./MediaConversionSettings";
import { MediaUpscalingSettings } from "./MediaUpscalingSettings";

export function SettingsProcessingPanel() {
  return (
    <div id="processing-settings" className="processing-settings">
      <p className="text-muted">
        Configure workers and defaults here; start processing from the selected
        media or Tasks. Each section has its own Save action.
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
