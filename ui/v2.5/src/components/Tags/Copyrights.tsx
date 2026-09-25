import React, { useEffect, useMemo, useState } from "react";
import { FormattedMessage, useIntl } from "react-intl";
import { Helmet } from "react-helmet";
import { Col, Form, Row, Spinner, Tab, Tabs } from "react-bootstrap";
import { Route, Switch, useHistory, useParams } from "react-router-dom";

import * as GQL from "src/core/generated-graphql";
import {
  useCopyrightCreateMutation,
  useCopyrightDestroyMutation,
  useCopyrightUpdateMutation,
} from "src/core/generated-graphql";
import { CopyrightLink } from "src/components/Copyrights/CopyrightLink";
import {
  CopyrightBreadcrumb,
  CopyrightChildrenOrderControl,
  StructuralRoleControl,
} from "src/components/Copyrights/CopyrightTaxonomyControls";
import { CopyrightSelect } from "src/components/Copyrights/CopyrightSelect";
import { DetailsEditNavbar } from "src/components/Shared/DetailsEditNavbar";
import { DetailItem } from "src/components/Shared/DetailItem";
import { ErrorMessage } from "src/components/Shared/ErrorMessage";
import { SweatDrops } from "src/components/Shared/Icon";
import { ImageLightbox } from "src/components/Lightbox/ImageLightbox";
import { PerformerList } from "src/components/Performers/PerformerList";
import { SceneList } from "src/components/Scenes/SceneList";
import { TagLink } from "src/components/Shared/TagLink";
import { useToast } from "src/hooks/Toast";
import { ListFilterModel } from "src/models/list-filter/filter";
import { View } from "src/components/List/views";
import { useCopyrightFilterHook as createCopyrightFilterHook } from "src/core/copyrights";
import { ImageList } from "src/components/Images/ImageList";
import { aliasesFromText } from "src/utils/form";
import ImageUtils from "src/utils/image";

interface CopyrightFormValues {
  name: string;
  sort_name: string;
  description: string;
  aliases: string;
  parents: Copyright[];
  children: Copyright[];
  favorite: boolean;
}

type Copyright = GQL.SlimCopyrightDataFragment;

const emptyValues: CopyrightFormValues = {
  name: "",
  sort_name: "",
  description: "",
  aliases: "",
  parents: [],
  children: [],
  favorite: false,
};

function renderRelations(items: Copyright[]) {
  if (items.length === 0) return undefined;
  return (
    <>
      {items.map((item) => (
        <CopyrightLink key={item.id} copyright={item} />
      ))}
    </>
  );
}

const CopyrightDetailsPanel: React.FC<{
  copyright: GQL.CopyrightDataFragment;
  fullWidth?: boolean;
}> = ({ copyright, fullWidth }) => {
  return (
    <div className="detail-group">
      <CopyrightBreadcrumb items={copyright.breadcrumb} />
      <StructuralRoleControl
        entityType="copyright"
        entityID={copyright.id}
        role={copyright.structural_role}
      />
      <DetailItem
        id="subtree-media"
        label="Taxonomy subtree"
        value={`${copyright.subtree_image_count} images · ${copyright.subtree_scene_count} videos · ${copyright.subtree_performer_count} characters`}
        fullWidth={fullWidth}
      />
      <DetailItem
        id="sort_name"
        value={copyright.sort_name}
        fullWidth={fullWidth}
      />
      <DetailItem
        id="details"
        value={copyright.description}
        fullWidth={fullWidth}
      />
      <DetailItem
        id="aliases"
        value={copyright.aliases.join(", ")}
        fullWidth={fullWidth}
      />
      <DetailItem
        id="parent-series"
        label="Parent Series"
        value={renderRelations(copyright.parents)}
        fullWidth={fullWidth}
      />
      <DetailItem
        id="sub-series"
        label="Sub-series"
        value={renderRelations(copyright.ordered_children)}
        fullWidth={fullWidth}
      />
      <CopyrightChildrenOrderControl
        parentID={copyright.id}
        orderedChildren={copyright.ordered_children}
      />
      <p className="text-muted small">
        <FormattedMessage id="copyright_hierarchy.delete_help" />
      </p>
    </div>
  );
};

