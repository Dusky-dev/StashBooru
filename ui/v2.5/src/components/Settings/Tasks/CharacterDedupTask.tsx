import React, { useEffect, useRef, useState } from "react";
import { Alert, Button, Table } from "react-bootstrap";
import { Link } from "react-router-dom";

interface Character {
  id: number;
  name: string;
}
interface MergePlan {
  fingerprint: string;
  scanned: number;
  groups: {
    destination: Character;
    sources: Character[];
    copyrights: Character[];
    reason: string;
  }[];
  skipped: { character: Character; reason: string }[];
}

const pageSize = 25;
export const CharacterDedupTask: React.FC = () => {
  const [plan, setPlan] = useState<MergePlan>();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [queued, setQueued] = useState<number>();
  const [page, setPage] = useState(0);
  const [skipPage, setSkipPage] = useState(0);
  const request = useRef<AbortController>();
  useEffect(() => () => request.current?.abort(), []);

  async function perform(apply: boolean) {
    request.current?.abort();
    const controller = new AbortController();
    request.current = controller;
    setBusy(true);
    setError("");
    setQueued(undefined);
    try {
      const response = await fetch("character-dedup", {
        signal: controller.signal,
        ...(apply
          ? {
              method: "POST",
              headers: { "Content-Type": "application/json" },
              body: JSON.stringify({ fingerprint: plan?.fingerprint }),
            }
          : {}),
      });
      if (!response.ok) {
        throw new Error((await response.text()) || response.statusText);
      }
      const value = await response.json();
      if (controller.signal.aborted) return;
      if (apply) {
        setQueued((value as { jobID: number }).jobID);
        setPlan(undefined);
      } else {
        setPlan(value as MergePlan);
        setPage(0);
        setSkipPage(0);
      }
    } catch (e) {
      if (!controller.signal.aborted) {
        setError(e instanceof Error ? e.message : String(e));
        setPlan(undefined);
      }
    } finally {
      if (!controller.signal.aborted) setBusy(false);
    }
  }

  function pages(current: number, total: number, change: (n: number) => void) {
    if (total <= pageSize) return null;
    return (
      <div className="d-flex align-items-center mb-3">
        <Button
          variant="secondary"
          disabled={current === 0}
          onClick={() => change(current - 1)}
        >
          Previous
        </Button>
        <span className="mx-2">
          Page {current + 1} of {Math.ceil(total / pageSize)}
        </span>
        <Button
          variant="secondary"
          disabled={(current + 1) * pageSize >= total}
          onClick={() => change(current + 1)}
        >
          Next
        </Button>
      </div>
    );
  }

  return (
    <section
      id="character-dedup-task"
      aria-labelledby="character-dedup-heading"
    >
      <h2 id="character-dedup-heading">Merge duplicate Characters</h2>
      <p>
        Find matching full names, swapped first and last names, and unique short
        names within the same directly assigned Copyrights. Missing Copyrights,
        ambiguous names, variants and conflicting profiles stay separate.
      </p>
      <p>
        Preview first, then merge all eligible groups in one cancellable job.
        Media links and compatible profile data are preserved. Former names
        become Copyright-qualified aliases. Check the surviving name and
        Copyrights before applying; matching names and Copyrights cannot prove
        identity.
      </p>
      <Button
        variant="secondary"
        disabled={busy}
        onClick={() => void perform(false)}
      >
        {busy ? "Working…" : "Preview Character merges"}
      </Button>
      {plan && plan.groups.length > 0 && (
        <Button
          variant="danger"
          className="ml-2 mt-2 mt-sm-0"
          disabled={busy}
          onClick={() => void perform(true)}
        >
          Merge {plan.groups.length} reviewed groups
        </Button>
      )}
      {error && (
        <Alert variant="danger" className="mt-3" role="alert">
          {error}
        </Alert>
      )}
      {queued !== undefined && (
        <Alert variant="info" className="mt-3" role="status">
          Merge job #{queued} queued. Follow progress or cancel in the job queue
          above. If the catalogue changed, the job will ask you to preview
          again.
        </Alert>
      )}
      {plan && (
        <>
          <p className="mt-3" role="status">
            {plan.scanned} Characters checked · {plan.groups.length} eligible
            groups · {plan.skipped.length} skipped Characters
          </p>
          {plan.groups.length === 0 && <p>No eligible duplicates found.</p>}
          {plan.groups.length > 0 && (
            <Table responsive striped size="sm">
              <thead>
                <tr>
                  <th>Keep</th>
                  <th>Merge into this Character</th>
                  <th>Copyrights</th>
                  <th>Match</th>
                </tr>
              </thead>
              <tbody>
                {plan.groups
                  .slice(page * pageSize, (page + 1) * pageSize)
                  .map((group) => (
                    <tr key={group.destination.id}>
                      <td>
                        <Link to={`/performers/${group.destination.id}`}>
                          {group.destination.name}
                        </Link>
                      </td>
                      <td>
                        {group.sources.map((source) => (
                          <div key={source.id}>
                            <Link to={`/performers/${source.id}`}>
                              {source.name}
                            </Link>
                          </div>
                        ))}
                      </td>
                      <td>
                        {group.copyrights.map((copyright) => (
                          <div key={copyright.id}>
                            <Link to={`/copyrights/${copyright.id}`}>
                              {copyright.name}
                            </Link>
                          </div>
                        ))}
                      </td>
                      <td>{group.reason}</td>
                    </tr>
                  ))}
              </tbody>
            </Table>
          )}
          {pages(page, plan.groups.length, setPage)}
          {plan.skipped.length > 0 && (
            <details>
              <summary>Skipped Characters ({plan.skipped.length})</summary>
              <ul className="mt-2">
                {plan.skipped
                  .slice(skipPage * pageSize, (skipPage + 1) * pageSize)
                  .map((item) => (
                    <li key={item.character.id}>
                      <Link to={`/performers/${item.character.id}`}>
                        {item.character.name}
                      </Link>
                      {": "}
                      {item.reason}
                    </li>
                  ))}
              </ul>
              {pages(skipPage, plan.skipped.length, setSkipPage)}
            </details>
          )}
        </>
      )}
    </section>
  );
};
