import React, { useEffect, useMemo, useState } from "react";
import { Helmet } from "react-helmet";
import { Button, Card, Col, Form, Row, Spinner, Tab, Tabs } from "react-bootstrap";
import {
  Link,
  Route,
  Switch,
  useHistory,
  useParams,
} from "react-router-dom";
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
import { TruncatedText } from "src/components/Shared/TruncatedText";
import {
  Performer,
  PerformerSelect,
} from "src/components/Performers/PerformerSelect";
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

const CopyrightList: React.FC = () => {
  const [search, setSearch] = useState("");
  const { data, loading } = GQL.useFindCopyrightsQuery({
    variables: {
      filter: {
        q: search || undefined,
        page: 1,
        per_page: -1,
        sort: "name",
        direction: GQL.SortDirectionEnum.Asc,
      },
    },
  });

  const copyrights = data?.findCopyrights.copyrights ?? [];

  return (
    <div className="container-fluid py-3">
      <div className="d-flex align-items-center mb-3">
        <h2 className="mb-0">Copyrights</h2>
        <Button as={Link} to="/copyrights/new" className="ml-auto">
          New Copyright
        </Button>
      </div>
      <Form.Control
        className="mb-3"
        value={search}
        onChange={(event) => setSearch(event.currentTarget.value)}
        placeholder="Search Copyrights"
      />
      {loading ? (
        <Spinner animation="border" />
      ) : (
        <Row className="justify-content-center">
          {copyrights.map((copyright) => (
            <Col key={copyright.id} sm={6} lg={4} xl={3} className="mb-3">
              <Card
                as={Link}
                to={`/copyrights/${copyright.id}`}
                className="tag-card h-100 text-reset text-decoration-none"
              >
                <img
                  className="tag-card-image"
                  src={copyright.image_path}
                  alt={copyright.name}
                  loading="lazy"
                />
                <Card.Body>
                  <Card.Title>{copyright.name}</Card.Title>
                  {copyright.description ? (
                    <TruncatedText
                      className="tag-description"
                      text={copyright.description}
                      lineCount={3}
                    />
                  ) : null}
                  <small className="text-muted d-block mt-2">
                    {copyright.scene_count} Videos · {copyright.image_count}{" "}
                    Images · {copyright.performer_count} Characters
                  </small>
                </Card.Body>
              </Card>
            </Col>
          ))}
          {copyrights.length === 0 ? (
            <Col className="text-muted">No Copyrights found.</Col>
          ) : null}
        </Row>
      )}
    </div>
  );
};

const CopyrightDetailsPanel: React.FC<{
  copyright: GQL.CopyrightDataFragment;
}> = ({ copyright }) => {
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
      <DetailItem id="details" value={copyright.description} />
      <DetailItem
        id="parent-series"
        label="Parent Series"
        value={renderRelations(copyright.parents)}
      />
      <DetailItem
        id="sub-series"
        label="Sub-series"
        value={renderRelations(copyright.children)}
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
        const result = await createCopyright({ variables: { input } });
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
    if (!window.confirm(`Delete Copyright “${copyright.name}”?`)) return;
    setSaving(true);
    try {
      await destroyCopyright({ variables: { id: copyright.id } });
      history.replace("/copyrights");
    } catch (error) {
      Toast.error(error);
      setSaving(false);
    }
  }

  return (
    <>
      {create ? <h2>New Copyright</h2> : null}
      <Form noValidate onSubmit={(event) => event.preventDefault()} id="copyright-edit">
        <Form.Group>
          <Form.Label>Name</Form.Label>
          <Form.Control
            value={values.name}
            onChange={(event) =>
              setValues({ ...values, name: event.currentTarget.value })
            }
          />
        </Form.Group>
        <Form.Group>
          <Form.Label>Aliases</Form.Label>
          <Form.Control
            as="textarea"
            rows={3}
            value={values.aliases}
            onChange={(event) =>
              setValues({ ...values, aliases: event.currentTarget.value })
            }
            placeholder="One alias per line"
          />
        </Form.Group>
        <Form.Group>
          <Form.Label>Details</Form.Label>
          <Form.Control
            as="textarea"
            rows={5}
            value={values.description}
            onChange={(event) =>
              setValues({ ...values, description: event.currentTarget.value })
            }
          />
        </Form.Group>
        <Form.Group>
          <Form.Label>Parent Series</Form.Label>
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
        </Form.Group>
        <Form.Group>
          <Form.Label>Sub-series</Form.Label>
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
        </Form.Group>
      </Form>

      <Tabs defaultActiveKey="characters" id="copyright-edit-tabs" className="mt-3">
        <Tab
          eventKey="characters"
          title={`Characters (${performers.length})`}
        >
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
        onDelete={copyright ? destroy : undefined}
        acceptSVG
      />
    </>
  );
};