const CopyrightEditPanel: React.FC<{
  copyright?: GQL.CopyrightDataFragment;
  onSaved: (copyright: GQL.CopyrightDataFragment) => void;
  setImage: (image: string | null) => void;
  setEncodingImage: (value: boolean) => void;
}> = ({
  copyright,
  onSaved,
  setImage,
  setEncodingImage,
}) => {
  const intl = useIntl();
  const history = useHistory();
  const Toast = useToast();
  const [createCopyright] = useCopyrightCreateMutation();
  const [updateCopyright] = useCopyrightUpdateMutation();
  const [destroyCopyright] = useCopyrightDestroyMutation();
  const [values, setValues] = useState<CopyrightFormValues>(emptyValues);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (!copyright) {
      setValues(emptyValues);
      return;
    }
    setValues({
      name: copyright.name,
      sort_name: copyright.sort_name,
      description: copyright.description,
      aliases: copyright.aliases.join("\n"),
      parents: copyright.parents,
      children: copyright.children,
      favorite: copyright.favorite,
    });
  }, [copyright]);

  const field = (label: React.ReactNode, control: React.ReactNode) => (
    <Form.Group as={Row}>
      <Form.Label column sm={3}>
        {label}
      </Form.Label>
      <Col sm={9}>{control}</Col>
    </Form.Group>
  );

  async function save() {
    const name = values.name.trim();
    if (!name) return;
    setSaving(true);
    try {
      const input = {
        name,
        sort_name: values.sort_name.trim(),
        description: values.description,
        aliases: aliasesFromText(values.aliases),
        parent_ids: values.parents.map((item) => item.id),
        child_ids: values.children.map((item) => item.id),
        favorite: values.favorite,
      };
      let saved: GQL.CopyrightDataFragment | undefined;
      if (copyright) {
        const result = await updateCopyright({
          variables: { input: { id: copyright.id, ...input } },
        });
        saved = result.data?.copyrightUpdate;
      } else {
        const result = await createCopyright({ variables: { input } });
        saved = result.data?.copyrightCreate;
      }
      if (!saved) throw new Error("Copyright save returned no result");
      onSaved(saved);
      if (!copyright) history.replace(`/copyrights/${saved.id}`);
    } catch (error) {
      Toast.error(error);
    } finally {
      setSaving(false);
    }
  }

  async function destroy() {
    if (!copyright) return;
    try {
      await destroyCopyright({ variables: { input: { id: copyright.id } } });
      history.push("/copyrights");
    } catch (error) {
      Toast.error(error);
    }
  }

  async function onImageChange(event: React.ChangeEvent<HTMLInputElement>) {
    const file = event.currentTarget.files?.[0];
    if (!file) return;
    setEncodingImage(true);
    try {
      setImage(await ImageUtils.fileToDataURL(file));
    } finally {
      setEncodingImage(false);
    }
  }

  return (
    <div className="edit-panel">
      <Form>
        {field(
          "Name",
          <Form.Control
            className="text-input"
            value={values.name}
            onChange={(event) =>
              setValues({ ...values, name: event.currentTarget.value })
            }
          />
        )}
        {field(
          <FormattedMessage id="sort_name" />,
          <>
            <Form.Control
              className="text-input"
              aria-label={intl.formatMessage({ id: "sort_name" })}
              value={values.sort_name}
              onChange={(event) =>
                setValues({ ...values, sort_name: event.currentTarget.value })
              }
            />
            <Form.Text className="text-muted">
              <FormattedMessage id="copyright_hierarchy.sort_name_help" />
            </Form.Text>
          </>
        )}
        {field(
          "Aliases",
          <Form.Control
            as="textarea"
            value={values.aliases}
            onChange={(event) =>
              setValues({ ...values, aliases: event.currentTarget.value })
            }
          />
        )}
        {field(
          "Description",
          <Form.Control
            as="textarea"
            value={values.description}
            onChange={(event) =>
              setValues({ ...values, description: event.currentTarget.value })
            }
          />
        )}
        {field(
          "Favorite",
          <Form.Check
            checked={values.favorite}
            onChange={(event) =>
              setValues({ ...values, favorite: event.currentTarget.checked })
            }
          />
        )}
        {field(
          "Image",
          <Form.File custom onChange={onImageChange} label="Choose image" />
        )}
        {field(
          "Parent Series",
          <CopyrightSelect
            values={values.parents}
            onSelect={(items) => setValues({ ...values, parents: items })}
            isMulti
            excludeIds={copyright ? [copyright.id] : []}
            creatable={false}
          />
        )}
        {field(
          "Sub-series",
          <CopyrightSelect
            values={values.children}
            onSelect={(items) => setValues({ ...values, children: items })}
            isMulti
            excludeIds={copyright ? [copyright.id] : []}
            creatable={false}
          />
        )}
        <Form.Text className="text-muted">
          <FormattedMessage id="copyright_hierarchy.parent_help" />
        </Form.Text>
      </Form>

      <Tabs defaultActiveKey="tags" className="mt-3">
        <Tab eventKey="tags" title="Tags">
          {copyright ? (
            <div className="mt-2">
              {copyright.tags.map((tag) => (
                <TagLink key={tag.id} tag={tag} />
              ))}
            </div>
          ) : null}
        </Tab>
      </Tabs>

      <div className="mt-3 d-flex justify-content-between">
        <button
          type="button"
          className="btn btn-primary"
          disabled={saving || !values.name.trim()}
          onClick={() => void save()}
        >
          Save
        </button>
        {copyright ? (
          <button
            type="button"
            className="btn btn-danger"
            disabled={saving}
            onClick={() => void destroy()}
          >
            Delete
          </button>
        ) : null}
      </div>
    </div>
  );
};

