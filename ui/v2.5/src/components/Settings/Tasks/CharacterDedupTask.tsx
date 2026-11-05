import React, { useEffect, useRef, useState } from "react";
import { Alert, Button, Form, Table } from "react-bootstrap";
import { Link } from "react-router-dom";
import { TaskHelpHeading } from "./TaskHelpHeading";

interface Character {
  id: number;
  name: string;
}
interface Pair {
  leftId: number;
  rightId: number;
}
interface MergeGroup {
  destination: Character;
  sources: Character[];
  copyrights: Character[];
  reason: string;
}
interface MergePlan {
  fingerprint: string;
  scanned: number;
  groups: MergeGroup[];
  skipped: { character: Character; reason: string }[];
  ignored: { left: Character; right: Character }[];
}

const pageSize = 25;

export const CharacterDedupTask: React.FC = () => {
  const [plan, setPlan] = useState<MergePlan>();
  const [selected, setSelected] = useState<number[]>([]);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [queued, setQueued] = useState<number>();
  const [page, setPage] = useState(0);
  const [skipPage, setSkipPage] = useState(0);
  const [ignoredPage, setIgnoredPage] = useState(0);
  const request = useRef<AbortController>();
  useEffect(() => () => request.current?.abort(), []);

  async function perform(
    action: "preview" | "merge" | "exclude" | "restore",
    selectedIDs: number[] = selected,
    pairs: Pair[] = []
  ) {
    request.current?.abort();
    const controller = new AbortController();
    request.current = controller;
    setBusy(true);
    setError("");
    setQueued(undefined);
    try {
      const response = await fetch("character-dedup", {
        signal: controller.signal,
        ...(action !== "preview"
          ? {
              method: "POST",
              headers: { "Content-Type": "application/json" },
              body: JSON.stringify({
                fingerprint: plan?.fingerprint,
                action,
                selected: action === "restore" ? [] : selectedIDs,
                pairs,
              }),
            }
          : {}),
      });
      if (!response.ok) {
        throw new Error((await response.text()) || response.statusText);
      }
      const value = await response.json();
      if (controller.signal.aborted) return;
      if (action === "merge") {
        setQueued((value as { jobID: number }).jobID);
        setPlan(undefined);
      } else {
        setPlan(value as MergePlan);
        setPage(0);
        setSkipPage(0);
        setIgnoredPage(0);
      }
      setSelected([]);
    } catch (e) {
      if (!controller.signal.aborted) {
        setError(e instanceof Error ? e.message : String(e));
        setPlan(undefined);
        setSelected([]);
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
          disabled={busy || current === 0}
          onClick={() => change(current - 1)}
        >
          Previous
        </Button>
        <span className="mx-2">
          Page {current + 1} of {Math.ceil(total / pageSize)}
        </span>
        <Button
          variant="secondary"
          disabled={busy || (current + 1) * pageSize >= total}
          onClick={() => change(current + 1)}
        >
          Next
        </Button>
      </div>
    );
  }

  function toggleSelection(id: number, checked: boolean) {
    setSelected((previous) =>
      checked
        ? previous.includes(id)
          ? previous
          : [...previous, id]
        : previous.filter((value) => value !== id)
    );
  }

  return (
    <section
      id="character-dedup-task"
      aria-labelledby="character-dedup-heading"
    >
      <TaskHelpHeading
        id="character-dedup-heading"
        title="Merge duplicate Characters"
      />
      <Button
        variant="secondary"
        disabled={busy}
        onClick={() => void perform("preview")}
      >
        {busy ? "Working…" : "Preview Character merges"}
      </Button>
      {plan && plan.groups.length > 0 && (
        <div className="d-flex flex-wrap align-items-center mt-3 mb-2">
          <Form.Check
            id="character-merges-select-all"
            type="checkbox"
            className="mr-3"
            label={`Select all ${plan.groups.length}`}
            disabled={busy}
            checked={selected.length === plan.groups.length}
            onChange={(event) =>
              setSelected(
                event.currentTarget.checked
                  ? plan.groups.map((group) => group.destination.id)
                  : []
              )
            }
          />
          <Button
            variant="danger"
            className="mr-2 mb-2"
            disabled={busy || selected.length === 0}
            onClick={() => void perform("merge")}
          >
            Merge {selected.length} selected groups
          </Button>
          <Button
            variant="outline-secondary"
            className="mb-2"
            disabled={busy || selected.length === 0}
            onClick={() => void perform("exclude")}
          >
            Mark selected as not duplicates
          </Button>
        </div>
      )}
      {error && (
        <Alert variant="danger" className="mt-3" role="alert">
          {error}
        </Alert>
      )}
      {queued !== undefined && (
        <Alert variant="info" className="mt-3" role="status">
          Merge job #{queued} queued. Follow progress or cancel in the job queue
          above. If the catalogue changed, preview again.
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
                  <th>Select</th>
                  <th>Keep</th>
                  <th>Merge into this Character</th>
                  <th>Copyrights</th>
                  <th>Match</th>
                  <th>Decision</th>
                </tr>
              </thead>
              <tbody>
                {plan.groups
                  .slice(page * pageSize, (page + 1) * pageSize)
                  .map((group) => (
                    <tr key={group.destination.id}>
                      <td>
                        <Form.Check
                          type="checkbox"
                          id={`character-merge-${group.destination.id}`}
                          aria-label={`Select ${group.destination.name} merge group`}
                          checked={selected.includes(group.destination.id)}
                          disabled={busy}
                          onChange={(event) =>
                            toggleSelection(
                              group.destination.id,
                              event.currentTarget.checked
                            )
                          }
                        />
                      </td>
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
                      <td>
                        <Button
                          variant="link"
                          size="sm"
                          disabled={busy}
                          onClick={() =>
                            void perform("exclude", [group.destination.id])
                          }
                        >
                          Not duplicate
                        </Button>
                      </td>
                    </tr>
                  ))}
              </tbody>
            </Table>
          )}
          {pages(page, plan.groups.length, setPage)}
          {plan.ignored.length > 0 && (
            <details>
              <summary>Not duplicates ({plan.ignored.length})</summary>
              <Table responsive size="sm" className="mt-2">
                <thead>
                  <tr>
                    <th>Character</th>
                    <th>Not the same as</th>
                    <th>Decision</th>
                  </tr>
                </thead>
                <tbody>
                  {plan.ignored
                    .slice(ignoredPage * pageSize, (ignoredPage + 1) * pageSize)
                    .map(({ left, right }) => (
                      <tr key={`${left.id}:${right.id}`}>
                        <td>
                          <Link to={`/performers/${left.id}`}>{left.name}</Link>
                        </td>
                        <td>
                          <Link to={`/performers/${right.id}`}>
                            {right.name}
                          </Link>
                        </td>
                        <td>
                          <Button
                            variant="link"
                            size="sm"
                            disabled={busy}
                            onClick={() =>
                              void perform(
                                "restore",
                                [],
                                [{ leftId: left.id, rightId: right.id }]
                              )
                            }
                          >
                            Suggest again
                          </Button>
                        </td>
                      </tr>
                    ))}
                </tbody>
              </Table>
              {pages(ignoredPage, plan.ignored.length, setIgnoredPage)}
            </details>
          )}
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
