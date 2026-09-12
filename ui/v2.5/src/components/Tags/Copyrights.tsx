import React, { useEffect, useMemo, useState } from "react";
import { Helmet } from "react-helmet";
import { Col, Form, Row, Spinner, Tab, Tabs } from "react-bootstrap";
import { Route, Switch, useHistory, useParams } from "react-router-dom";
import cx from "classnames";

import * as GQL from "src/core/generated-graphql";
import {
  Copyright,
  CopyrightSelect,
} from "src/components/Copyrights/CopyrightSelect";
import { CopyrightLink } from "src/components/Copyrights/CopyrightLink";
import { DetailsEditNavbar } from "src/components/Shared/DetailsEditNavbar";
import { DetailImage } from "src/components/Shared/DetailImage";
import { DetailItem } from "src/components/Shared/DetailItem";
import { FavoriteIcon } from "src/components/Shared/FavoriteIcon";
import { BackgroundImage } from "src/components/Shared/DetailsPage/BackgroundImage";
import { AliasList } from "src/components/Shared/DetailsPage/AliasList";
import { DetailTitle } from "src/components/Shared/DetailsPage/DetailTitle";
import { HeaderImage } from "src/components/Shared/DetailsPage/HeaderImage";
import { TabTitleCounter } from "src/components/Shared/DetailsPage/Tabs";
import { ExpandCollapseButton } from "src/components/Shared/CollapseButton";
import {
  Performer,
  PerformerSelect,
} from "src/components/Performers/PerformerSelect";
import { FilteredPerformerList } from "src/components/Performers/PerformerList";
import { FilteredSceneList } from "src/components/Scenes/SceneList";
import { FilteredImageList } from "src/components/Images/ImageList";
import { View } from "src/components/List/views";
import { useConfigurationContext } from "src/hooks/Config";
import { useToast } from "src/hooks/Toast";
import ImageUtils from "src/utils/image";

interface CopyrightFormValues {
  name: string;
  description: string;
  aliases: string;
  parents: Copyright[];
  children: Copyright[];
}

const emptyValues: CopyrightFormValues = {
  name: "",
  description: "",
  aliases: "",
  parents: [],
  children: [],
};

function aliasesFromText(value: string) {
  return value
    .split(/[,\n]/)
    .map((item) => item.trim())
    .filter(Boolean);
}

const CopyrightDetailsPanel: React.FC<{
  copyright: GQL.CopyrightDataFragment;
  fullWidth?: boolean;
}> = ({ copyright, fullWidth }) => {
  function renderRelations(items: Copyright[]) {
    if (items.length === 0) return;
    return (
      <>
        {items.map((item) => (
          <CopyrightLink key={item.id} copyright={item} />
        ))}
      </>
    );
  }

  return (
    <div className="detail-group">
      <DetailItem
        id="details"
        value={copyright.description}
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
        value={renderRelations(copyright.children)}
        fullWidth={fullWidth}
      />
    </div>
  );
};

