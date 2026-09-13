import React from "react";
import * as GQL from "src/core/generated-graphql";
import TextUtils from "src/utils/text";
import { GalleryLink } from "src/components/Shared/TagLink";
import { FormattedMessage, useIntl } from "react-intl";
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
        <>
          <h6>
            <FormattedMessage id="details" />:{" "}
          </h6>
          <p className="pre">{props.image.details}</p>
        </>
      );
    }

    function renderGalleries() {
      if (props.image.galleries.length === 0) return;
      const galleries = props.image.galleries.map((gallery) => (
        <GalleryLink key={gallery.id} gallery={gallery} />
      ));
      return (
        <>
          <h6>
            <FormattedMessage
              id="countables.galleries"
              values={{ count: props.image.galleries.length }}
            />
          </h6>
          {galleries}
        </>
      );
    }

    // filename should use entire row if there is no studio
    const imageDetailsWidth = props.image.studio ? "col-9" : "col-12";

    return (
      <>
        <div className="row">
          <div className={`${imageDetailsWidth} col-12 image-details`}>
            {renderGalleries()}
            {
              <h6>
                {" "}
                <FormattedMessage id="created_at" />:{" "}
                {TextUtils.formatDateTime(intl, props.image.created_at)}{" "}
              </h6>
            }
            {
              <h6>
                <FormattedMessage id="updated_at" />:{" "}
                {TextUtils.formatDateTime(intl, props.image.updated_at)}{" "}
              </h6>
            }
            {props.image.code && (
              <h6>
                <FormattedMessage id="scene_code" />: {props.image.code}{" "}
              </h6>
            )}
            {props.image.photographer && (
              <h6>
                <FormattedMessage id="photographer" />:{" "}
                <PhotographerLink
                  photographer={props.image.photographer}
                  linkType="image"
                />
              </h6>
            )}
          </div>
        </div>
        <div className="row">
          <div className="col-12">
            {renderDetails()}
            <CustomFields values={props.image.custom_fields} fullWidth />
            <hr />
            <h6>File info</h6>
            <ImageFileInfoPanel image={props.image} />
          </div>
        </div>
      </>
    );
  }
);
