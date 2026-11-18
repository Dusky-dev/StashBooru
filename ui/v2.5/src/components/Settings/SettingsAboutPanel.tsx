import React from "react";
import { Alert, Button } from "react-bootstrap";
import { useIntl } from "react-intl";
import { useLatestVersion } from "src/core/StashService";
import { ExternalLink } from "../Shared/ExternalLink";
import { ConstantSetting, SettingGroup } from "./Inputs";
import { SettingSection } from "./SettingSection";
import { compareStableVersions } from "./releaseVersion";
import { SettingsLogsPanel } from "./SettingsLogsPanel";
import Changelog from "../Changelog/Changelog";

export const SettingsAboutPanel: React.FC = () => {
  const gitHash = import.meta.env.VITE_APP_GITHASH;
  const stashVersion = import.meta.env.VITE_APP_STASH_VERSION;
  const version = import.meta.env.VITE_APP_STASHBOORU_VERSION;
  const buildTime = import.meta.env.VITE_APP_DATE;
  const intl = useIntl();
  const { data, error, loading, refetch, networkStatus } = useLatestVersion();
  const checking = loading || networkStatus === 4;
  const latest = data?.latestversion;
  const comparison = latest
    ? compareStableVersions(latest.version, version)
    : undefined;

  return (
    <>
      <SettingsLogsPanel />
      <SettingSection heading={`StashBooru ${version}`}>
        <SettingGroup settingProps={{ heading: "Installed build" }}>
          <ConstantSetting
            headingID="config.about.upstream_version"
            value={stashVersion ?? "Unknown"}
          />
          <ConstantSetting
            headingID="config.about.build_hash"
            value={gitHash}
          />
          <ConstantSetting
            headingID="config.about.build_time"
            value={buildTime}
          />
        </SettingGroup>
        <div className="p-3">
          {checking ? (
            <p role="status">Checking for updates…</p>
          ) : error ? (
            <Alert variant="warning">
              Could not check for updates: {error.message}
            </Alert>
          ) : latest ? (
            <p role="status">
              <span>
                {comparison === undefined
                  ? "Could not compare release versions."
                  : comparison > 0
                    ? `StashBooru ${latest.version} is available.`
                    : comparison === 0
                      ? "You are running the latest stable StashBooru release."
                      : `This build is newer than the latest stable release (${latest.version}).`}
              </span>{" "}
              <small className="text-muted">
                Latest release: {latest.version} · {latest.release_date}
              </small>
            </p>
          ) : null}
          <div className="settings-actions">
            <Button
              disabled={checking}
              onClick={() => {
                void refetch().catch(() => undefined);
              }}
            >
              Check for updates
            </Button>
            {latest?.url && !error && !checking && (
              <a className="btn btn-secondary" href={latest.url}>
                Download release
              </a>
            )}
          </div>
          <p className="about-links mt-3 mb-0">
            <ExternalLink href="https://github.com/Dusky-dev/StashBooru">
              StashBooru on GitHub
            </ExternalLink>
            {" · "}
            <ExternalLink href="https://github.com/Dusky-dev/StashBooru/releases">
              Release notes
            </ExternalLink>
            {" · "}
            <ExternalLink
              href={`https://github.com/Dusky-dev/StashBooru/blob/stashbooru-v${import.meta.env.VITE_APP_STASHBOORU_VERSION}/docs/features.md`}
            >
              Feature guide
            </ExternalLink>
          </p>
          <small className="text-muted d-block mt-2">
            {intl.formatMessage(
              { id: "config.about.stash_home" },
              {
                url: (
                  <ExternalLink href="https://github.com/stashapp/stash">
                    GitHub
                  </ExternalLink>
                ),
              }
            )}{" "}
            {intl.formatMessage(
              { id: "config.about.stash_wiki" },
              {
                url: (
                  <ExternalLink href="https://docs.stashapp.cc">
                    documentation
                  </ExternalLink>
                ),
              }
            )}{" "}
            {intl.formatMessage(
              { id: "config.about.stash_community" },
              {
                forumUrl: (
                  <ExternalLink href="https://discourse.stashapp.cc">
                    forum
                  </ExternalLink>
                ),
                discordUrl: (
                  <ExternalLink href="https://discord.gg/2TsNFKt">
                    Discord
                  </ExternalLink>
                ),
              }
            )}{" "}
            {intl.formatMessage(
              { id: "config.about.support_us" },
              {
                openCollectiveUrl: (
                  <ExternalLink href="https://opencollective.com/stashapp">
                    Open Collective
                  </ExternalLink>
                ),
                githubSponsorsUrl: (
                  <ExternalLink href="https://github.com/sponsors/stashapp">
                    GitHub Sponsors
                  </ExternalLink>
                ),
              }
            )}
          </small>
        </div>
      </SettingSection>
      <div id="about-changelog">
        <Changelog />
      </div>
    </>
  );
};
