import React, { useCallback, useEffect, useRef, useState } from "react";
import {
  Accordion,
  Alert,
  Button,
  Card,
  Form,
  ProgressBar,
  Table,
} from "react-bootstrap";
import { useIntl } from "react-intl";
import { Link } from "react-router-dom";
import { useSettings } from "../context";

type Domain = "character" | "artist" | "copyright" | "tag";

interface InheritanceDefaults {
  characters: boolean;
  artists: boolean;
  copyrights: boolean;
  tags: boolean;
}

interface Origin {
  kind: string;
  sourceType: Domain;
  sourceID: number;
  viaType?: Domain;
  viaID?: number;
  sourceTagID?: number;
}

interface Membership {
  associationType: Domain;
  associationID: number;
  origins: Origin[];
}

interface ReviewSample {
  mediaType: "image" | "video";
  mediaID: number;
  label: string;
  before: Membership[];
  after: Membership[];
  truncated: boolean;
}

interface DomainSummary {
  beforeInherited: number;
  afterInherited: number;
  added: number;
  removed: number;
}

interface ReviewReport {
  imagesReviewed: number;
  videosReviewed: number;
  affectedMedia: number;
  errorCount: number;
  errorSamples: string[];
  domains: Record<Domain, DomainSummary>;
  samples: ReviewSample[];
}

interface Review {
  reviewID: string;
  jobID: number;
  status: string;
  current: InheritanceDefaults;
  proposed: InheritanceDefaults;
  processed: number;
  total: number;
  report?: ReviewReport;
  error?: string;
}

const endpoint = "association-inheritance";
const storageKey = "association-inheritance-review";
const prefix = "config.association_inheritance.review";
const domains: Domain[] = ["character", "artist", "copyright", "tag"];
const routes: Record<Domain, string> = {
  character: "performers",
  artist: "studios",
  copyright: "copyrights",
  tag: "tags",
};
const flags: (keyof InheritanceDefaults)[] = [
  "characters",
  "artists",
  "copyrights",
  "tags",
];

function active(status?: string) {
  return [
    "queued_preview",
    "previewing",
    "queued_apply",
    "applying",
    "cancelling",
  ].includes(status ?? "");
}

async function readResponse<T>(response: Response): Promise<T> {
  if (!response.ok) {
    throw new Error((await response.text()) || response.statusText);
  }
  return response.json() as Promise<T>;
}

function remember(id?: string) {
  try {
    if (id) sessionStorage.setItem(storageKey, id);
    else sessionStorage.removeItem(storageKey);
  } catch {
    // A disabled session store does not prevent review or application.
  }
}

const MembershipList: React.FC<{ items: Membership[] }> = ({ items }) => {
  const intl = useIntl();
  const label = (domain: Domain) =>
    intl.formatMessage({ id: prefix + "." + domain });
  const sourceLink = (domain: Domain, id: number) => (
    <Link to={"/" + routes[domain] + "/" + id}>
      {label(domain)} #{id}
    </Link>
  );

  return (
    <ul className="pl-3 mb-0">
      {items.map((membership) => (
        <li key={membership.associationType + ":" + membership.associationID}>
          {sourceLink(membership.associationType, membership.associationID)}
          <ul className="pl-3">
            {membership.origins.map((origin) => (
              <li key={JSON.stringify(origin)}>
                {intl.formatMessage({ id: prefix + ".origin_" + origin.kind })}
                {": "}
                {sourceLink(origin.sourceType, origin.sourceID)}
                {origin.viaType && origin.viaID ? (
                  <>
                    {" "}
                    {intl.formatMessage({ id: prefix + ".via" })}{" "}
                    {sourceLink(origin.viaType, origin.viaID)}
                  </>
                ) : null}
                {origin.sourceTagID && origin.sourceType !== "tag" ? (
                  <> ({sourceLink("tag", origin.sourceTagID)})</>
                ) : null}
              </li>
            ))}
          </ul>
        </li>
      ))}
    </ul>
  );
};

