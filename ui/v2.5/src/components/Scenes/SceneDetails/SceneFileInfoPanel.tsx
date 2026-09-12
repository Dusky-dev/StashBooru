import React from "react";
import * as GQL from "src/core/generated-graphql";

interface ISceneFileInfoPanelProps {
  scene: GQL.SceneDataFragment;
}

// Kept as a no-op plugin API compatibility shim. File information is no longer
// exposed as a separate Video tab because it duplicated the Details view.
export const SceneFileInfoPanel: React.FC<ISceneFileInfoPanelProps> = () =>
  null;

export default SceneFileInfoPanel;
