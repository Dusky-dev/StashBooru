import React, { useState } from "react";
import { defineMessages, useIntl } from "react-intl";
import { LoadingIndicator } from "src/components/Shared/LoadingIndicator";
import { CharacterDedupTask } from "./CharacterDedupTask";
import { LibraryTasks } from "./LibraryTasks";
import { AnimationInspectionTask } from "./AnimationInspectionTask";
import { AssociationInheritanceTask } from "./AssociationInheritanceTask";
import { VideoOverlapTask } from "./VideoOverlapTask";
import { DataManagementTasks } from "./DataManagementTasks";
import { PluginTasks } from "./PluginTasks";
import { JobTable } from "./JobTable";

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

// Native details keeps the original task components mounted while collapsed.
const TaskSection: React.FC<{
  id: string;
  title: string;
  description: string;
}> = ({ id, title, description, children }) => (
  <details className="settings-details task-section" id={id}>
    <summary aria-labelledby={`${id}-title`} aria-describedby={`${id}-help`}>
      <span className="task-section-title" id={`${id}-title`}>
        {title}
      </span>
      <span className="task-section-help" id={`${id}-help`}>
        {description}
      </span>
    </summary>
    <div className="task-section-content">{children}</div>
  </details>
);

export const SettingsTasksPanel: React.FC = () => {
  const intl = useIntl();
  const [isBackupRunning, setIsBackupRunning] = useState<boolean>(false);
  const [isAnonymiseRunning, setIsAnonymiseRunning] = useState<boolean>(false);

  if (isBackupRunning) {
    return (
      <LoadingIndicator
        message={intl.formatMessage({ id: "config.tasks.backing_up_database" })}
      />
    );
  }

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
        <TaskSection
          id="tasks-library"
          title={intl.formatMessage(sections.library)}
          description={intl.formatMessage(sections.library_help)}
        >
          <LibraryTasks />
        </TaskSection>
        <TaskSection
          id="tasks-profiles"
          title={intl.formatMessage(sections.profiles)}
          description={intl.formatMessage(sections.profiles_help)}
        >
          <CharacterDedupTask />
          <hr />
          <AssociationInheritanceTask />
        </TaskSection>
        <TaskSection
          id="tasks-media"
          title={intl.formatMessage(sections.media)}
          description={intl.formatMessage(sections.media_help)}
        >
          <AnimationInspectionTask />
          <hr />
          <VideoOverlapTask />
        </TaskSection>
        <TaskSection
          id="tasks-maintenance"
          title={intl.formatMessage(sections.maintenance)}
          description={intl.formatMessage(sections.maintenance_help)}
        >
          <DataManagementTasks
            setIsBackupRunning={setIsBackupRunning}
            setIsAnonymiseRunning={setIsAnonymiseRunning}
          />
        </TaskSection>
        <TaskSection
          id="tasks-plugins"
          title={intl.formatMessage(sections.plugins)}
          description={intl.formatMessage(sections.plugins_help)}
        >
          <PluginTasks />
        </TaskSection>
      </div>
    </div>
  );
};
