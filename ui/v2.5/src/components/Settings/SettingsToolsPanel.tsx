import React, { useState } from "react";
import { Alert, Badge, Button, Card, Form } from "react-bootstrap";
import { FormattedMessage } from "react-intl";
import { Link } from "react-router-dom";
import { Setting } from "./Inputs";
import { SettingSection } from "./SettingSection";
import { PatchContainerComponent } from "src/patch";
import { ExternalLink } from "../Shared/ExternalLink";

const SettingsToolsSection = PatchContainerComponent("SettingsToolsSection");

interface SimilarityClusterMember {
  id: number;
  width: number;
  height: number;
  fileSize: number;
  preferred: boolean;
}

interface SimilarityCluster {
  ids: number[];
  members: SimilarityClusterMember[];
  preferredID: number;
}

interface SimilarityClusterResponse {
  threshold: number;
  neighbors: number;
  indexedImages: number;
  clusters: SimilarityCluster[];
  singletonsShown: boolean;
}

function formatBytes(value: number) {
  if (!value) return "unknown size";
  const units = ["B", "KiB", "MiB", "GiB"];
  let size = value;
  let unit = 0;
  while (size >= 1024 && unit < units.length - 1) {
    size /= 1024;
    unit++;
  }
  return `${size.toFixed(unit === 0 ? 0 : 1)} ${units[unit]}`;
}

export const SettingsToolsPanel: React.FC = () => {
  const [clusters, setClusters] = useState<SimilarityClusterResponse>();
  const [clusterError, setClusterError] = useState<string>();
  const [clusterLoading, setClusterLoading] = useState(false);
  const [threshold, setThreshold] = useState("0.92");
  const [includeSingletons, setIncludeSingletons] = useState(false);

  async function loadSimilarityClusters() {
    setClusterLoading(true);
    setClusterError(undefined);
    try {
      const query = new URLSearchParams({
        threshold,
        includeSingletons: String(includeSingletons),
      });
      const response = await fetch(`image/visual-similarity/clusters?${query}`);
      if (!response.ok) {
        const message = (await response.text()).trim();
        throw new Error(message || `Similarity clustering failed (${response.status})`);
      }
      setClusters((await response.json()) as SimilarityClusterResponse);
    } catch (cause) {
      setClusterError(cause instanceof Error ? cause.message : String(cause));
    } finally {
      setClusterLoading(false);
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
            heading="Visual Similarity Triage"
            subHeading="Review EVA02 similarity clusters and the suggested preferred copy. This tool never deletes or merges Images automatically."
          >
            <div className="d-flex align-items-center flex-wrap">
              <Form.Control
                className="mr-2"
                style={{ width: "7rem" }}
                type="number"
                min="0.01"
                max="1"
                step="0.01"
                value={threshold}
                disabled={clusterLoading}
                aria-label="Similarity threshold"
                onChange={(event) => setThreshold(event.currentTarget.value)}
              />
              <Form.Check
                className="mr-3"
                id="similarity-clusters-include-singletons"
                type="checkbox"
                label="Show singletons"
                checked={includeSingletons}
                disabled={clusterLoading}
                onChange={() => setIncludeSingletons(!includeSingletons)}
              />
              <Button
                variant="secondary"
                disabled={clusterLoading}
                onClick={() => void loadSimilarityClusters()}
              >
                {clusterLoading ? "Clustering…" : "Find clusters"}
              </Button>
            </div>
          </Setting>
          {clusterError ? (
            <Alert className="mx-3" variant="danger">
              {clusterError}
            </Alert>
          ) : null}
          {clusters ? (
            <div className="px-3 pb-3">
              <div className="mb-3 text-muted">
                {clusters.indexedImages} indexed Images · {clusters.clusters.length}{" "}
                cluster{clusters.clusters.length === 1 ? "" : "s"} · threshold{" "}
                {clusters.threshold.toFixed(3)} · {clusters.neighbors} neighbors
              </div>
              {clusters.clusters.length === 0 ? (
                <Alert variant="info">No clusters matched this threshold.</Alert>
              ) : (
                clusters.clusters.map((cluster, clusterIndex) => (
                  <div
                    className="mb-4"
                    key={`${cluster.preferredID}-${cluster.ids.join("-")}`}
                  >
                    <div className="mb-2 d-flex align-items-center">
                      <strong className="mr-2">Cluster {clusterIndex + 1}</strong>
                      <Badge variant="success">
                        Preferred Image #{cluster.preferredID}
                      </Badge>
                    </div>
                    <div className="row">
                      {cluster.members.map((member) => (
                        <div
                          className="col-6 col-md-4 col-lg-3 col-xl-2 mb-3"
                          key={member.id}
                        >
                          <Card
                            className={member.preferred ? "border-success h-100" : "h-100"}
                          >
                            <Link to={`/images/${member.id}`}>
                              <Card.Img
                                variant="top"
                                src={`image/${member.id}/thumbnail`}
                                alt={`Image ${member.id}`}
                              />
                            </Link>
                            <Card.Body className="p-2">
                              <div className="d-flex justify-content-between align-items-center">
                                <Link to={`/images/${member.id}`}>
                                  Image #{member.id}
                                </Link>
                                {member.preferred ? (
                                  <Badge variant="success">Preferred</Badge>
                                ) : null}
                              </div>
                              <small className="text-muted">
                                {member.width && member.height
                                  ? `${member.width}×${member.height}`
                                  : "unknown resolution"}
                                {" · "}
                                {formatBytes(member.fileSize)}
                              </small>
                            </Card.Body>
                          </Card>
                        </div>
                      ))}
                    </div>
                  </div>
                ))
              )}
              <div className="text-muted">
                Preferred-copy ranking uses resolution, then file size, then ID as a deterministic tie-breaker. No destructive action is performed here.
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