const CopyrightEditPanel: React.FC<{
  copyright?: GQL.CopyrightDataFragment;
  create?: boolean;
  onSaved: (copyright: GQL.CopyrightDataFragment) => void;
  onCancel: () => void;
  setImage: (image?: string | null) => void;
  setEncodingImage: (loading: boolean) => void;
}> = ({
  copyright,
  create = false,
  onSaved,
  onCancel,
  setImage,
  setEncodingImage,
}) => {
  const history = useHistory();
  const Toast = useToast();
  const [createCopyright] = GQL.useCopyrightCreateMutation();
  const [updateCopyright] = GQL.useCopyrightUpdateMutation();
  const [destroyCopyright] = GQL.useCopyrightDestroyMutation();
  const [updatePerformers] = GQL.useCopyrightPerformersUpdateMutation();
  const [values, setValues] = useState<CopyrightFormValues>(emptyValues);
  const [performers, setPerformers] = useState<Performer[]>([]);
  const [imageValue, setImageValue] = useState<string | null>();
  const [imageTouched, setImageTouched] = useState(false);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (!copyright) {
      setValues(emptyValues);
      setPerformers([]);
      setImageValue(undefined);
      setImageTouched(false);
      return;
    }
    setValues({
      name: copyright.name,
      description: copyright.description,
      aliases: copyright.aliases.join("\n"),
      parents: copyright.parents,
      children: copyright.children,
    });
    setPerformers(copyright.performers);
    setImageValue(undefined);
    setImageTouched(false);
  }, [copyright]);

  const encodingImage = ImageUtils.usePasteImage((data) => {
    setImageValue(data);
    setImage(data);
    setImageTouched(true);
  });

  useEffect(() => {
    setEncodingImage(encodingImage);
  }, [encodingImage, setEncodingImage]);

  function setEditedImage(data: string | null) {
    setImageValue(data);
    setImage(data);
    setImageTouched(true);
  }

  function onImageChange(event: React.FormEvent<HTMLInputElement>) {
    ImageUtils.onImageChange(event, setEditedImage);
  }

  async function save() {
    const name = values.name.trim();
    if (!name) return;

    setSaving(true);
    try {
      const input = {
        name,
        description: values.description,
        aliases: aliasesFromText(values.aliases),
        parent_ids: values.parents.map((item) => item.id),
        child_ids: values.children.map((item) => item.id),
        ...(imageTouched ? { image: imageValue } : {}),
      };

      let saved: GQL.CopyrightDataFragment | undefined;
      if (create) {
        const result = await createCopyright({
          variables: { input },
          refetchQueries: ["FindCopyrights"],
          awaitRefetchQueries: true,
        });
        saved = result.data?.copyrightCreate;
      } else if (copyright) {
        const result = await updateCopyright({
          variables: { input: { id: copyright.id, ...input } },
        });
        saved = result.data?.copyrightUpdate;
      }

      if (!saved) return;

      await updatePerformers({
        variables: {
          copyrightID: saved.id,
          performerIDs: performers.map((item) => item.id),
        },
      });

      Toast.success(`Saved Copyright “${saved.name}”.`);
      onSaved(saved);
    } catch (error) {
      Toast.error(error);
    } finally {
      setSaving(false);
    }
  }

  async function destroy() {
    if (!copyright) return;
    setSaving(true);
    try {
      await destroyCopyright({ variables: { id: copyright.id } });
      history.replace("/copyrights");
    } catch (error) {
      Toast.error(error);
      setSaving(false);
    }
  }

  const field = (label: React.ReactNode, control: React.ReactNode) => (
    <Form.Group as={Row}>
      <Form.Label column sm={3} xl={2}>
        {label}
      </Form.Label>
      <Col sm={9} xl={7}>
        {control}
      </Col>
    </Form.Group>
  );

  return (
    <>
      {create ? <h2>New Copyright</h2> : null}
      <Form
        noValidate
        onSubmit={(event) => event.preventDefault()}
        id="copyright-edit"
      >
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
          "Aliases",
          <Form.Control
            className="text-input"
            as="textarea"
            rows={3}
            value={values.aliases}
            onChange={(event) =>
              setValues({ ...values, aliases: event.currentTarget.value })
            }
            placeholder="One alias per line"
          />
        )}
        {field(
          "Details",
          <Form.Control
            className="text-input"
            as="textarea"
            rows={5}
            value={values.description}
            onChange={(event) =>
              setValues({ ...values, description: event.currentTarget.value })
            }
          />
        )}
        {field(
          "Parent Series",
          <CopyrightSelect
            isMulti
            values={values.parents}
            excludeIds={[
              ...(copyright ? [copyright.id] : []),
              ...values.children.map((item) => item.id),
            ]}
            onSelect={(parents) => setValues({ ...values, parents })}
            creatable={false}
          />
        )}
        {field(
          "Sub-series",
          <CopyrightSelect
            isMulti
            values={values.children}
            excludeIds={[
              ...(copyright ? [copyright.id] : []),
              ...values.parents.map((item) => item.id),
            ]}
            onSelect={(children) => setValues({ ...values, children })}
            creatable={false}
          />
        )}
      </Form>

      <Tabs
        defaultActiveKey="characters"
        id="copyright-edit-tabs"
        className="mt-3"
      >
        <Tab eventKey="characters" title={`Characters (${performers.length})`}>
          <div className="pt-3">
            <PerformerSelect
              isMulti
              values={performers}
              onSelect={setPerformers}
              noSelectionString="Select Characters"
            />
          </div>
        </Tab>
      </Tabs>

      <DetailsEditNavbar
        objectName={values.name || "Copyright"}
        classNames="col-xl-9 mt-3"
        isNew={create}
        isEditing
        onToggleEdit={onCancel}
        onSave={save}
        saveDisabled={saving || !values.name.trim()}
        onImageChange={onImageChange}
        onImageChangeURL={setEditedImage}
        onClearImage={() => setEditedImage(null)}
        onDelete={destroy}
        acceptSVG
      />
    </>
  );
};

