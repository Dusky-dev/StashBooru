import { useState } from "react";
import ReactDOM from "react-dom";
import {
  ApolloClient,
  ApolloLink,
  ApolloProvider,
  InMemoryCache,
  Observable,
} from "@apollo/client";
import { GraphQLError } from "graphql";
import { BrowserRouter } from "react-router-dom";
import { Button } from "react-bootstrap";
import { IntlProvider } from "react-intl";
import Mousetrap from "mousetrap";
import MousetrapPause from "mousetrap-pause";
import * as GQL from "../../../src/core/generated-graphql";
import { JobQueueProvider } from "../../../src/hooks/JobQueue";
import { JobTable } from "../../../src/components/Settings/Tasks/JobTable";
import { VisualStackDialog } from "../../../src/components/VisualStacks/VisualStackDialog";
import { DetailStackFilmstrip } from "../../../src/components/VisualStacks/StackFilmstrip";
import { ConfigurationProvider } from "../../../src/hooks/Config";
import { ToastProvider } from "../../../src/hooks/Toast";
import { LightboxProvider } from "../../../src/hooks/Lightbox/context";
import { useLightbox } from "../../../src/hooks/Lightbox/hooks";
import "../../../src/index.scss";

MousetrapPause(Mousetrap);
const thumbnail = (id: string) =>
  `data:image/svg+xml,${encodeURIComponent(`<svg xmlns="http://www.w3.org/2000/svg" width="320" height="240"><rect width="320" height="240" fill="${id === "1" ? "#31606a" : "#635286"}"/><circle cx="${80 + Number(id) * 35}" cy="120" r="40" fill="#bfe4f2"/></svg>`)}`;
const images: GQL.SlimImageDataFragment[] = ["1", "2", "3"].map((id) => ({
  __typename: "Image",
  id,
  title: `Image ${id}`,
  code: null,
  date: null,
  urls: [],
  details: "",
  photographer: null,
  rating100: null,
  organized: false,
  o_counter: 0,
  visual_stack: null,
  paths: {
    __typename: "ImagePathsType",
    image: thumbnail(id),
    thumbnail: thumbnail(id),
    preview: null,
  },
  galleries: [],
  studio: null,
  tags: [],
  performers: [],
  visual_files: [
    {
      __typename: "ImageFile",
      id,
      path: `/fixture/image-${id}.png`,
      size: 1000 + Number(id),
      frame_count: 1,
      mod_time: "2026-10-04T12:00:00Z",
      width: 320,
      height: 240,
      frame_rate: 0,
      format: "png",
      fingerprints: [],
    },
  ],
}));
let version = 1;
let rep = "image:1";
let members = [
  { kind: GQL.MediaKind.Image, id: "1", label: "Original" },
  { kind: GQL.MediaKind.Video, id: "1", label: "Converted copy" },
  { kind: GQL.MediaKind.Image, id: "2", label: "Restoration" },
  { kind: GQL.MediaKind.Image, id: "3", label: "Edited variant" },
];
const key = (media: GQL.MediaReferenceInput) =>
  `${media.kind === GQL.MediaKind.Image ? "image" : "scene"}:${media.id}`;
