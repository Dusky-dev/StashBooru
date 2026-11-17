import React, { useState } from "react";
import { Alert, Badge, Button, Table } from "react-bootstrap";
import { defineMessages, FormattedMessage, useIntl } from "react-intl";
import { Link } from "react-router-dom";
import { Setting } from "./Inputs";

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

type EntityKind = "character" | "artist" | "copyright" | "tag";

const entityPaths: Record<EntityKind, string> = {
  character: "/performers",
  artist: "/studios",
  copyright: "/copyrights",
  tag: "/tags",
};

const mediaPaths: Record<string, string> = {
  image: "/images",
  video: "/scenes",
};

const messages = defineMessages({
  heading: { id: "config.tools.health.heading" },
  description: { id: "config.tools.health.description" },
  scan: { id: "config.tools.health.scan" },
  scanning: { id: "config.tools.health.scanning" },
  failed: { id: "config.tools.health.failed" },
  findings: { id: "config.tools.health.findings" },
  none: { id: "config.tools.health.none" },
  readOnly: { id: "config.tools.health.read_only" },
  item: { id: "config.tools.health.item" },
  details: { id: "config.tools.health.details" },
  missing: { id: "config.tools.health.missing" },
});

// Plain-language title and next step for each backend diagnostic code.
// Unknown codes fall back to the raw code so new diagnostics stay visible.
const knownCodes = [
  "duplicate-canonical-name",
  "same-name-character-context",
  "legacy-copyright-tag-collision",
  "legacy-tag-migration-ambiguous",
  "legacy-artist-missing-native",
  "native-artist-missing-legacy",
  "legacy-artist-mismatch",
  "orphan-character-relation",
  "orphan-tag-relation",
  "orphan-legacy-artist",
];

function codeKey(code: string) {
  return code.replace(/-/g, "_");
}

// Entities that no longer exist cannot be linked; show their IDs only.
const orphanCodes = new Set([
  "orphan-character-relation",
  "orphan-tag-relation",
  "orphan-legacy-artist",
]);

function entityKindFor(
  finding: MetadataHealthFinding,
  index: number
): EntityKind | null {
  switch (finding.code) {
    case "legacy-copyright-tag-collision":
      return index === 0 ? "tag" : "copyright";
    case "legacy-tag-migration-ambiguous":
    case "orphan-tag-relation":
      return "tag";
    case "orphan-character-relation":
      return "character";
    case "legacy-artist-missing-native":
    case "native-artist-missing-legacy":
    case "legacy-artist-mismatch":
    case "orphan-legacy-artist":
      return "artist";
    default:
      return finding.kind in entityPaths ? (finding.kind as EntityKind) : null;
  }
}

const FindingItems: React.FC<{ finding: MetadataHealthFinding }> = ({
  finding,
}) => {
  const intl = useIntl();
  const items: React.ReactNode[] = [];
  const mediaPath = mediaPaths[finding.kind];
  if (finding.mediaID && mediaPath) {
    items.push(
      <Link key="media" to={`${mediaPath}/${finding.mediaID}`}>
        {intl.formatMessage({
          id: finding.kind === "image" ? "image" : "scene",
        })}{" "}
        #{finding.mediaID}
      </Link>
    );
  }
  finding.entityIDs?.forEach((id, index) => {
    const kind = entityKindFor(finding, index);
    const label = kind
      ? `${intl.formatMessage({
          id:
            kind === "character"
              ? "performer"
              : kind === "artist"
                ? "studio"
                : kind,
        })} #${id}`
      : `#${id}`;
    if (!kind || orphanCodes.has(finding.code)) {
      items.push(
        <span key={id} className="text-muted">
          {label}
          {orphanCodes.has(finding.code)
            ? ` (${intl.formatMessage(messages.missing)})`
            : ""}
        </span>
      );
    } else {
      items.push(
        <Link key={id} to={`${entityPaths[kind]}/${id}`}>
          {label}
        </Link>
      );
    }
  });
  if (finding.value) {
    items.push(<em key="value">“{finding.value}”</em>);
  }
  if (items.length === 0) return <>—</>;
  return (
    <>
      {items.map((item, index) => (
        <React.Fragment key={index}>
          {index > 0 && ", "}
          {item}
        </React.Fragment>
      ))}
    </>
  );
};