const CopyrightMediaTabs: React.FC<{
  copyright: GQL.CopyrightDataFragment;
  initialTab?: string;
}> = ({ copyright, initialTab }) => {
  const history = useHistory();
  const active = ["videos", "images", "characters"].includes(initialTab ?? "")
    ? initialTab
    : "videos";

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
      <Tab eventKey="videos" title={`Videos (${copyright.scene_count})`}>
        <Row className="pt-3">
          {copyright.scenes.map((scene) => (
            <Col key={scene.id} sm={6} lg={4} xl={3} className="mb-3">
              <Card
                as={Link}
                to={`/scenes/${scene.id}`}
                className="h-100 text-reset"
              >
                {scene.paths.screenshot ? (
                  <img
                    src={scene.paths.screenshot}
                    alt=""
                    style={{
                      width: "100%",
                      aspectRatio: "16 / 9",
                      objectFit: "cover",
                    }}
                  />
                ) : null}
                <Card.Body>
                  <TruncatedText text={scene.title || `Video #${scene.id}`} />
                </Card.Body>
              </Card>
            </Col>
          ))}
          {copyright.scenes.length === 0 ? (
            <Col className="text-muted">No Videos assigned.</Col>
          ) : null}
        </Row>
      </Tab>
      <Tab eventKey="images" title={`Images (${copyright.image_count})`}>
        <Row className="pt-3">
          {copyright.images.map((image) => (
            <Col key={image.id} sm={6} lg={4} xl={3} className="mb-3">
              <Card
                as={Link}
                to={`/images/${image.id}`}
                className="h-100 text-reset"
              >
                <img
                  src={image.paths.thumbnail ?? image.paths.image ?? ""}
                  alt=""
                  style={{
                    width: "100%",
                    aspectRatio: "1 / 1",
                    objectFit: "cover",
                  }}
                />
                <Card.Body>
                  <TruncatedText text={image.title || `Image #${image.id}`} />
                </Card.Body>
              </Card>
            </Col>
          ))}
          {copyright.images.length === 0 ? (
            <Col className="text-muted">No Images assigned.</Col>
          ) : null}
        </Row>
      </Tab>
      <Tab
        eventKey="characters"
        title={`Characters (${copyright.performer_count})`}
      >
        <Row className="pt-3">
          {copyright.performers.map((performer) => (
            <Col key={performer.id} sm={6} lg={4} xl={3} className="mb-3">
              <Card
                as={Link}
                to={`/performers/${performer.id}`}
                className="h-100 text-reset"
              >
                {performer.image_path ? (
                  <img
                    src={performer.image_path}
                    alt=""
                    style={{
                      width: "100%",
                      aspectRatio: "3 / 4",
                      objectFit: "cover",
                    }}
                  />
                ) : null}
                <Card.Body>
                  <strong>{performer.name}</strong>
                  {performer.disambiguation ? (
                    <small className="text-muted d-block">
                      {performer.disambiguation}
                    </small>
                  ) : null}
                </Card.Body>
              </Card>
            </Col>
          ))}
          {copyright.performers.length === 0 ? (
            <Col className="text-muted">No Characters assigned.</Col>
          ) : null}
        </Row>
      </Tab>
    </Tabs>
  );
};

const CopyrightDetail: React.FC = () => {
  const { id, tab } = useParams<{ id: string; tab?: string }>();
  const { configuration } = useConfigurationContext();
  const [editing, setEditing] = useState(false);
  const [image, setImage] = useState<string | null>();
  const [encodingImage, setEncodingImage] = useState(false);
  const [updateCopyright] = GQL.useCopyrightUpdateMutation();
  const { data, loading, refetch } = GQL.useFindCopyrightQuery({
    variables: { id },
  });

  const copyright = data?.findCopyright;
  const compactExpandedDetails =
    configuration?.ui.compactExpandedDetails ?? false;

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

  async function setFavorite(value: boolean) {
    await updateCopyright({
      variables: { input: { id: copyright.id, favorite: value } },
    });
    void refetch();
  }

  function finishEdit() {
    setEditing(false);
    setImage(undefined);
    void refetch();
  }

  const headerClassName = cx("detail-header", {
    edit: editing,
    "full-width": !compactExpandedDetails,
  });

  return (
    <div id="tag-page" className="row">
      <Helmet title={copyright.name} />
      <div className={headerClassName}>
        <BackgroundImage
          imagePath={copyright.image_path ?? undefined}
          show={!editing}
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
                  <CopyrightDetailsPanel copyright={copyright} />
                  <DetailsEditNavbar
                    objectName={copyright.name}
                    isNew={false}
                    isEditing={false}
                    onToggleEdit={() => setEditing(true)}
                    onSave={() => {}}
                    onImageChange={() => {}}
                    onClearImage={() => {}}
                    onDelete={() => setEditing(true)}
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
              <CopyrightMediaTabs copyright={copyright} initialTab={tab} />
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
      <Route exact path="/copyrights" component={CopyrightList} />
      <Route exact path="/copyrights/new" component={CopyrightCreate} />
      <Route path="/copyrights/:id/:tab?" component={CopyrightDetail} />
    </Switch>
  </>
);

export default CopyrightRoutes;
