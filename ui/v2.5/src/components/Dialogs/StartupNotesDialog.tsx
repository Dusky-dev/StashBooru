import React from "react";
import { faCogs } from "@fortawesome/free-solid-svg-icons";
import { useIntl } from "react-intl";
import { Link } from "react-router-dom";
import { ModalComponent } from "../Shared/Modal";
import { MarkdownPage } from "../Shared/MarkdownPage";
import { stashBooruIntroduction } from "src/docs/en/ReleaseNotes";
import type { StartupNotes } from "./startupNotes";

export const StartupNotesDialog: React.FC<{
  notes: NonNullable<StartupNotes>;
  onClose: () => void;
}> = ({ notes, onClose }) => {
  const intl = useIntl();
  const introduction = notes.kind === "introduction";
  const latest = notes.kind === "changes" ? notes.entries[0] : undefined;

  return (
    <ModalComponent
      show
      icon={faCogs}
      dialogClassName="startup-notes-dialog"
      header={
        introduction
          ? intl.formatMessage({
              id: "startup_notes.introduction_title",
              defaultMessage: "Welcome to StashBooru",
            })
          : intl.formatMessage(
              {
                id: "startup_notes.changes_title",
                defaultMessage: "What's new in StashBooru {version}",
              },
              { version: latest?.version }
            )
      }
      accept={{
        onClick: onClose,
        text: introduction
          ? intl.formatMessage({
              id: "startup_notes.get_started",
              defaultMessage: "Get started",
            })
          : intl.formatMessage({ id: "actions.close" }),
      }}
    >
      {introduction ? (
        <MarkdownPage page={stashBooruIntroduction.content} />
      ) : (
        <>
          {notes.entries.map((entry) => (
            <section key={entry.version} className="mb-3">
              <h3>
                StashBooru {entry.version}{" "}
                <small className="text-muted">{entry.date}</small>
              </h3>
              <MarkdownPage page={entry.content} />
            </section>
          ))}
          <p className="text-muted mb-0">
            {intl.formatMessage({
              id: "startup_notes.history",
              defaultMessage: "Earlier versions are listed in",
            })}{" "}
            <Link to="/settings?tab=about#about-changelog" onClick={onClose}>
              {intl.formatMessage({
                id: "startup_notes.history_link",
                defaultMessage: "Settings → About",
              })}
            </Link>
            .
          </p>
        </>
      )}
    </ModalComponent>
  );
};
