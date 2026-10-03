import { useCallback, useEffect, useRef, useState } from "react";
import { useApolloClient } from "@apollo/client";
import { Alert, Button, Form } from "react-bootstrap";
import * as GQL from "src/core/generated-graphql";
import { ModalComponent } from "../Shared/Modal";
import { parseMemberURL, stackKey, stackPath } from "./identity";
import { StackMember, memberThumbnail, memberTitle } from "./media";

interface Props {
  stackID?: string;
  selection?: GQL.MediaReferenceInput[];
  onClose: () => void;
}
const labels = [
  "",
  "Original",
  "Edited variant",
  "Converted copy",
  "Upscale",
  "Restoration",
];

export function VisualStackDialog({
  stackID,
  selection: providedSelection = [],
  onClose,
}: Props) {
  const client = useApolloClient();
  const selection = useRef(providedSelection);
  const [targetID, setTargetID] = useState(stackID);
  const [snapshot, setSnapshot] = useState<GQL.VisualStackDataFragment>();
  const [rows, setRows] = useState<StackMember[]>([]);
  const [title, setTitle] = useState("");
  const [representative, setRepresentative] = useState("");
  const [splitKeys, setSplitKeys] = useState<Set<string>>(new Set());
  const [merge, setMerge] = useState<GQL.VisualStackDataFragment>();
  const [memberURL, setMemberURL] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const query = GQL.useFindVisualStackQuery({
    variables: { id: targetID ?? "" },
    skip: !targetID,
    fetchPolicy: "network-only",
  });
  const [create] = GQL.useVisualStackCreateMutation();
  const [update] = GQL.useVisualStackUpdateMutation();
  const [split] = GQL.useVisualStackSplitMutation();
  const [mergeStacks] = GQL.useVisualStackMergeMutation();
  const [destroy] = GQL.useVisualStackDestroyMutation();
  const stack = query.data?.findVisualStack;

  const adopt = useCallback((value: GQL.VisualStackDataFragment) => {
    setSnapshot(value);
    setRows(value.members);
    setTitle(value.title);
    setRepresentative(value.representative);
    setSplitKeys(new Set());
    setMerge(undefined);
  }, []);
  useEffect(() => {
    if (stack && stack.id === targetID && snapshot?.id !== targetID)
      adopt(stack);
  }, [stack, targetID, snapshot?.id, adopt]);
  useEffect(() => {
    if (targetID) return;
    let disposed = false;
    async function load() {
      if (selection.current.length < 2 || selection.current.length > 200) {
        setError("Select between 2 and 200 members.");
        return;
      }
      setBusy(true);
      try {
        const loaded: StackMember[] = [];
        // Bound concurrent native reads; no full-library query or inference.
        for (let start = 0; start < selection.current.length; start += 8) {
          loaded.push(
            ...(await Promise.all(
              selection.current
                .slice(start, start + 8)
                .map(async (media, i) => {
                  const image =
                    media.kind === GQL.MediaKind.Image
                      ? (
                          await client.query({
                            query: GQL.FindImageDocument,
                            variables: { id: media.id },
                          })
                        ).data.findImage
                      : undefined;
                  const scene =
                    media.kind === GQL.MediaKind.Video
                      ? (
                          await client.query({
                            query: GQL.FindSceneDocument,
                            variables: { id: media.id },
                          })
                        ).data.findScene
                      : undefined;
                  if (!image && !scene)
                    throw new Error(
                      `Member ${stackKey(media)} does not exist.`
                    );
                  return {
                    id: stackKey(media),
                    media,
                    position: start + i,
                    label: "",
                    representative: false,
                    image,
                    scene,
                  };
                })
            ))
          );
        }
        if (!disposed) {
          setRows(loaded);
          setRepresentative(loaded[0].id);
        }
      } catch (e) {
        if (!disposed) setError(String(e));
      } finally {
        if (!disposed) setBusy(false);
      }
    }
    void load();
    return () => {
      disposed = true;
    };
  }, [targetID, client]);

  const groupedSelection = !targetID
    ? rows.filter((m) => m.image?.visual_stack || m.scene?.visual_stack)
    : [];
  const groupedStackIDs = Array.from(
    new Set(
      groupedSelection.map(
        (m) => (m.image?.visual_stack ?? m.scene?.visual_stack)!.id
      )
    )
  );
  const ungroupedSelection = !targetID
    ? rows.filter((m) => !m.image?.visual_stack && !m.scene?.visual_stack)
    : [];
  const members = rows.map((m) => ({
    media: { kind: m.media.kind, id: m.media.id },
    label: m.label,
  }));
  const rep = rows.find((m) => m.id === representative)?.media;
  const dirty =
    snapshot &&
    (title !== snapshot.title ||
      representative !== snapshot.representative ||
      JSON.stringify(members) !==
        JSON.stringify(
          snapshot.members.map((m) => ({
            media: { kind: m.media.kind, id: m.media.id },
            label: m.label,
          }))
        ));
  async function refresh() {
    for (const m of [
      ...rows,
      ...(snapshot?.members ?? []),
      ...(merge?.members ?? []),
    ]) {
      const id = client.cache.identify({
        __typename: m.media.kind === GQL.MediaKind.Image ? "Image" : "Scene",
        id: m.media.id,
      });
      if (id) client.cache.evict({ id, fieldName: "visual_stack" });
    }
    await client.refetchQueries({
      include: [
        "FindMedia",
        "FindImages",
        "FindScenes",
        "FindImage",
        "FindScene",
        "FindVisualStack",
        "FindVisualStackForMedia",
      ],
    });
  }
  async function run(action: () => Promise<void>) {
    setBusy(true);
    setError("");
    try {
      await action();
    } catch (e) {
      setError(String(e));
    } finally {
      setBusy(false);
    }
  }
  const save = () =>
    run(async () => {
      if (!rep) throw new Error("Choose a representative.");
      if (snapshot)
        await update({
          variables: {
            input: {
              id: snapshot.id,
              version: snapshot.version,
              title,
              members,
              representative: rep,
            },
          },
        });
      else
        await create({
          variables: { input: { title, members, representative: rep } },
        });
      await refresh();
      onClose();
    });
  function move(index: number, direction: -1 | 1) {
    const next = [...rows];
    [next[index], next[index + direction]] = [
      next[index + direction],
      next[index],
    ];
    setRows(next);
  }
  const add = () =>
    run(async () => {
      const media = parseMemberURL(memberURL);
      if (!media)
        throw new Error(
          "Enter an Image or Video URL, such as /images/123 or /scenes/123."
        );
      if (rows.some((m) => m.id === stackKey(media)))
        throw new Error("That member is already in this stack.");
      const found = (
        await client.query({
          query: GQL.FindVisualStackForMediaDocument,
          variables: { media },
          fetchPolicy: "network-only",
        })
      ).data.findVisualStackForMedia;
      if (found) {
        if (!snapshot)
          throw new Error(
            "This member already belongs to a stack. Manage or merge that stack."
          );
        setMerge(found);
        return;
      }
      if (rows.length >= 200)
        throw new Error("A stack can contain at most 200 members.");
      const image =
        media.kind === "IMAGE"
          ? (
              await client.query({
                query: GQL.FindImageDocument,
                variables: { id: media.id },
                fetchPolicy: "network-only",
              })
            ).data.findImage
          : undefined;
      const scene =
        media.kind === "VIDEO"
          ? (
              await client.query({
                query: GQL.FindSceneDocument,
                variables: { id: media.id },
                fetchPolicy: "network-only",
              })
            ).data.findScene
          : undefined;
      if (!image && !scene) throw new Error("Member does not exist.");
      setRows([
        ...rows,
        {
          id: stackKey(media),
          media: {
            ...media,
            kind:
              media.kind === "IMAGE"
                ? GQL.MediaKind.Image
                : GQL.MediaKind.Video,
          },
          position: rows.length,
          label: "",
          representative: false,
          image,
          scene,
        },
      ]);
      setMemberURL("");
    });

  const mergeSelectionInto = (targetStackID: string) =>
    run(async () => {
      const loaded: GQL.VisualStackDataFragment[] = await Promise.all(
        groupedStackIDs.map(async (id) => {
          const value: GQL.VisualStackDataFragment | undefined = (
            await client.query({
              query: GQL.FindVisualStackDocument,
              variables: { id },
              fetchPolicy: "network-only",
            })
          ).data.findVisualStack;
          if (!value) throw new Error(`Stack ${id} no longer exists.`);
          return value;
        })
      );
      const target = loaded.find((value) => value.id === targetStackID);
      if (!target) throw new Error("Target stack no longer exists.");

      const orderedStacks = [
        target,
        ...loaded.filter((value) => value.id !== target.id),
      ];
      const combinedCount =
        orderedStacks.reduce((sum, value) => sum + value.member_count, 0) +
        ungroupedSelection.length;
      if (combinedCount > 200)
        throw new Error("Merged stack would exceed 200 members.");

      let value = target;
      if (orderedStacks.length > 1) {
        const targetRepresentative = target.members.find(
          (member) => member.representative
        )?.media;
        if (!targetRepresentative)
          throw new Error("Target stack has no representative.");

        const mergedValue = (
          await mergeStacks({
            variables: {
              input: {
                stacks: orderedStacks.map((stack) => ({
                  id: stack.id,
                  version: stack.version,
                })),
                title: target.title,
                representative: {
                  kind: targetRepresentative.kind,
                  id: targetRepresentative.id,
                },
              },
            },
          })
        ).data?.visualStackMerge;
        if (!mergedValue)
          throw new Error("Stack merge did not return a stack.");
        value = mergedValue;
      }

      const existing = new Set(
        value.members.map((member) => stackKey(member.media))
      );
      const additions = ungroupedSelection.filter(
        (member) => !existing.has(member.id)
      );
      if (additions.length > 0) {
        const currentRepresentative = value.members.find(
          (member) => member.representative
        )?.media;
        if (!currentRepresentative)
          throw new Error("Target stack has no representative.");

        const updatedValue = (
          await update({
            variables: {
              input: {
                id: value.id,
                version: value.version,
                title: value.title,
                members: [
                  ...value.members.map((member) => ({
                    media: {
                      kind: member.media.kind,
                      id: member.media.id,
                    },
                    label: member.label,
                  })),
                  ...additions.map((member) => ({
                    media: {
                      kind: member.media.kind,
                      id: member.media.id,
                    },
                    label: member.label,
                  })),
                ],
                representative: {
                  kind: currentRepresentative.kind,
                  id: currentRepresentative.id,
                },
              },
            },
          })
        ).data?.visualStackUpdate;
        if (!updatedValue)
          throw new Error("Adding selected media did not return a stack.");
        value = updatedValue;
      }

      await refresh();
      setTargetID(value.id);
      adopt(value);
    });

  return (
    <ModalComponent
      show
      header={targetID ? "Manage stack" : "Create stack"}
      onHide={onClose}
      modalProps={{ size: "lg" }}
      cancel={{ onClick: onClose }}
      accept={{ text: snapshot ? "Save stack" : "Create stack", onClick: save }}
      isRunning={busy}
      disabled={Boolean(
        query.loading || query.error || !rep || groupedSelection.length
      )}
      leftFooterButtons={
        snapshot && (
          <Button
            variant="danger"
            disabled={busy}
            onClick={() =>
              run(async () => {
                await destroy({
                  variables: {
                    input: { id: snapshot.id, version: snapshot.version },
                  },
                });
                await refresh();
                onClose();
              })
            }
          >
            Unstack all (keep media)
          </Button>
        )
      }
    >
      <p>
        Grouping keeps each member’s file and metadata separate. Selection and
        file actions target individual members.
      </p>
      {(error || query.error) && (
        <Alert variant="danger">{error || query.error?.message}</Alert>
      )}
      {snapshot && (
        <Button
          size="sm"
          variant="secondary"
          disabled={busy}
          onClick={() =>
            run(async () => {
              const value = (await query.refetch()).data.findVisualStack;
              if (!value) throw new Error("Stack no longer exists.");
              adopt(value);
            })
          }
        >
          Reload (discard edits)
        </Button>
      )}
      {groupedSelection.length > 0 && (
        <Alert variant="warning">
          <div>
            Some selected members already belong to a stack. Manage an existing
            stack, or merge the full selection into one of them.
          </div>
          {groupedStackIDs.length > 1 && (
            <small className="d-block mt-1">
              Merging also includes every member of the selected existing
              stacks, not only the members currently selected.
            </small>
          )}
          <div className="visual-stack-selection-actions">
            {groupedStackIDs.map((id) => (
              <div className="visual-stack-selection-action" key={id}>
                <Button
                  size="sm"
                  variant="secondary"
                  disabled={busy}
                  onClick={() => {
                    setSnapshot(undefined);
                    setTargetID(id);
                  }}
                >
                  Manage stack {id}
                </Button>
                {(groupedStackIDs.length > 1 ||
                  ungroupedSelection.length > 0) && (
                  <Button
                    size="sm"
                    disabled={busy}
                    onClick={() => mergeSelectionInto(id)}
                  >
                    Merge selection into stack {id}
                  </Button>
                )}
              </div>
            ))}
          </div>
        </Alert>
      )}
      <Form.Group controlId="visual-stack-title">
        <Form.Label>Stack title (optional)</Form.Label>
        <Form.Control
          className="text-input"
          value={title}
          maxLength={200}
          disabled={busy}
          onChange={(e) => setTitle(e.target.value)}
        />
      </Form.Group>
      <div className="visual-stack-editor">
        {rows.map((m, i) => (
          <div
            className="visual-stack-editor-member"
            key={m.id}
            data-member-id={m.id}
          >
            <img src={memberThumbnail(m)} alt="" />
            <div className="visual-stack-member-info">
              <a href={stackPath(m.media)}>{memberTitle(m)}</a>
              <small>
                {m.media.kind === GQL.MediaKind.Image ? "Image" : "Video"} ·{" "}
                {m.media.id}
              </small>
              <Form.Control
                as="select"
                className="input-control"
                aria-label={`Relationship for ${m.id}`}
                value={m.label}
                disabled={busy}
                onChange={(e) =>
                  setRows(
                    rows.map((row) =>
                      row.id === m.id ? { ...row, label: e.target.value } : row
                    )
                  )
                }
              >
                {Array.from(new Set([...labels, m.label])).map((label) => (
                  <option key={label} value={label}>
                    {label || "No relationship label"}
                  </option>
                ))}
              </Form.Control>
            </div>
            <div className="visual-stack-member-controls">
              <Form.Check
                type="radio"
                name="stack-representative"
                label="Representative"
                aria-label={`Representative ${m.id}`}
                checked={representative === m.id}
                disabled={busy}
                onChange={() => setRepresentative(m.id)}
              />
              {snapshot && (
                <Form.Check
                  label="Split out"
                  aria-label={`Split ${m.id}`}
                  checked={splitKeys.has(m.id)}
                  disabled={busy}
                  onChange={(e) => {
                    const next = new Set(splitKeys);
                    if (e.target.checked) next.add(m.id);
                    else next.delete(m.id);
                    setSplitKeys(next);
                  }}
                />
              )}
              <Button
                size="sm"
                disabled={busy || i === 0}
                aria-label={`Move ${m.id} up`}
                onClick={() => move(i, -1)}
              >
                ↑
              </Button>
              <Button
                size="sm"
                disabled={busy || i === rows.length - 1}
                aria-label={`Move ${m.id} down`}
                onClick={() => move(i, 1)}
              >
                ↓
              </Button>
              <Button
                size="sm"
                variant="secondary"
                disabled={busy || rows.length <= (snapshot ? 1 : 2)}
                aria-label={`Remove ${m.id} from stack`}
                onClick={() => {
                  const next = rows.filter((row) => row.id !== m.id);
                  setRows(next);
                  if (representative === m.id) setRepresentative(next[0].id);
                  const selected = new Set(splitKeys);
                  selected.delete(m.id);
                  setSplitKeys(selected);
                }}
              >
                Remove
              </Button>
            </div>
          </div>
        ))}
      </div>
      <Form.Group controlId="visual-stack-member-url">
        <Form.Label>Add member by Image or Video URL</Form.Label>
        <Form.Control
          className="text-input"
          value={memberURL}
          placeholder="/images/123 or /scenes/123"
          disabled={busy}
          onChange={(e) => setMemberURL(e.target.value)}
        />
      </Form.Group>
      <Button disabled={busy || !memberURL} onClick={add}>
        Review member
      </Button>
      {snapshot && (
        <div className="mt-3">
          <Button
            disabled={
              busy ||
              Boolean(dirty) ||
              splitKeys.size < 2 ||
              splitKeys.size >= rows.length
            }
            onClick={() =>
              run(async () => {
                const moved = rows.filter((m) => splitKeys.has(m.id));
                await split({
                  variables: {
                    input: {
                      id: snapshot.id,
                      version: snapshot.version,
                      members: moved.map((m) => m.media),
                      representative:
                        moved.find((m) => m.id === representative)?.media ??
                        moved[0].media,
                      title: "",
                    },
                  },
                });
                await refresh();
                onClose();
              })
            }
          >
            Split selected members into a new stack
          </Button>
          <small className="d-block">
            Save edits first. Select at least two members and leave one in this
            stack.
          </small>
        </div>
      )}
      {snapshot && merge && (
        <Alert variant="info" className="mt-3">
          Merge all {merge.member_count} members of “
          {merge.title || `Stack ${merge.id}`}” into this stack? All files
          remain.
          <Button
            className="ml-2"
            disabled={
              busy || Boolean(dirty) || rows.length + merge.member_count > 200
            }
            onClick={() =>
              run(async () => {
                if (!rep) throw new Error("Choose a representative.");
                const value = (
                  await mergeStacks({
                    variables: {
                      input: {
                        stacks: [
                          { id: snapshot.id, version: snapshot.version },
                          { id: merge.id, version: merge.version },
                        ],
                        title,
                        representative: rep,
                      },
                    },
                  })
                ).data?.visualStackMerge;
                await refresh();
                if (value) adopt(value);
              })
            }
          >
            Merge stacks (keep all members)
          </Button>
        </Alert>
      )}
    </ModalComponent>
  );
}