const FindingGroup: React.FC<{
  code: string;
  findings: MetadataHealthFinding[];
}> = ({ code, findings }) => {
  const intl = useIntl();
  const known = knownCodes.includes(code);
  const key = codeKey(code);
  return (
    <div className="mb-3">
      <h6 className="mb-1">
        {known
          ? intl.formatMessage({ id: `config.tools.health.codes.${key}` })
          : code}{" "}
        <Badge variant="secondary">{findings.length}</Badge>
      </h6>
      {known && (
        <div className="text-muted small mb-2">
          {intl.formatMessage({
            id: `config.tools.health.codes.${key}_help`,
          })}
        </div>
      )}
      <Table responsive size="sm">
        <thead>
          <tr>
            <th>{intl.formatMessage(messages.item)}</th>
            <th>{intl.formatMessage(messages.details)}</th>
          </tr>
        </thead>
        <tbody>
          {findings.map((finding, index) => (
            <tr
              key={`${finding.kind}-${finding.mediaID ?? finding.value ?? ""}-${index}`}
            >
              <td>
                <FindingItems finding={finding} />
              </td>
              <td>{finding.detail || "—"}</td>
            </tr>
          ))}
        </tbody>
      </Table>
    </div>
  );
};

export const MetadataHealth: React.FC = () => {
  const intl = useIntl();
  const [health, setHealth] = useState<MetadataHealthResponse>();
  const [error, setError] = useState<string>();
  const [loading, setLoading] = useState(false);

  async function scan() {
    setLoading(true);
    setError(undefined);
    try {
      const response = await fetch("image/metadata-health");
      if (!response.ok) {
        const message = (await response.text()).trim();
        throw new Error(
          message ||
            intl.formatMessage(messages.failed, { status: response.status })
        );
      }
      setHealth((await response.json()) as MetadataHealthResponse);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : String(cause));
    } finally {
      setLoading(false);
    }
  }

  const groups = new Map<string, MetadataHealthFinding[]>();
  health?.findings.forEach((finding) => {
    groups.set(finding.code, [...(groups.get(finding.code) ?? []), finding]);
  });
  const ordered = [...groups.entries()].sort(([a], [b]) => {
    const ia = knownCodes.indexOf(a);
    const ib = knownCodes.indexOf(b);
    if (ia !== ib)
      return (
        (ia < 0 ? knownCodes.length : ia) - (ib < 0 ? knownCodes.length : ib)
      );
    return a.localeCompare(b);
  });

  return (
    <>
      <Setting
        heading={intl.formatMessage(messages.heading)}
        subHeading={intl.formatMessage(messages.description)}
      >
        <Button
          variant="secondary"
          disabled={loading}
          onClick={() => void scan()}
        >
          {intl.formatMessage(loading ? messages.scanning : messages.scan)}
        </Button>
      </Setting>
      {error ? (
        <Alert className="mx-3" variant="danger">
          {error}
        </Alert>
      ) : null}
      {health ? (
        <div className="px-3 pb-3">
          <div className="mb-3">
            <strong>
              <FormattedMessage
                {...messages.findings}
                values={{ count: health.findings.length }}
              />
            </strong>
          </div>
          {health.findings.length === 0 ? (
            <Alert variant="success">{intl.formatMessage(messages.none)}</Alert>
          ) : (
            ordered.map(([code, findings]) => (
              <FindingGroup key={code} code={code} findings={findings} />
            ))
          )}
          <div className="text-muted">
            {intl.formatMessage(messages.readOnly)}
          </div>
        </div>
      ) : null}
    </>
  );
};
