import ReactDOM from "react-dom";
import {
  ApolloClient,
  ApolloLink,
  ApolloProvider,
  InMemoryCache,
  Observable,
} from "@apollo/client";
import { BrowserRouter } from "react-router-dom";
import { IntlProvider } from "react-intl";
import * as GQL from "../../../src/core/generated-graphql";
import { ConfigurationProvider } from "../../../src/hooks/Config";
import { ToastProvider } from "../../../src/hooks/Toast";
import { LightboxProvider } from "../../../src/hooks/Lightbox/context";
import { OnThisDayRow } from "../../../src/components/FrontPage/OnThisDayRow";
import "../../../src/index.scss";
import { messages } from "./messages";

// Mounts the main page's On this day row against an isolated Apollo
// transport; the test answers on-this-day requests itself.
const thumbnail = (id: string) =>
  `data:image/svg+xml,${encodeURIComponent(`<svg xmlns="http://www.w3.org/2000/svg" width="320" height="240"><rect width="320" height="240" fill="${id === "1" ? "#31606a" : "#635286"}"/><circle cx="${80 + Number(id) * 35}" cy="120" r="40" fill="#bfe4f2"/></svg>`)}`;
const images: GQL.SlimImageDataFragment[] = ["1", "2", "3", "4"].map((id) => ({
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
const requested: unknown[] = [];
Object.assign(window, { onThisDayFixture: { requested } });
const client = new ApolloClient({
  cache: new InMemoryCache({
    possibleTypes: {
      VisualFile: ["ImageFile", "VideoFile"],
      BaseFile: ["ImageFile", "VideoFile", "GalleryFile"],
    },
  }),
  link: new ApolloLink(
    (operation) =>
      new Observable((observer) => {
        requested.push(operation.variables);
        const ids = (operation.variables.image_ids ?? []) as number[];
        observer.next({
          data: {
            findImages: {
              __typename: "FindImagesResultType",
              count: ids.length,
              // Ascending ID order, as the server returns it.
              images: images.filter((image) => ids.includes(Number(image.id))),
            },
          },
        });
        observer.complete();
      })
  ),
});
const configuration = {
  interface: { imageLightbox: { disableAnimation: true } },
  ui: {},
} as GQL.ConfigDataFragment;
ReactDOM.render(
  <ApolloProvider client={client}>
    <BrowserRouter>
      <IntlProvider locale="en-GB" messages={messages}>
        <ConfigurationProvider configuration={configuration}>
          <ToastProvider>
            <LightboxProvider>
              <main className="p-3 recommendations-container">
                <h1>On this day fixture</h1>
                <OnThisDayRow kind="image" />
              </main>
            </LightboxProvider>
          </ToastProvider>
        </ConfigurationProvider>
      </IntlProvider>
    </BrowserRouter>
  </ApolloProvider>,
  document.getElementById("root")
);
