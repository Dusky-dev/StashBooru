import assert from "node:assert/strict";
import test from "node:test";
import {
  dispatchMediaUpdates,
  mediaConversionTargets,
  splitMediaSelection,
} from "../src/components/Media/mediaSelection.ts";

const image = { id: "image:123", image: { id: "123" } };
const scene = { id: "scene:123", scene: { id: "123" } };

test("equal native IDs remain separate media selections and conversion targets", () => {
  assert.deepEqual(splitMediaSelection([image, scene, image]), {
    imageIDs: ["123"],
    sceneIDs: ["123"],
  });
  assert.deepEqual(mediaConversionTargets([image, scene]), [
    { kind: "image", id: 123 },
    { kind: "scene", id: 123 },
  ]);
});

test("mixed edits dispatch only the selected IDs to each native writer", async () => {
  const calls: unknown[] = [];
  const fields = { title: "shared metadata", organized: true };
  const outcomes = await dispatchMediaUpdates(
    [image, scene, { id: "image:456", image: { id: "456" } }],
    fields,
    async (ids, input) => {
      calls.push({ kind: "image", ids, input });
    },
    async (ids, input) => {
      calls.push({ kind: "scene", ids, input });
    }
  );
  assert.deepEqual(calls, [
    { kind: "image", ids: ["123", "456"], input: fields },
    { kind: "scene", ids: ["123"], input: fields },
  ]);
  assert.ok(outcomes.every((outcome) => outcome.status === "fulfilled"));
});

test("image-only and empty selections never invoke an unrelated writer", async () => {
  let imageCalls = 0;
  const updateImages = async () => {
    imageCalls++;
  };
  const updateScenes = async () => {
    assert.fail("Videos were not selected");
  };
  await dispatchMediaUpdates([image], {}, updateImages, updateScenes);
  await dispatchMediaUpdates([], {}, updateImages, updateScenes);
  assert.equal(imageCalls, 1);
});

test("a failed native mutation preserves the other type's successful outcome", async () => {
  const failure = new Error("Video update failed");
  const outcomes = await dispatchMediaUpdates(
    [image, scene],
    {},
    async () => "updated image",
    async () => {
      throw failure;
    }
  );
  assert.deepEqual(outcomes, [
    { status: "fulfilled", value: "updated image" },
    { status: "rejected", reason: failure },
  ]);
});

test("invalid typed identities cannot dispatch a native mutation", async () => {
  const invalid = { id: "image:123", scene: { id: "123" } };
  assert.throws(
    () => splitMediaSelection([invalid]),
    /Invalid media selection/
  );
  await assert.rejects(
    dispatchMediaUpdates(
      [invalid],
      {},
      async () => assert.fail(),
      async () => assert.fail()
    ),
    /Invalid media selection/
  );
});
