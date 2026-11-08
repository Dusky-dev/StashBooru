import React, { useState } from "react";
import { Alert, Badge, Button, Table } from "react-bootstrap";
import { defineMessages, FormattedMessage, useIntl } from "react-intl";
import { Link } from "react-router-dom";
import { Setting } from "./Inputs";
import { SettingSection } from "./SettingSection";
import { SettingsDisclosure } from "./SettingsDisclosure";
import { PatchContainerComponent } from "src/patch";
import { ExternalLink } from "../Shared/ExternalLink";

const SettingsToolsSection = PatchContainerComponent("SettingsToolsSection");

const sections = defineMessages({
  health: {
    id: "config.tools.sections.health",
    defaultMessage: "Metadata Health",
  },
  health_help: {
    id: "config.tools.sections.health_help",
    defaultMessage:
      "Check library metadata for conflicts and broken associations without changing it.",
  },
  developer: {
    id: "config.tools.sections.developer",
    defaultMessage: "Developer tools",
  },
  developer_help: {
    id: "config.tools.sections.developer_help",
    defaultMessage:
      "Open the GraphQL playground to inspect or run API operations.",
  },
});

interface MetadataHealthFinding {
  code: string;
  kind: string;
  value?: string;
  entityIDs?: number[];
  mediaID?: number;
  detail?: string;
}

interface MetadataHealthResponse {
  findings: MetadataHealthFinding[];
  counts: Record<string, number>;
}

function findingContext(finding: MetadataHealthFinding) {
  const parts: string[] = [];
  if (finding.mediaID) {
    parts.push(`${finding.kind} #${finding.mediaID}`);
  } else if (finding.kind) {
    parts.push(finding.kind);
  }
  if (finding.entityIDs?.length) {
    parts.push(`IDs: #${finding.entityIDs.join(", #")}`);
  }
  if (finding.value) parts.push(`key: ${finding.value}`);
  return parts.join(" · ");
}

export const SettingsToolsPanel: React.FC = () => {
  const intl = useIntl();
  const [health, setHealth] = useState<MetadataHealthResponse>();
  const [healthError, setHealthError] = useState<string>();
  const [healthLoading, setHealthLoading] = useState(false);

  async function scanMetadataHealth() {
    setHealthLoading(true);
    setHealthError(undefined);
    try {
      const response = await fetch("image/metadata-health");
      if (!response.ok) {
        const message = (await response.text()).trim();
        throw new Error(
          message || `Metadata Health failed (${response.status})`
        );
      }
      setHealth((await response.json()) as MetadataHealthResponse);
    } catch (cause) {
      setHealthError(cause instanceof Error ? cause.message : String(cause));
    } finally {
      setHealthLoading(false);
    }
  }

  return (
    <>
      <SettingSection headingID="config.tools.heading">
        <SettingsToolsSection>
          <SettingsDisclosure
            id="tools-health"
            title={intl.formatMessage(sections.health)}
            description={intl.formatMessage(sections.health_help)}
          >
            <Setting
              heading="Metadata Health"
              subHeading="Read-only diagnostics for duplicate and same-name identities, legacy Copyright Tags, legacy/native Artist drift, orphaned Character/Tag/legacy Artist relationships, and alias-ambiguous legacy Tag migration."
            >
              <Button
                variant="secondary"
                disabled={healthLoading}
                onClick={() => void scanMetadataHealth()}
              >
                {healthLoading ? "Scanning…" : "Scan metadata"}
              </Button>
            </Setting>
            {healthError ? (
              <Alert className="mx-3" variant="danger">
                {healthError}
              </Alert>
            ) : null}
            {health ? (
              <div className="px-3 pb-3">
                <div className="mb-3 d-flex flex-wrap align-items-center">
                  <strong className="mr-3">
                    {health.findings.length} finding
                    {health.findings.length === 1 ? "" : "s"}
                  </strong>
                  {Object.entries(health.counts)
                    .sort(([left], [right]) => left.localeCompare(right))
                    .map(([code, count]) => (
                      <Badge
                        className="mr-2 mb-1"
                        key={code}
                        variant="secondary"
                      >
                        {code}: {count}
                      </Badge>
                    ))}
                </div>
                {health.findings.length === 0 ? (
                  <Alert variant="success">No metadata health findings.</Alert>
                ) : (
                  <Table responsive size="sm">
                    <thead>
                      <tr>
                        <th>Diagnostic</th>
                        <th>Context</th>
                        <th>Details</th>
                      </tr>
                    </thead>
                    <tbody>
                      {health.findings.map((finding, index) => (
                        <tr
                          key={`${finding.code}-${finding.kind}-${finding.mediaID ?? finding.value ?? index}`}
                        >
                          <td>
                            <Badge variant="secondary">{finding.code}</Badge>
                          </td>
                          <td>{findingContext(finding) || "—"}</td>
                          <td>{finding.detail || "—"}</td>
                        </tr>
                      ))}
                    </tbody>
                  </Table>
                )}
                <div className="text-muted">
                  This inspector only reports potential metadata problems and
                  ambiguity. It never changes metadata; any repair must remain
                  an explicit user action.
                </div>
              </div>
            ) : null}
          </SettingsDisclosure>
          <SettingsDisclosure
            id="tools-developer"
            title={intl.formatMessage(sections.developer)}
            description={intl.formatMessage(sections.developer_help)}
          >
            <Setting
              heading={
                <ExternalLink href="/playground">
                  <Button>
                    <FormattedMessage id="config.tools.graphql_playground" />
                  </Button>
                </ExternalLink>
              }
            />
          </SettingsDisclosure>
        </SettingsToolsSection>
      </SettingSection>
      <SettingSection headingID="config.tools.scene_tools">
        <SettingsToolsSection>
          <Setting
            heading={
              <Link to="/sceneDuplicateChecker">
                <Button>
                  <FormattedMessage id="config.tools.scene_duplicate_checker" />
                </Button>
              </Link>
            }
          />
        </SettingsToolsSection>
      </SettingSection>
    </>
  );
};
