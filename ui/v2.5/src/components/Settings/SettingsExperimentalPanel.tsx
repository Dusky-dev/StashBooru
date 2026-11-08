import { Alert } from "react-bootstrap";
import { Link, useLocation } from "react-router-dom";
import { SettingsDisclosure } from "./SettingsDisclosure";
import { settingsSearch } from "./settingsNavigation";
import { ImageStorageEstimateTask } from "./Tasks/ImageStorageEstimateTask";
import { ImageStorageBackupPreflightTask } from "./Tasks/ImageStorageBackupPreflightTask";
import { ImageStorageCandidateTask } from "./Tasks/ImageStorageCandidateTask";
import { ImageStorageRecoveryTask } from "./Tasks/ImageStorageRecoveryTask";
import { ImageStorageMissingOriginalTask } from "./Tasks/ImageStorageMissingOriginalTask";
import { ImageStorageRestorePlanTask } from "./Tasks/ImageStorageRestorePlanTask";
import { ImageStorageShadowTask } from "./Tasks/ImageStorageShadowTask";
import { ImageStorageConsumerAuditTask } from "./Tasks/ImageStorageConsumerAuditTask";
import { VideoOverlapTask } from "./Tasks/VideoOverlapTask";

export function SettingsExperimentalPanel() {
  const location = useLocation();
  const tasksLink = (hash: string) => ({
    pathname: location.pathname,
    search: settingsSearch(location.search, "tasks"),
    hash,
  });

  return (
    <div id="experimental-settings" className="tasks-panel-tasks">
      <h1>Experimental</h1>
      <p className="text-muted">
        Early-stage analysis tools are separated from routine Settings tasks.
        Opening this page does not start a job or enable packed image serving.
      </p>
      <SettingsDisclosure
        id="experimental-video-overlap"
        title="Video overlap / containment"
        description="Explore possible Video duplicates, partial overlaps and clips within compilations. Indexing and matches require review before taking action."
      >
        <span id="tasks-media" />
        <VideoOverlapTask />
      </SettingsDisclosure>
      <section id="experimental-image-storage">
        <h2>P12/P13 image storage experiments</h2>
        <Alert variant="warning">
          <strong>
            P12/P13 storage is not ready to replace original images.
          </strong>{" "}
          Estimates are hypothetical, candidate packs consume extra space, and
          recovery/shadow tests do not permit deleting originals. Preserve your
          original files and independent, verified backups.
        </Alert>
        <p>
          Before creating candidates, make and verify an{" "}
          <Link to={tasksLink("#media-backup-task")}>original-media backup</Link>.
          Follow running experiments in the{" "}
          <Link to={tasksLink("#job-queue")}>Task Queue</Link>.
        </p>
        <SettingsDisclosure
          id="experimental-storage-assess"
          title="Estimate and verify"
          description="Compare potential space savings and inspect backup, restore and media-readiness reports without changing originals."
        >
          <ImageStorageEstimateTask />
          <hr />
          <ImageStorageBackupPreflightTask />
          <hr />
          <ImageStorageRestorePlanTask />
          <hr />
          <ImageStorageConsumerAuditTask />
        </SettingsDisclosure>
        <SettingsDisclosure
          id="experimental-storage-candidates"
          title="Candidate packs and shadow reads"
          description="Create additional experimental P12/P13 candidates and test isolated native reads; original files remain authoritative."
        >
          <ImageStorageCandidateTask />
          <hr />
          <ImageStorageShadowTask />
        </SettingsDisclosure>
        <SettingsDisclosure
          id="experimental-storage-recovery"
          title="Recovery rehearsals"
          description="Reconstruct and verify separate copies, including a missing-original simulation; do not replace the live library."
        >
          <ImageStorageRecoveryTask />
          <hr />
          <ImageStorageMissingOriginalTask />
        </SettingsDisclosure>
      </section>
    </div>
  );
}
