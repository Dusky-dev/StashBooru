import React, { useMemo, useState } from "react";
import { useHistory, useLocation } from "react-router-dom";
import { useIntl } from "react-intl";
import * as GQL from "src/core/generated-graphql";
import { useTagCreate } from "src/core/StashService";
import { LoadingIndicator } from "src/components/Shared/LoadingIndicator";
import { useToast } from "src/hooks/Toast";
import { tagRelationHook } from "src/core/tags";
import { TagEditPanel } from "./TagEditPanel";

interface IProps {
  basePath?: string;
  requiredParentId?: string;
  entityName?: string;
}

const TagCreate: React.FC<IProps> = ({
  basePath = "/tags",
  requiredParentId,
  entityName,
}) => {
  const intl = useIntl();
  const history = useHistory();
  const Toast = useToast();

  const location = useLocation();
  const query = useMemo(() => new URLSearchParams(location.search), [location]);
  const tag = {
    name: query.get("q") ?? undefined,
  };

  // Editing tag state
  const [image, setImage] = useState<string | null>();
  const [encodingImage, setEncodingImage] = useState<boolean>(false);

  const [createTag] = useTagCreate();

  async function onSave(input: GQL.TagCreateInput, andNew?: boolean) {
    const oldRelations = {
      parents: [],
      children: [],
    };
    const createInput = requiredParentId
      ? {
          ...input,
          parent_ids: Array.from(
            new Set([...(input.parent_ids ?? []), requiredParentId])
          ),
        }
      : input;
    const result = await createTag({
      variables: { input: createInput },
    });
    if (result.data?.tagCreate?.id) {
      const created = result.data.tagCreate;
      tagRelationHook(created, oldRelations, {
        parents: created.parents,
        children: created.children,
      });
      if (!andNew) {
        history.push(`${basePath}/${created.id}`);
      }
      Toast.success(
        intl.formatMessage(
          { id: "toast.created_entity" },
          {
            entity: (
              entityName ?? intl.formatMessage({ id: "tag" })
            ).toLocaleLowerCase(),
          }
        )
      );
    }
  }

  function renderImage() {
    if (image) {
      return <img className="logo" alt="" src={image} />;
    }
  }

  return (
    <div className="row">
      <div className="tag-details col-md-8">
        <div className="text-center logo-container">
          {encodingImage ? (
            <LoadingIndicator
              message={intl.formatMessage({ id: "actions.encoding_image" })}
            />
          ) : (
            renderImage()
          )}
        </div>
        <TagEditPanel
          tag={tag}
          onSubmit={onSave}
          onCancel={() => history.push(basePath)}
          onDelete={() => {}}
          setImage={setImage}
          setEncodingImage={setEncodingImage}
          basePath={basePath}
          entityName={entityName}
        />
      </div>
    </div>
  );
};

export default TagCreate;
