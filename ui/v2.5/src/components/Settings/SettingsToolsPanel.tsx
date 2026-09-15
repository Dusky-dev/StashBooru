import React, { useState } from "react";
import { Alert, Badge, Button, Form, Table } from "react-bootstrap";
import { FormattedMessage } from "react-intl";
import { Link } from "react-router-dom";
import { Setting } from "./Inputs";
import { SettingSection } from "./SettingSection";
import { PatchContainerComponent } from "src/patch";
import { ExternalLink } from "../Shared/ExternalLink";

const SettingsToolsSection = PatchContainerComponent("SettingsToolsSection");

interface AliasCollisionReference {
  entityID: number;
  name: string;
  matchKinds: string[];
}

interface AliasCollision {
  kind: string;
  value: string;
  references: AliasCollisionReference[];
}

interface AliasCollisionResponse {
  collisions: AliasCollision[];
  counts: Record<string, number>;
}

interface AliasInspectMatch {
  kind: string;
  entityID: number;
  name: string;
  matchKinds: string[];
}

interface AliasInspectResponse {
  input: string;
  normalized: string;
  matches: AliasInspectMatch[];
  ambiguous: boolean;
}

export const SettingsToolsPanel: React.FC = () => {
  const [collisions, setCollisions] = useState<AliasCollisionResponse>();
  const [collisionError, setCollisionError] = useState<string>();
  const [collisionLoading, setCollisionLoading] = useState(false);
  const [inspectValue, setInspectValue] = useState("");
  const [inspectResult, setInspectResult] = useState<AliasInspectResponse>();
  const [inspectError, setInspectError] = useState<string>();
  const [inspectLoading, setInspectLoading] = useState(false);

  async function inspectAliasCollisions() {
    setCollisionLoading(true);
    setCollisionError(undefined);
    try {
      const response = await fetch("image/alias-collisions");
      if (!response.ok) {
        const message = (await response.text()).trim();
        throw new Error(
          message || `Alias Collision inspection failed (${response.status})`
        );
      }
      setCollisions((await response.json()) as AliasCollisionResponse);
    } catch (cause) {
      setCollisionError(cause instanceof Error ? cause.message : String(cause));
    } finally {
      setCollisionLoading(false);
    }
  }

  async function inspectExactAliasValue() {
    const value = inspectValue.trim();
    if (!value) return;
    setInspectLoading(true);
    setInspectError(undefined);
    try {
      const query = new URLSearchParams({ value });
      const response = await fetch(`image/alias-collisions/inspect?${query}`);
      if (!response.ok) {
        const message = (await response.text()).trim();
        throw new Error(
          message || `Alias value inspection failed (${response.status})`
        );
      }
      setInspectResult((await response.json()) as AliasInspectResponse);
    } catch (cause) {
      setInspectError(cause instanceof Error ? cause.message : String(cause));
    } finally {
      setInspectLoading(false);
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
            heading="Alias Collision Inspector"
            subHeading="Read-only inspection of canonical-name and alias ambiguity for Characters, Artists, Copyrights, and Tags. Exact lookup shows every matching entity and how the value matched; it never chooses one automatically."
          >
            <div className="d-flex flex-wrap align-items-center">
              <Form.Control
                className="mr-2 mb-2"
                style={{ maxWidth: "24rem" }}
                value={inspectValue}
                disabled={inspectLoading}
                placeholder="Canonical name or alias"
                aria-label="Canonical name or alias to inspect"
                onChange={(event) => setInspectValue(event.currentTarget.value)}
                onKeyDown={(event) => {
                  if (event.key === "Enter") {
                    event.preventDefault();
                    void inspectExactAliasValue();
                  }
                }}
              />
              <Button
                className="mr-2 mb-2"
                variant="primary"
                disabled={inspectLoading || !inspectValue.trim()}
                onClick={() => void inspectExactAliasValue()}
              >
                {inspectLoading ? "Resolving…" : "Inspect value"}
              </Button>
              <Button
                className="mb-2"
                variant="secondary"
                disabled={collisionLoading}
                onClick={() => void inspectAliasCollisions()}
              >
                {collisionLoading ? "Inspecting…" : "Scan all collisions"}
              </Button>
            </div>
          </Setting>
          {inspectError ? (
            <Alert className="mx-3" variant="danger">
              {inspectError}
            </Alert>
          ) : null}
          {inspectResult ? (
            <div className="px-3 pb-3">
              <Alert
                variant={
                  inspectResult.ambiguous
                    ? "warning"
                    : inspectResult.matches.length === 0
                      ? "secondary"
                      : "success"
                }
              >
                <strong>{inspectResult.input}</strong> normalizes to{" "}
                <code>{inspectResult.normalized}</code> and matches{" "}
                {inspectResult.matches.length} native entit
                {inspectResult.matches.length === 1 ? "y" : "ies"}.
                {inspectResult.ambiguous
                  ? " This value is ambiguous; no entity is selected automatically."
                  : ""}
              </Alert>
              {inspectResult.matches.length > 0 ? (
                <Table responsive size="sm">
                  <thead>
                    <tr>
                      <th>Kind</th>
                      <th>Entity</th>
                      <th>Matched as</th>
                    </tr>
                  </thead>
                  <tbody>
                    {inspectResult.matches.map((match) => (
                      <tr key={`${match.kind}-${match.entityID}`}>
                        <td>{match.kind}</td>
                        <td>
                          {match.name} #{match.entityID}
                        </td>
                        <td>
                          {match.matchKinds.map((matchKind) => (
                            <Badge
                              className="mr-1"
                              key={matchKind}
                              variant={
                                matchKind === "canonical" ? "primary" : "info"
                              }
                            >
                              {matchKind}
                            </Badge>
                          ))}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </Table>
              ) : null}
            </div>
          ) : null}
          {collisionError ? (
            <Alert className="mx-3" variant="danger">
              {collisionError}
            </Alert>
          ) : null}
          {collisions ? (
            <div className="px-3 pb-3">
              <div className="mb-3 d-flex flex-wrap align-items-center">
                <strong className="mr-3">
                  {collisions.collisions.length} collision
                  {collisions.collisions.length === 1 ? "" : "s"}
                </strong>
                {Object.entries(collisions.counts)
                  .sort(([left], [right]) => left.localeCompare(right))
                  .map(([kind, count]) => (
                    <Badge className="mr-2 mb-1" key={kind} variant="secondary">
                      {kind}: {count}
                    </Badge>
                  ))}
              </div>
              {collisions.collisions.length === 0 ? (
                <Alert variant="success">No alias collisions found.</Alert>
              ) : (
                <Table responsive size="sm">
                  <thead>
                    <tr>
                      <th>Kind</th>
                      <th>Normalized value</th>
                      <th>Matching entities</th>
                    </tr>
                  </thead>
                  <tbody>
                    {collisions.collisions.map((collision) => (
                      <tr key={`${collision.kind}-${collision.value}`}>
                        <td>{collision.kind}</td>
                        <td>
                          <code>{collision.value}</code>
                        </td>
                        <td>
                          {collision.references.map((reference) => (
                            <div
                              className="mb-1"
                              key={`${collision.kind}-${collision.value}-${reference.entityID}`}
                            >
                              <strong>
                                {reference.name} #{reference.entityID}
                              </strong>{" "}
                              {reference.matchKinds.map((matchKind) => (
                                <Badge
                                  className="mr-1"
                                  key={matchKind}
                                  variant={
                                    matchKind === "canonical"
                                      ? "primary"
                                      : "info"
                                  }
                                >
                                  {matchKind}
                                </Badge>
                              ))}
                            </div>
                          ))}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </Table>
              )}
              <div className="text-muted">
                This inspector reports ambiguity and provenance only. It never
                rewrites or removes aliases, and exact lookup never silently
                resolves an ambiguous value.
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
