import React, { useState } from "react";
import { Alert, Badge, Button, Table } from "react-bootstrap";
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
  normalized: string;
  references: AliasCollisionReference[];
}

interface AliasCollisionResponse {
  collisions: AliasCollision[];
  counts: Record<string, number>;
}

export const SettingsToolsPanel: React.FC = () => {
  const [collisions, setCollisions] = useState<AliasCollisionResponse>();
  const [collisionError, setCollisionError] = useState<string>();
  const [collisionLoading, setCollisionLoading] = useState(false);

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
            subHeading="Read-only inspection of canonical-name and alias ambiguity for Characters, Artists, Copyrights, and Tags."
          >
            <Button
              variant="secondary"
              disabled={collisionLoading}
              onClick={() => void inspectAliasCollisions()}
            >
              {collisionLoading ? "Inspecting…" : "Inspect aliases"}
            </Button>
          </Setting>
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
                      <tr key={`${collision.kind}-${collision.normalized}`}>
                        <td>{collision.kind}</td>
                        <td>
                          <code>{collision.normalized}</code>
                        </td>
                        <td>
                          {collision.references.map((reference) => (
                            <div
                              className="mb-1"
                              key={`${collision.kind}-${collision.normalized}-${reference.entityID}`}
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
                This inspector reports ambiguity only. It never rewrites or
                removes aliases.
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