function stack(): GQL.VisualStackDataFragment {
  const summary = {
    __typename: "VisualStack" as const,
    id: "1",
    title: "Original and variants",
    version,
    member_count: members.length,
    representative: rep,
  };
  return {
    ...summary,
    members: members.map((member, position) => ({
      __typename: "VisualStackMember",
      id: key(member),
      media: { __typename: "MediaReference", kind: member.kind, id: member.id },
      position,
      label: member.label,
      representative: key(member) === rep,
      image:
        member.kind === GQL.MediaKind.Image
          ? {
              ...images.find((image) => image.id === member.id)!,
              visual_stack: summary,
            }
          : null,
      scene:
        member.kind === GQL.MediaKind.Video
          ? {
              __typename: "Scene",
              id: member.id,
              title: `Video ${member.id}`,
              code: null,
              details: "",
              director: null,
              urls: [],
              date: null,
              production_date: null,
              rating100: null,
              o_counter: 0,
              organized: false,
              interactive: false,
              interactive_speed: null,
              resume_time: 0,
              play_duration: 0,
              play_count: 0,
              files: [],
              visual_stack: summary,
              paths: {
                __typename: "ScenePathsType",
                screenshot: thumbnail(member.id),
                preview: null,
                stream: null,
                webp: null,
                vtt: null,
                sprite: null,
                funscript: null,
                interactive_heatmap: null,
                caption: null,
              },
              scene_markers: [],
              galleries: [],
              studio: null,
              groups: [],
              tags: [],
              performers: [],
              stash_ids: [],
            }
          : null,
    })),
  };
}
const subscriptions = new Set<(data: GQL.JobsSubscribeSubscription) => void>();
let queue: GQL.JobDataFragment[] = [];
let heldQuery: (() => void) | undefined;
let heldMutation: (() => void) | undefined;
const fixture = {
  holdQueue: false,
  holdMutation: false,
  conflict: false,
  mutations: [] as GQL.VisualStackUpdateInput[],
  emitJob(job: GQL.JobDataFragment, type: GQL.JobStatusUpdateType) {
    for (const next of subscriptions)
      next({ jobsSubscribe: { __typename: "JobStatusUpdate", job, type } });
  },
  setQueue(jobs: GQL.JobDataFragment[]) {
    queue = jobs;
  },
  releaseQueue() {
    fixture.holdQueue = false;
    heldQuery?.();
    heldQuery = undefined;
  },
  releaseMutation() {
    fixture.holdMutation = false;
    heldMutation?.();
    heldMutation = undefined;
  },
  snapshot: stack,
  refreshMetadata() {
    client.cache.modify({
      id: "Image:1",
      fields: { title: () => "Refreshed original" },
    });
  },
  openOther: () => {},
};
Object.assign(window, { qolFixture: fixture });
const client = new ApolloClient({
  cache: new InMemoryCache({
    typePolicies: { MediaReference: { keyFields: ["kind", "id"] } },
    possibleTypes: {
      VisualFile: ["ImageFile", "VideoFile"],
      BaseFile: ["ImageFile", "VideoFile", "GalleryFile"],
    },
  }),
  link: new ApolloLink(
    (operation) =>
      new Observable((observer) => {
        const send = (data: Record<string, unknown>) => {
          observer.next({ data });
          observer.complete();
        };
        if (operation.operationName === "JobsSubscribe") {
          const next = (data: GQL.JobsSubscribeSubscription) =>
            observer.next({ data });
          subscriptions.add(next);
          return () => subscriptions.delete(next);
        }
        if (operation.operationName === "JobQueue") {
          const snapshot = [...queue];
          const complete = () => send({ jobQueue: snapshot });
          if (fixture.holdQueue) heldQuery = complete;
          else complete();
        } else if (operation.operationName === "FindVisualStackForMedia")
          send({ findVisualStackForMedia: stack() });
        else if (operation.operationName === "FindVisualStack")
          send({ findVisualStack: stack() });
        else if (operation.operationName === "VisualStackUpdate") {
          const input = operation.variables.input as GQL.VisualStackUpdateInput;
          fixture.mutations.push(input);
          const complete = () => {
            if (fixture.conflict || input.version !== version) {
              observer.next({
                errors: [
                  new GraphQLError("Stack changed. Reload and try again."),
                ],
              });
              observer.complete();
              return;
            }
            members = input.members.map(({ media, label }) => ({
              ...media,
              label: label ?? "",
            }));
            rep = key(input.representative);
            version += 1;
            send({ visualStackUpdate: stack() });
          };
          if (fixture.holdMutation) heldMutation = complete;
          else complete();
        } else send({});
      })
  ),
});

const configuration = {
  interface: { imageLightbox: { disableAnimation: true } },
  ui: {},
} as GQL.ConfigDataFragment;
const previewImages = [images[1]];
const otherImages = [images[2]];
function OtherCaller() {
  const show = useLightbox({ images: otherImages, showNavigation: false });
  fixture.openOther = () => show({ initialIndex: 0 });
  return null;
}
function Fixture() {
  const [tasks, setTasks] = useState(true);
  const [editor, setEditor] = useState(false);
  const [kind, setKind] = useState(GQL.MediaKind.Image);
  const show = useLightbox({ images: previewImages, showNavigation: false });
  return (
    <main className="p-3">
      <h1>Isolated jobs and stacks fixture</h1>
      <div className="d-flex flex-wrap mb-3" style={{ gap: "0.5rem" }}>
        <Button onClick={() => setTasks(!tasks)}>Toggle tasks</Button>
        <Button onClick={() => setEditor(true)}>Open editor</Button>
        <Button onClick={() => show({ initialIndex: 0 })}>Open preview</Button>
        <Button
          onClick={() =>
            setKind(
              kind === GQL.MediaKind.Image
                ? GQL.MediaKind.Video
                : GQL.MediaKind.Image
            )
          }
        >
          Switch detail kind
        </Button>
      </div>
      {tasks && <JobTable />}
      <DetailStackFilmstrip
        kind={kind}
        id={kind === GQL.MediaKind.Image ? "2" : "1"}
      />
      {editor && (
        <VisualStackDialog stackID="1" onClose={() => setEditor(false)} />
      )}
      <OtherCaller />
    </main>
  );
}
ReactDOM.render(
  <ApolloProvider client={client}>
    <BrowserRouter>
      <IntlProvider
        locale="en"
        messages={{ "config.tasks.empty_queue": "No jobs queued." }}
      >
        <ConfigurationProvider configuration={configuration}>
          <ToastProvider>
            <JobQueueProvider>
              <LightboxProvider>
                <Fixture />
              </LightboxProvider>
            </JobQueueProvider>
          </ToastProvider>
        </ConfigurationProvider>
      </IntlProvider>
    </BrowserRouter>
  </ApolloProvider>,
  document.getElementById("root")
);
