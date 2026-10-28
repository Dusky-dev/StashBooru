import React, { useState } from "react";
import { defineMessages, useIntl } from "react-intl";
import { LoadingIndicator } from "src/components/Shared/LoadingIndicator";
import { CharacterDedupTask } from "./CharacterDedupTask";
import { LibraryTasks } from "./LibraryTasks";
import { AnimationInspectionTask } from "./AnimationInspectionTask";
import { AssociationInheritanceTask } from "./AssociationInheritanceTask";
import { VideoOverlapTask } from "./VideoOverlapTask";
import { ImageStorageEstimateTask } from "./ImageStorageEstimateTask";
import { DataManagementTasks } from "./DataManagementTasks";
import { MediaBackupTask } from "./MediaBackupTask";
import { PluginTasks } from "./PluginTasks";
import { JobTable } from "./JobTable";
import { SettingsDisclosure } from "../SettingsDisclosure";

const sections = defineMessages({
  library: {
    id: "config.tasks.sections.library",
    defaultMessage: "Update library",
  },
  library_help: {
    id: "config.tasks.sections.library_help",
    defaultMessage:
      "Scan folders, generate previews and hashes, or match metadata from paths.",
  },
  profiles: {
    id: "config.tasks.sections.profiles",
    defaultMessage: "Review profiles & associations",
  },
  profiles_help: {
    id: "config.tasks.sections.profiles_help",
    defaultMessage:
      "Preview duplicate Character merges and changes to inherited associations before applying them.",
  },
  media: {
    id: "config.tasks.sections.media",
    defaultMessage: "Inspect media",
  },
  media_help: {
    id: "config.tasks.sections.media_help",
    defaultMessage:
      "Inspect animation frames and review Video overlap or containment.",
  },
  maintenance: {
    id: "config.tasks.sections.maintenance",
    defaultMessage: "Backup & maintenance",
  },
  maintenance_help: {
    id: "config.tasks.sections.maintenance_help",
    defaultMessage:
      "Back up the database, import or export metadata, clean generated files and run migrations.",
  },
  plugins: {
    id: "config.tasks.sections.plugins",
    defaultMessage: "Plugin tasks",
  },
  plugins_help: {
    id: "config.tasks.sections.plugins_help",
    defaultMessage: "Tasks from enabled plugins appear here when available.",
  },
});

export const SettingsTasksPanel: React.FC = () => {
  const intl = useIntl();
  const [isAnonymiseRunning, setIsAnonymiseRunning] = useState<boolean>(false);

  if (isAnonymiseRunning) {
    return (
      <LoadingIndicator
        message={intl.formatMessage({
          id: "config.tasks.anonymising_database",
        })}
      />
    );
  }

  return (
    <div id="tasks-panel">
      <div className="tasks-panel-queue" id="job-queue">
        <h1>{intl.formatMessage({ id: "config.tasks.job_queue" })}</h1>
        <JobTable />
      </div>

      <div className="tasks-panel-tasks">
        <SettingsDisclosure
          id="tasks-library"
          title={intl.formatMessage(sections.library)}
          description={intl.formatMessage(sections.library_help)}
        >
          <LibraryTasks />
        </SettingsDisclosure>
        <SettingsDisclosure
          id="tasks-profiles"
          title={intl.formatMessage(sections.profiles)}
          description={intl.formatMessage(sections.profiles_help)}
        >
          <CharacterDedupTask />
          <hr />
          <AssociationInheritanceTask />
        </SettingsDisclosure>
        <SettingsDisclosure
          id="tasks-media"
          title={intl.formatMessage(sections.media)}
          description={intl.formatMessage(sections.media_help)}
        >
          <AnimationInspectionTask />
          <hr />
          <VideoOverlapTask />
          <hr />
          <ImageStorageEstimateTask />
        </SettingsDisclosure>
        <SettingsDisclosure
          id="tasks-maintenance"
          title={intl.formatMessage(sections.maintenance)}
          description={intl.formatMessage(sections.maintenance_help)}
        >
          <MediaBackupTask />
          <hr />
          <DataManagementTasks setIsAnonymiseRunning={setIsAnonymiseRunning} />
        </SettingsDisclosure>
        <SettingsDisclosure
          id="tasks-plugins"
          title={intl.formatMessage(sections.plugins)}
          description={intl.formatMessage(sections.plugins_help)}
        >
          <PluginTasks />
        </SettingsDisclosure>
      </div>
    </div>
  );
};
