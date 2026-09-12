import React from "react";
import { FormattedMessage, useIntl } from "react-intl";
import { Link } from "react-router-dom";
import * as GQL from "src/core/generated-graphql";
import TextUtils from "src/utils/text";
import { sceneAgeFromDate } from "src/utils/scene";
import { TagLink } from "src/components/Shared/TagLink";
import { CopyrightLink } from "src/components/Copyrights/CopyrightLink";
import { PerformerCard } from "src/components/Performers/PerformerCard";
import { sortPerformers } from "src/core/performers";
import { DirectorLink } from "src/components/Shared/Link";
import { CustomFields } from "src/components/Shared/CustomFields";
import { SceneFileInfoPanel } from "./SceneFileInfoPanel";

interface ISceneDetailProps {
  scene: GQL.SceneDataFragment;
}

export const SceneDetailPanel: React.FC<ISceneDetailProps> = (props) => {
  const intl = useIntl();

  function renderDetails() {
    if (!props.scene.details || props.scene.details === "") return;
    return (
      <>
        <h6>
          <FormattedMessage id="details" />:{" "}
        </h6>
        <p className="pre">{props.scene.details}</p>
      </>
    );
  }

  function renderArtists() {
    const artists = props.scene.artists ?? [];
    if (artists.length === 0) return;

    return (
      <>
        <h6>
          {intl.formatMessage({ id: "artists", defaultMessage: "Artists" })} ({artists.length})
        </h6>
        <div className="mb-3">
          {artists.map((artist, index) => (
            <React.Fragment key={artist.id}>
              {index > 0 ? ", " : null}
              <Link to={`/studios/${artist.id}`}>{artist.name}</Link>
            </React.Fragment>
          ))}
        </div>
      </>
    );
  }

  function renderMetadata() {
    const copyrights = props.scene.copyrights ?? [];
    const tags = props.scene.tags;
    if (tags.length === 0 && copyrights.length === 0) return;

    return (
      <>
        {copyrights.length > 0 && (
          <>
            <h6>
              {intl.formatMessage({
                id: "copyrights",
                defaultMessage: "Copyrights",
              })}{" "}
              ({copyrights.length})
            </h6>
            {copyrights.map((copyright) => (
              <CopyrightLink key={copyright.id} copyright={copyright} />
            ))}
          </>
        )}
        {tags.length > 0 && (
          <>
            <h6>
              <FormattedMessage
                id="countables.tags"
                values={{ count: tags.length }}
              />
            </h6>
            {tags.map((tag) => (
              <TagLink key={tag.id} tag={tag} />
            ))}
          </>
        )}
      </>
    );
  }

  function renderPerformers() {
    if (props.scene.performers.length === 0) return;
    const performers = sortPerformers(props.scene.performers);

    const ageFromDate = sceneAgeFromDate(
      props.scene.production_date,
      props.scene.date
    );

    const cards = performers.map((performer) => (
      <PerformerCard
        key={performer.id}
        performer={performer}
        ageFromDate={ageFromDate}
      />
    ));

    return (
      <>
        <h6>
          <FormattedMessage
            id="countables.performers"
            values={{ count: props.scene.performers.length }}
          />
        </h6>
        <div className="row justify-content-center scene-performers">
          {cards}
        </div>
      </>
    );
  }

  // filename should use entire row if there is no studio
  const sceneDetailsWidth = props.scene.studio ? "col-9" : "col-12";

  return (
    <>
      <div className="row">
        <div className={`${sceneDetailsWidth} col-12 scene-details`}>
          <h6>
            <FormattedMessage id="created_at" />:{" "}
            {TextUtils.formatDateTime(intl, props.scene.created_at)}{" "}
          </h6>
          <h6>
            <FormattedMessage id="updated_at" />:{" "}
            {TextUtils.formatDateTime(intl, props.scene.updated_at)}{" "}
          </h6>
          {props.scene.production_date && (
            <h6>
              <FormattedMessage id="production_date" />:{" "}
              {TextUtils.formatFuzzyDate(intl, props.scene.production_date)}
            </h6>
          )}
          {props.scene.code && (
            <h6>
              <FormattedMessage id="scene_code" />: {props.scene.code}{" "}
            </h6>
          )}
          {props.scene.director && (
            <h6>
              <FormattedMessage id="director" />:{" "}
              <DirectorLink director={props.scene.director} linkType="scene" />
            </h6>
          )}
        </div>
      </div>
      <div className="row">
        <div className="col-12">
          {renderDetails()}
          {renderArtists()}
          {renderMetadata()}
          {renderPerformers()}
          <CustomFields values={props.scene.custom_fields} fullWidth />
          <hr />
          <h6>File info</h6>
          <SceneFileInfoPanel scene={props.scene} />
        </div>
      </div>
    </>
  );
};

export default SceneDetailPanel;