export const AssociationInheritanceTask: React.FC = () => {
  const intl = useIntl();
  const { adoptAssociationInheritance } = useSettings();
  const [current, setCurrent] = useState<InheritanceDefaults>();
  const [proposed, setProposed] = useState<InheritanceDefaults>({
    characters: true,
    artists: true,
    copyrights: true,
    tags: true,
  });
  const [review, setReview] = useState<Review>();
  const [loading, setLoading] = useState(true);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState("");
  const loadingController = useRef<AbortController>();
  const message = (key: string) =>
    intl.formatMessage({ id: prefix + "." + key });
  const running = active(review?.status);

  const receive = useCallback(
    (value: Review) => {
      setReview(value);
      setProposed(value.proposed);
      remember(value.reviewID);
      if (value.status === "applied") {
        setCurrent(value.proposed);
        adoptAssociationInheritance(value.proposed);
      }
    },
    [adoptAssociationInheritance]
  );

  const load = useCallback(async () => {
    loadingController.current?.abort();
    const controller = new AbortController();
    loadingController.current = controller;
    setLoading(true);
    setError("");
    try {
      const response = await fetch(endpoint, { signal: controller.signal });
      const defaults = await readResponse<{ current: InheritanceDefaults }>(
        response
      );
      setCurrent(defaults.current);
      setProposed(defaults.current);
      let retained: string | null = null;
      try {
        retained = sessionStorage.getItem(storageKey);
      } catch {
        // Reviews also work without browser persistence.
      }
      if (retained) {
        const saved = await fetch(
          endpoint + "?reviewID=" + encodeURIComponent(retained),
          { signal: controller.signal }
        );
        if (saved.status === 404) {
          remember();
          setReview(undefined);
        } else {
          receive(await readResponse<Review>(saved));
        }
      }
    } catch (e) {
      if (!controller.signal.aborted) {
        setError(e instanceof Error ? e.message : String(e));
      }
    } finally {
      if (!controller.signal.aborted) setLoading(false);
    }
  }, [receive]);

  useEffect(() => {
    void load();
    return () => loadingController.current?.abort();
  }, [load]);

  const reviewID = review?.reviewID;
  useEffect(() => {
    if (!reviewID || !running) return;
    const controller = new AbortController();
    let timer: ReturnType<typeof setTimeout> | undefined;
    async function poll() {
      try {
        const response = await fetch(
          endpoint + "?reviewID=" + encodeURIComponent(reviewID ?? ""),
          { signal: controller.signal }
        );
        if (response.status === 404) {
          remember();
          setReview(undefined);
          throw new Error(await response.text());
        }
        const value = await readResponse<Review>(response);
        receive(value);
        setError("");
        if (!active(value.status)) return;
      } catch (e) {
        if (controller.signal.aborted) return;
        setError(e instanceof Error ? e.message : String(e));
      }
      if (!controller.signal.aborted) {
        timer = setTimeout(() => void poll(), 1000);
      }
    }
    void poll();
    return () => {
      controller.abort();
      clearTimeout(timer);
    };
  }, [receive, reviewID, running]);

  async function perform(action: "preview" | "apply" | "cancel" | "discard") {
    setSubmitting(true);
    setError("");
    try {
      const response = await fetch(endpoint, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          action,
          reviewID,
          proposed: action === "preview" ? proposed : undefined,
        }),
      });
      if (action === "discard") {
        if (response.status !== 404) await readResponse<unknown>(response);
        remember();
        setReview(undefined);
        await load();
      } else {
        receive(await readResponse<Review>(response));
      }
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setSubmitting(false);
    }
  }

  const report = review?.report;
  return (
    <div id="association-inheritance-task">
      <h2>{message("heading")}</h2>
      <p>{message("description")}</p>
      <p>
        <Link to="/settings?tab=system#association-inheritance-settings">
          {message("shared_defaults")}
        </Link>
      </p>
      {flags.map((flag) => (
        <Form.Check
          key={flag}
          id={"review-inherit-" + flag}
          type="switch"
          label={intl.formatMessage({
            id: "config.association_inheritance." + flag + ".heading",
          })}
          checked={proposed[flag]}
          disabled={loading || submitting || !!review}
          onChange={(event) => {
            const checked = event.currentTarget.checked;
            setProposed((value) => ({ ...value, [flag]: checked }));
          }}
        />
      ))}
      {error || review?.error ? (
        <Alert variant="danger" className="mt-3">
          {error || review?.error}
          {!current && (
            <Button variant="link" onClick={() => void load()}>
              {message("retry")}
            </Button>
          )}
        </Alert>
      ) : null}
      <div className="mt-3 mb-3">
        {!review && (
          <Button
            variant="secondary"
            disabled={loading || submitting || !current}
            onClick={() => void perform("preview")}
          >
            {message("preview")}
          </Button>
        )}
        {review?.status === "ready" && (
          <Button
            variant="primary"
            disabled={submitting || !report || report.errorCount > 0}
            onClick={() => void perform("apply")}
          >
            {message("apply")}
          </Button>
        )}
        {running && (
          <Button
            variant="secondary"
            disabled={submitting || review?.status === "cancelling"}
            onClick={() => void perform("cancel")}
          >
            {message("cancel")}
          </Button>
        )}
        {review && !running && (
          <Button
            className="ml-2"
            variant="secondary"
            disabled={submitting}
            onClick={() => void perform("discard")}
          >
            {message("discard")}
          </Button>
        )}
      </div>
      {review && (
        <p>
          {message("status_" + review.status)}
          {" · "}
          {review.processed.toLocaleString()} / {review.total.toLocaleString()}
        </p>
      )}
      {running && (
        <ProgressBar
          animated
          now={
            review && review.total > 0
              ? (100 * review.processed) / review.total
              : 100
          }
        />
      )}
      {review?.status === "applied" && (
        <Alert variant="success">{message("applied")}</Alert>
      )}
      {report && (
        <>
          <p>
            {intl.formatMessage(
              { id: prefix + ".summary" },
              {
                images: report.imagesReviewed,
                videos: report.videosReviewed,
                affected: report.affectedMedia,
                errors: report.errorCount,
              }
            )}
          </p>
          <Table responsive size="sm" striped>
            <thead>
              <tr>
                <th>{message("domain")}</th>
                <th>{message("before")}</th>
                <th>{message("after")}</th>
                <th>{message("added")}</th>
                <th>{message("removed")}</th>
              </tr>
            </thead>
            <tbody>
              {domains.map((domain) => (
                <tr key={domain}>
                  <td>{message(domain)}</td>
                  <td>{report.domains[domain].beforeInherited}</td>
                  <td>{report.domains[domain].afterInherited}</td>
                  <td>{report.domains[domain].added}</td>
                  <td>{report.domains[domain].removed}</td>
                </tr>
              ))}
            </tbody>
          </Table>
          {report.errorSamples.map((item) => (
            <Alert variant="danger" key={item}>
              {item}
            </Alert>
          ))}
          {report.samples.length > 0 && (
            <>
              <p>{message("samples")}</p>
              <Accordion>
                {report.samples.map((sample) => {
                  const key = sample.mediaType + ":" + sample.mediaID;
                  return (
                    <Card key={key}>
                      <Card.Header>
                        <Accordion.Toggle
                          as={Button}
                          variant="link"
                          eventKey={key}
                        >
                          {sample.label ||
                            sample.mediaType + " #" + sample.mediaID}
                        </Accordion.Toggle>
                      </Card.Header>
                      <Accordion.Collapse eventKey={key}>
                        <Card.Body>
                          <Link
                            to={
                              "/" +
                              (sample.mediaType === "image"
                                ? "images"
                                : "scenes") +
                              "/" +
                              sample.mediaID
                            }
                          >
                            {message("open_media")}
                          </Link>
                          <Table responsive size="sm" className="mt-2">
                            <thead>
                              <tr>
                                <th>{message("current_memberships")}</th>
                                <th>{message("proposed_memberships")}</th>
                              </tr>
                            </thead>
                            <tbody>
                              <tr>
                                <td>
                                  <MembershipList items={sample.before} />
                                </td>
                                <td>
                                  <MembershipList items={sample.after} />
                                </td>
                              </tr>
                            </tbody>
                          </Table>
                          {sample.truncated && <p>{message("truncated")}</p>}
                        </Card.Body>
                      </Accordion.Collapse>
                    </Card>
                  );
                })}
              </Accordion>
            </>
          )}
        </>
      )}
    </div>
  );
};