const CopyrightMediaTabs: React.FC<{
  copyright: GQL.CopyrightDataFragment;
  initialTab?: string;
  abbreviateCounter: boolean;
}> = ({ copyright, initialTab, abbreviateCounter }) => {
  const history = useHistory();

  const populatedDefaultTab = useMemo(() => {
    if (copyright.scene_count !== 0) return "videos";
    if (copyright.image_count !== 0) return "images";
    if (copyright.performer_count !== 0) return "characters";
    return "videos";
  }, [copyright.scene_count, copyright.image_count, copyright.performer_count]);

  const active = ["videos", "images", "characters"].includes(initialTab ?? "")
    ? initialTab
    : populatedDefaultTab;

  const sceneIDs = useMemo(
    () => copyright.scenes.map((scene) => Number(scene.id)),
    [copyright.scenes]
  );
  const imageIDs = useMemo(
    () => copyright.images.map((image) => Number(image.id)),
    [copyright.images]
  );
  const performerIDs = useMemo(
    () => copyright.performers.map((performer) => Number(performer.id)),
    [copyright.performers]
  );

  return (
    <Tabs
      id="copyright-media-tabs"
      activeKey={active}
      onSelect={(key) => {
        if (key) history.replace(`/copyrights/${copyright.id}/${key}`);
      }}
      mountOnEnter
      unmountOnExit
    >
      <Tab
        eventKey="videos"
        title={
          <TabTitleCounter
            messageID="scenes"
            count={copyright.scene_count}
            abbreviateCounter={abbreviateCounter}
          />
        }
      >
        <FilteredSceneList
          sceneIDs={sceneIDs}
          alterQuery
          view={View.CopyrightScenes}
        />
      </Tab>
      <Tab
        eventKey="images"
        title={
          <TabTitleCounter
            messageID="images"
            count={copyright.image_count}
            abbreviateCounter={abbreviateCounter}
          />
        }
      >
        <FilteredImageList
          imageIDs={imageIDs}
          alterQuery
          view={View.CopyrightImages}
        />
      </Tab>
      <Tab
        eventKey="characters"
        title={
          <TabTitleCounter
            messageID="performers"
            count={copyright.performer_count}
            abbreviateCounter={abbreviateCounter}
          />
        }
      >
        <FilteredPerformerList
          performerIDs={performerIDs}
          alterQuery
          view={View.CopyrightPerformers}
        />
      </Tab>
    </Tabs>
  );
};

