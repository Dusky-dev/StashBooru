import assert from "node:assert/strict";
import test from "node:test";
import {
  dispatchMediaUpdates,
  mediaConversionTargets,
  splitMediaSelection,
  summarizeMediaUpdates,
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

test("retry and close retain only unconfirmed typed media", async () => {
  const other = { id: "image:456", image: { id: "456" } };
  const selected = [image, scene, other];
  const edits = { title: "reviewed title" };
  const first = await dispatchMediaUpdates(
    selected,
    edits,
    async () => ["123", "999"],
    async () => {
      throw new Error("Video service unavailable");
    }
  );
  const partial = summarizeMediaUpdates(selected, first);
  assert.deepEqual(partial.confirmed, ["image:123"]);
  assert.match(partial.errors[0], /Images: Stash did not confirm 1 of 2/);
  assert.match(partial.errors[1], /Videos: Video service unavailable/);
  const remaining = selected.filter(
    (item) => !partial.confirmed.includes(item.id)
  );
  assert.deepEqual(
    remaining.map((item) => item.id),
    ["scene:123", "image:456"]
  );

  const second = await dispatchMediaUpdates(
    remaining,
    edits,
    async (ids) => {
      assert.deepEqual(ids, ["456"]);
      return ids;
    },
    async (ids) => {
      assert.deepEqual(ids, ["123"]);
      return ids;
    }
  );
  assert.deepEqual(summarizeMediaUpdates(remaining, second), {
    confirmed: ["image:456", "scene:123"],
    errors: [],
  });
});

test("a synchronous writer error does not skip the other native type", async () => {
  const outcomes = await dispatchMediaUpdates(
    [image, scene],
    {},
    () => {
      throw new Error("Image preparation failed");
    },
    async () => ["123"]
  );
  assert.deepEqual(summarizeMediaUpdates([image, scene], outcomes), {
    confirmed: ["scene:123"],
    errors: ["Images: Image preparation failed"],
  });
});
