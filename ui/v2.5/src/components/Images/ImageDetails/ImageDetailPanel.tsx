import React from "react";
import * as GQL from "src/core/generated-graphql";
import { GalleryLink } from "src/components/Shared/TagLink";
import { FormattedMessage, useIntl } from "react-intl";
import { DetailsItem } from "./ImageDetailsCustomize";
import { PhotographerLink } from "src/components/Shared/Link";
import { PatchComponent } from "../../../patch";
import { CustomFields } from "src/components/Shared/CustomFields";
import { ImageFileInfoPanel } from "./ImageFileInfoPanel";

interface IImageDetailProps {
  image: GQL.ImageDataFragment;
}

export const ImageDetailPanel: React.FC<IImageDetailProps> = PatchComponent(
  "ImageDetailPanel",
  (props) => {
    const intl = useIntl();
    function renderDetails() {
      if (!props.image.details) return;
      return (
        <DetailsItem
          id="description"
          label={intl.formatMessage({ id: "details" })}
        >
          <h6>
            <FormattedMessage id="details" />:{" "}
          </h6>
          <p className="pre">{props.image.details}</p>
        </DetailsItem>
      );
    }

    function renderGalleries() {
      if (props.image.galleries.length === 0) return;
      const galleries = props.image.galleries.map((gallery) => (
        <GalleryLink key={gallery.id} gallery={gallery} />
      ));
      return (
        <DetailsItem
          id="galleries"
          label={intl.formatMessage(
            { id: "countables.galleries" },
            { count: props.image.galleries.length }
          )}
        >
          <h6>
            <FormattedMessage
              id="countables.galleries"
              values={{ count: props.image.galleries.length }}
            />
          </h6>
          {galleries}
        </DetailsItem>
      );
    }

    // filename should use entire row if there is no studio
    const imageDetailsWidth = props.image.studio ? "col-9" : "col-12";

    return (
      <>
        <div className="row">
          <div className={`${imageDetailsWidth} col-12 image-details`}>
            {renderGalleries()}
            {props.image.code && (
              <DetailsItem
                id="code"
                label={intl.formatMessage({ id: "scene_code" })}
              >
                <h6>
                  <FormattedMessage id="scene_code" />: {props.image.code}{" "}
                </h6>
              </DetailsItem>
            )}
            {props.image.photographer && (
              <DetailsItem
                id="photographer"
                label={intl.formatMessage({ id: "photographer" })}
              >
                <h6>
                  <FormattedMessage id="photographer" />:{" "}
                  <PhotographerLink
                    photographer={props.image.photographer}
                    linkType="image"
                  />
                </h6>
              </DetailsItem>
            )}
          </div>
        </div>
        <div className="row">
          <div className="col-12">
            {renderDetails()}
            <DetailsItem
              id="custom_fields"
              label={intl.formatMessage({ id: "custom_fields.title" })}
            >
              <CustomFields values={props.image.custom_fields} fullWidth />
            </DetailsItem>
            <hr />
            <ImageFileInfoPanel image={props.image} />
          </div>
        </div>
      </>
    );
  }
);