const CopyrightDetail: React.FC = () => {
  const { id, tab } = useParams<{ id: string; tab?: string }>();
  const history = useHistory();
  const Toast = useToast();
  const { configuration } = useConfigurationContext();
  const uiConfig = configuration?.ui;
  const abbreviateCounter = uiConfig?.abbreviateCounters ?? false;
  const enableBackgroundImage = uiConfig?.enableTagBackgroundImage ?? false;
  const showAllDetails = uiConfig?.showAllDetails ?? true;
  const compactExpandedDetails = uiConfig?.compactExpandedDetails ?? false;
  const [collapsed, setCollapsed] = useState(!showAllDetails);
  const [editing, setEditing] = useState(false);
  const [image, setImage] = useState<string | null>();
  const [encodingImage, setEncodingImage] = useState(false);
  const [updateCopyright] = GQL.useCopyrightUpdateMutation();
  const [destroyCopyright] = GQL.useCopyrightDestroyMutation();
  const { data, loading, refetch } = GQL.useFindCopyrightQuery({
    variables: { id },
  });

  const copyright = data?.findCopyright;

  const activeImage = useMemo(() => {
    if (!copyright) return undefined;
    if (editing) {
      if (image === null) return undefined;
      if (image) return image;
    }
    return copyright.image_path;
  }, [copyright, editing, image]);

  if (loading) return <Spinner animation="border" />;
  if (!copyright) {
    return <div className="alert alert-warning">Copyright not found.</div>;
  }
  const copyrightID = copyright.id;

  async function setFavorite(value: boolean) {
    await updateCopyright({
      variables: { input: { id: copyrightID, favorite: value } },
    });
    void refetch();
  }

  async function destroy() {
    try {
      await destroyCopyright({ variables: { id: copyrightID } });
      history.replace("/copyrights");
    } catch (error) {
      Toast.error(error);
    }
  }

  function finishEdit() {
    setEditing(false);
    setImage(undefined);
    void refetch();
  }

  const headerClassName = cx("detail-header", {
    edit: editing,
    collapsed,
    "full-width": !collapsed && !compactExpandedDetails,
  });

  return (
    <div id="tag-page" className="row">
      <Helmet title={copyright.name} />
      <div className={headerClassName}>
        <BackgroundImage
          imagePath={copyright.image_path ?? undefined}
          show={enableBackgroundImage && !editing && !!copyright.image_path}
        />
        <div className="detail-container">
          <HeaderImage encodingImage={encodingImage}>
            {activeImage ? (
              <DetailImage
                className="logo"
                alt={copyright.name}
                src={activeImage}
              />
            ) : null}
          </HeaderImage>
          <div className="row">
            <div className="tag-head col">
              <DetailTitle name={copyright.name} classNamePrefix="tag">
                {!editing ? (
                  <ExpandCollapseButton
                    collapsed={collapsed}
                    setCollapsed={(value) => setCollapsed(value)}
                  />
                ) : null}
                <span className="name-icons">
                  <FavoriteIcon
                    favorite={copyright.favorite}
                    onToggleFavorite={setFavorite}
                  />
                </span>
              </DetailTitle>
              <AliasList aliases={copyright.aliases} />
              {editing ? (
                <CopyrightEditPanel
                  copyright={copyright}
                  onSaved={finishEdit}
                  onCancel={() => {
                    setEditing(false);
                    setImage(undefined);
                  }}
                  setImage={setImage}
                  setEncodingImage={setEncodingImage}
                />
              ) : (
                <>
                  <CopyrightDetailsPanel
                    copyright={copyright}
                    fullWidth={!collapsed && !compactExpandedDetails}
                  />
                  <DetailsEditNavbar
                    objectName={copyright.name}
                    isNew={false}
                    isEditing={false}
                    onToggleEdit={() => setEditing(true)}
                    onSave={() => {}}
                    onImageChange={() => {}}
                    onClearImage={() => {}}
                    onDelete={destroy}
                    classNames="mb-2"
                  />
                </>
              )}
            </div>
          </div>
        </div>
      </div>

      <div className="detail-body">
        <div className="tag-body">
          <div className="tag-tabs">
            {!editing ? (
              <CopyrightMediaTabs
                copyright={copyright}
                initialTab={tab}
                abbreviateCounter={abbreviateCounter}
              />
            ) : null}
          </div>
        </div>
      </div>
    </div>
  );
};

const CopyrightCreate: React.FC = () => {
  const history = useHistory();
  const [image, setImage] = useState<string | null>();
  const [encodingImage, setEncodingImage] = useState(false);

  return (
    <div className="row new-view" id="tag-page">
      <div className="tag-details col-md-8">
        <div className="text-center logo-container">
          {encodingImage ? (
            <Spinner animation="border" />
          ) : image ? (
            <img className="logo" alt="" src={image} />
          ) : null}
        </div>
        <CopyrightEditPanel
          create
          onSaved={(created) => history.replace(`/copyrights/${created.id}`)}
          onCancel={() => history.push("/copyrights")}
          setImage={setImage}
          setEncodingImage={setEncodingImage}
        />
      </div>
    </div>
  );
};

const CopyrightRoutes: React.FC = () => (
  <>
    <Helmet title="Copyrights" />
    <Switch>
      <Route exact path="/copyrights/new" component={CopyrightCreate} />
      <Route path="/copyrights/:id/:tab?" component={CopyrightDetail} />
    </Switch>
  </>
);

export default CopyrightRoutes;
