import React, { useState } from "react";
import { Alert, Badge, Button, Table } from "react-bootstrap";
import { FormattedMessage } from "react-intl";
import { Link } from "react-router-dom";
import { Setting } from "./Inputs";
import { SettingSection } from "./SettingSection";
import { PatchContainerComponent } from "src/patch";
import { ExternalLink } from "../Shared/ExternalLink";

const SettingsToolsSection = PatchContainerComponent("SettingsToolsSection");

interface MetadataHealthFinding {
  code: string;
  severity: string;
  message: string;
  normalized?: string;
  entityKind?: string;
  entityIDs?: number[];
  assetKind?: string;
  assetID?: number;
  legacyArtistID?: number;
  artistIDs?: number[];
}

interface MetadataHealthResponse {
  findings: MetadataHealthFinding[];
  counts: Record<string, number>;
}

function findingContext(finding: MetadataHealthFinding) {
  const parts: string[] = [];
  if (finding.entityKind) {
    parts.push(
      `${finding.entityKind}${
        finding.entityIDs?.length ? ` #${finding.entityIDs.join(", #")}` : ""
      }`
    );
  }
  if (finding.assetKind && finding.assetID) {
    parts.push(`${finding.assetKind} #${finding.assetID}`);
  }
  if (finding.normalized) parts.push(`key: ${finding.normalized}`);
  return parts.join(" · ");
}

function severityVariant(severity: string) {
  switch (severity.toLowerCase()) {
    case "error":
      return "danger";
    case "warning":
      return "warning";
    default:
      return "info";
  }
}

export const SettingsToolsPanel: React.FC = () => {
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
          <Setting
            heading={
              <ExternalLink href="/playground">
                <Button>
                  <FormattedMessage id="config.tools.graphql_playground" />
                </Button>
              </ExternalLink>
            }
          />
          <Setting
            heading="Metadata Health"
            subHeading="Read-only diagnostics for duplicate identity names, legacy Copyright Tags, and legacy/native Artist relationship drift."
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
                    <Badge className="mr-2 mb-1" key={code} variant="secondary">
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
                      <th>Severity</th>
                      <th>Diagnostic</th>
                      <th>Context</th>
                      <th>Details</th>
                    </tr>
                  </thead>
                  <tbody>
                    {health.findings.map((finding, index) => (
                      <tr
                        key={`${finding.code}-${finding.assetKind ?? "entity"}-${finding.assetID ?? index}`}
                      >
                        <td>
                          <Badge variant={severityVariant(finding.severity)}>
                            {finding.severity}
                          </Badge>
                        </td>
                        <td>{finding.code}</td>
                        <td>{findingContext(finding) || "—"}</td>
                        <td>{finding.message}</td>
                      </tr>
                    ))}
                  </tbody>
                </Table>
              )}
              <div className="text-muted">
                This inspector never changes metadata. Repairs must remain
                explicit.
              </div>
            </div>
          ) : null}
        </SettingsToolsSection>
      </SettingSection>
      <SettingSection headingID="config.tools.scene_tools">
        <SettingsToolsSection>
          <Setting
            heading={
              <Link to="/sceneFilenameParser">
                <Button>
                  <FormattedMessage id="config.tools.scene_filename_parser.title" />
                </Button>
              </Link>
            }
          />

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
