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

interface SimilarityMetadataCopyCounts {
  characters: number;
  artists: number;
  copyrights: number;
  tags: number;
}

interface SimilarityMetadataCopyResult {
  imageID: number;
  added: SimilarityMetadataCopyCounts;
  error?: string;
}

interface SimilarityMetadataCopyResponse {
  sourceImageID: number;
  results: SimilarityMetadataCopyResult[];
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

function sumCopiedMetadata(results: SimilarityMetadataCopyResult[]) {
  return results.reduce(
    (total, result) => {
      if (result.error) return total;
      total.characters += result.added.characters;
      total.artists += result.added.artists;
      total.copyrights += result.added.copyrights;
      total.tags += result.added.tags;
      return total;
    },
    { characters: 0, artists: 0, copyrights: 0, tags: 0 }
  );
}

export const SettingsToolsPanel: React.FC = () => {
  const [clusters, setClusters] = useState<SimilarityClusterResponse>();
  const [clusterError, setClusterError] = useState<string>();
  const [clusterLoading, setClusterLoading] = useState(false);
  const [threshold, setThreshold] = useState("0.92");
  const [includeSingletons, setIncludeSingletons] = useState(false);
  const [selectedTargets, setSelectedTargets] = useState<
    Record<number, number[]>
  >({});
  const [copyingCluster, setCopyingCluster] = useState<number>();
  const [copyReports, setCopyReports] = useState<
    Record<number, SimilarityMetadataCopyResponse>
  >({});
  const [copyErrors, setCopyErrors] = useState<Record<number, string>>({});

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
        throw new Error(
          message || `Similarity clustering failed (${response.status})`
        );
      }
      setClusters((await response.json()) as SimilarityClusterResponse);
      setSelectedTargets({});
      setCopyReports({});
      setCopyErrors({});
    } catch (cause) {
      setClusterError(cause instanceof Error ? cause.message : String(cause));
    } finally {
      setClusterLoading(false);
    }
  }

  function setClusterTargets(clusterID: number, imageIDs: number[]) {
    setSelectedTargets((current) => ({ ...current, [clusterID]: imageIDs }));
    setCopyReports((current) => {
      const next = { ...current };
      delete next[clusterID];
      return next;
    });
    setCopyErrors((current) => {
      const next = { ...current };
      delete next[clusterID];
      return next;
    });
  }

  function toggleClusterTarget(clusterID: number, imageID: number) {
    const current = selectedTargets[clusterID] ?? [];
    setClusterTargets(
      clusterID,
      current.includes(imageID)
        ? current.filter((id) => id !== imageID)
        : [...current, imageID]
    );
  }

  async function copyClusterMetadata(cluster: SimilarityCluster) {
    const targetImageIDs = selectedTargets[cluster.preferredID] ?? [];
    if (targetImageIDs.length === 0) return;

    const confirmed = window.confirm(
      `Add missing Characters, Artists, Copyrights, and Tags from preferred Image #${cluster.preferredID} to ${targetImageIDs.length} selected Image${targetImageIDs.length === 1 ? "" : "s"}? Existing metadata is kept; nothing is removed or replaced.`
    );
    if (!confirmed) return;

    setCopyingCluster(cluster.preferredID);
    setCopyErrors((current) => {
      const next = { ...current };
      delete next[cluster.preferredID];
      return next;
    });
    try {
      const response = await fetch(
        "image/visual-similarity/clusters/copy-metadata",
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            sourceImageID: cluster.preferredID,
            targetImageIDs,
          }),
        }
      );
      if (!response.ok) {
        const message = (await response.text()).trim();
        throw new Error(
          message || `Metadata copy failed (${response.status})`
        );
      }
      const report = (await response.json()) as SimilarityMetadataCopyResponse;
      setCopyReports((current) => ({
        ...current,
        [cluster.preferredID]: report,
      }));
    } catch (cause) {
      setCopyErrors((current) => ({
        ...current,
        [cluster.preferredID]:
          cause instanceof Error ? cause.message : String(cause),
      }));
    } finally {
      setCopyingCluster(undefined);
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
            subHeading="Review EVA02 similarity clusters and the suggested preferred copy. Metadata copy is explicit and add-only; this tool never deletes or merges Images automatically."
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
                {clusters.indexedImages} indexed Images ·{" "}
                {clusters.clusters.length} cluster
                {clusters.clusters.length === 1 ? "" : "s"} · threshold{" "}
                {clusters.threshold.toFixed(3)} · {clusters.neighbors} neighbors
              </div>
              {clusters.clusters.length === 0 ? (
                <Alert variant="info">
                  No clusters matched this threshold.
                </Alert>
              ) : (
                clusters.clusters.map((cluster, clusterIndex) => {
                  const targets = selectedTargets[cluster.preferredID] ?? [];
                  const report = copyReports[cluster.preferredID];
                  const copyError = copyErrors[cluster.preferredID];
                  const failed = report?.results.filter((result) => result.error) ?? [];
                  const copied = report ? sumCopiedMetadata(report.results) : undefined;
                  const nonPreferredIDs = cluster.members
                    .filter((member) => !member.preferred)
                    .map((member) => member.id);

                  return (
                    <div
                      className="mb-4"
                      key={`${cluster.preferredID}-${cluster.ids.join("-")}`}
                    >
                      <div className="mb-2 d-flex align-items-center flex-wrap">
                        <strong className="mr-2">
                          Cluster {clusterIndex + 1}
                        </strong>
                        <Badge className="mr-2" variant="success">
                          Preferred Image #{cluster.preferredID}
                        </Badge>
                        {nonPreferredIDs.length > 0 ? (
                          <>
                            <Button
                              className="mr-2"
                              size="sm"
                              variant="outline-secondary"
                              disabled={copyingCluster === cluster.preferredID}
                              onClick={() =>
                                setClusterTargets(
                                  cluster.preferredID,
                                  nonPreferredIDs
                                )
                              }
                            >
                              Select other Images
                            </Button>
                            <Button
                              className="mr-2"
                              size="sm"
                              variant="outline-secondary"
                              disabled={
                                targets.length === 0 ||
                                copyingCluster === cluster.preferredID
                              }
                              onClick={() =>
                                setClusterTargets(cluster.preferredID, [])
                              }
                            >
                              Clear
                            </Button>
                            <Button
                              size="sm"
                              variant="primary"
                              disabled={
                                targets.length === 0 || copyingCluster !== undefined
                              }
                              onClick={() => void copyClusterMetadata(cluster)}
                            >
                              {copyingCluster === cluster.preferredID
                                ? "Copying…"
                                : `Add metadata to ${targets.length} selected`}
                            </Button>
                          </>
                        ) : null}
                      </div>
                      <div className="mb-2 text-muted">
                        Copy action only adds missing Character, Artist, Copyright,
                        and Tag relationships from the preferred Image. Existing
                        target metadata is preserved; legacy StudioID, files,
                        titles, and Images are never changed or removed.
                      </div>
                      {copyError ? <Alert variant="danger">{copyError}</Alert> : null}
                      {report && copied ? (
                        <Alert variant={failed.length > 0 ? "warning" : "success"}>
                          Added {copied.characters} Character, {copied.artists}{" "}
                          Artist, {copied.copyrights} Copyright, and {copied.tags}{" "}
                          Tag relationships across {report.results.length} target
                          Image{report.results.length === 1 ? "" : "s"}.
                          {failed.length > 0 ? (
                            <div className="mt-2">
                              {failed.map((result) => (
                                <div key={result.imageID}>
                                  Image #{result.imageID}: {result.error}
                                </div>
                              ))}
                            </div>
                          ) : null}
                        </Alert>
                      ) : null}
                      <div className="row">
                        {cluster.members.map((member) => (
                          <div
                            className="col-6 col-md-4 col-lg-3 col-xl-2 mb-3"
                            key={member.id}
                          >
                            <Card
                              className={
                                member.preferred
                                  ? "border-success h-100"
                                  : "h-100"
                              }
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
                                {!member.preferred ? (
                                  <Form.Check
                                    className="mt-2"
                                    id={`similarity-copy-${cluster.preferredID}-${member.id}`}
                                    type="checkbox"
                                    label="Add preferred metadata"
                                    checked={targets.includes(member.id)}
                                    disabled={copyingCluster === cluster.preferredID}
                                    onChange={() =>
                                      toggleClusterTarget(
                                        cluster.preferredID,
                                        member.id
                                      )
                                    }
                                  />
                                ) : null}
                              </Card.Body>
                            </Card>
                          </div>
                        ))}
                      </div>
                    </div>
                  );
                })
              )}
              <div className="text-muted">
                Preferred-copy ranking uses resolution, then file size, then ID
                as a deterministic tie-breaker. No file deletion, Image merge,
                relationship replacement, or automatic metadata copy is
                performed here.
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