const CopyrightDetails: React.FC = () => {
  const { id } = useParams<{ id: string }>();
  const [editing, setEditing] = useState(id === "new");
  const [image, setImage] = useState<string | null>(null);
  const [encodingImage, setEncodingImage] = useState(false);
  const { data, loading, error, refetch } = GQL.useFindCopyrightQuery({
    variables: { id },
    skip: id === "new",
  });

  const copyright = data?.findCopyright;
  const defaultTab =
    copyright &&
    copyright.subtree_image_count > 0
      ? "images"
      : copyright && copyright.subtree_scene_count > 0
        ? "videos"
        : "details";
  const [activeTab, setActiveTab] = useState(defaultTab);

  useEffect(() => {
    if (!activeTab && defaultTab) setActiveTab(defaultTab);
  }, [activeTab, defaultTab]);

  const sceneIDs = useMemo(
    () => copyright?.subtree_scenes.map((scene) => Number(scene.id)) ?? [],
    [copyright?.subtree_scenes]
  );
  const performerIDs = useMemo(
    () =>
      copyright?.subtree_performers.map((performer) => Number(performer.id)) ?? [],
    [copyright?.subtree_performers]
  );

  if (id !== "new" && loading) return <Spinner animation="border" />;
  if (id !== "new" && error) return <ErrorMessage error={error} />;
  if (id !== "new" && !copyright) return null;

  const displayedCopyright = copyright;
  const filterHook = displayedCopyright
    ? createCopyrightFilterHook(displayedCopyright)
    : undefined;

  return (
    <>
      <Helmet>
        <title>{displayedCopyright?.name ?? "New Copyright"}</title>
      </Helmet>
      <DetailsEditNavbar
        objectName={displayedCopyright?.name ?? "Copyright"}
        isNew={id === "new"}
        editing={editing}
        onEdit={() => setEditing(true)}
        onSave={() => setEditing(false)}
      />

      {editing ? (
        <CopyrightEditPanel
          copyright={displayedCopyright ?? undefined}
          onSaved={() => {
            setEditing(false);
            void refetch();
          }}
          setImage={setImage}
          setEncodingImage={setEncodingImage}
        />
      ) : displayedCopyright ? (
        <>
          {image ? <ImageLightbox images={[{ paths: { image } } as never]} /> : null}
          {encodingImage ? <SweatDrops /> : null}
          <Tabs
            activeKey={activeTab}
            onSelect={(key) => key && setActiveTab(key)}
            className="mt-3"
          >
            <Tab
              eventKey="details"
              title={<FormattedMessage id="details" />}
            >
              <CopyrightDetailsPanel copyright={displayedCopyright} />
            </Tab>
            <Tab
              eventKey="images"
              title={`Images (${displayedCopyright.subtree_image_count})`}
            >
              {filterHook ? (
                <ImageList
                  filterMode={GQL.FilterMode.Images}
                  defaultFilter={new ListFilterModel(GQL.FilterMode.Images)}
                  filterHook={filterHook}
                  view={View.Images}
                />
              ) : null}
            </Tab>
            <Tab
              eventKey="videos"
              title={`Videos (${displayedCopyright.subtree_scene_count})`}
            >
              <SceneList sceneIds={sceneIDs} />
            </Tab>
            <Tab
              eventKey="characters"
              title={`Characters (${displayedCopyright.subtree_performer_count})`}
            >
              <PerformerList performerIds={performerIDs} />
            </Tab>
          </Tabs>
        </>
      ) : null}
    </>
  );
};

const CopyrightRoutes: React.FC = () => (
  <Switch>
    <Route path="/copyrights/:id" component={CopyrightDetails} />
  </Switch>
);

export default CopyrightRoutes;