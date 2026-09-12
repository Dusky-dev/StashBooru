import React, { useEffect, useMemo, useState } from "react";
import { Helmet } from "react-helmet";
import {
  Badge,
  Button,
  Card,
  Col,
  Form,
  Row,
  Spinner,
  Tab,
  Tabs,
} from "react-bootstrap";
import { Link, Route, Switch, useHistory, useParams } from "react-router-dom";
import Select from "react-select";

import * as GQL from "src/core/generated-graphql";
import {
  Copyright,
  CopyrightSelect,
} from "src/components/Copyrights/CopyrightSelect";
import { ImageInput } from "src/components/Shared/ImageInput";
import { FavoriteIcon } from "src/components/Shared/FavoriteIcon";
import { TruncatedText } from "src/components/Shared/TruncatedText";
import { useToast } from "src/hooks/Toast";
import ImageUtils from "src/utils/image";

interface CopyrightFormValues {
  name: string;
  sortName: string;
  description: string;
  aliases: string;
  favorite: boolean;
  parents: Copyright[];
  children: Copyright[];
}

interface SelectOption {
  value: string;
  label: string;
}

const emptyValues: CopyrightFormValues = {
  name: "",
  sortName: "",
  description: "",
  aliases: "",
  favorite: false,
  parents: [],
  children: [],
};

function aliasesFromText(value: string) {
  return value
    .split(/[,\n]/)
    .map((item) => item.trim())
    .filter(Boolean);
}

function relationFilter(sort: string): GQL.FindFilterType {
  return {
    page: 1,
    per_page: -1,
    sort,
    direction: GQL.SortDirectionEnum.Asc,
  };
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
        <Row>
          {copyrights.map((copyright) => (
            <Col key={copyright.id} sm={6} lg={4} xl={3} className="mb-3">
              <Card
                as={Link}
                to={`/copyrights/${copyright.id}`}
                className="h-100 text-reset text-decoration-none"
              >
                <div
                  style={{
                    aspectRatio: "16 / 9",
                    overflow: "hidden",
                    background: "var(--secondary)",
                  }}
                >
                  <img
                    src={copyright.image_path}
                    alt=""
                    loading="lazy"
                    style={{
                      width: "100%",
                      height: "100%",
                      objectFit: "cover",
                    }}
                  />
                </div>
                <Card.Body>
                  <div className="d-flex align-items-center">
                    <strong>{copyright.name}</strong>
                    {copyright.favorite ? (
                      <span className="ml-auto">
                        <FavoriteIcon favorite />
                      </span>
                    ) : null}
                  </div>
                  {copyright.aliases.length > 0 ? (
                    <small className="text-muted d-block mt-1">
                      {copyright.aliases.join(", ")}
                    </small>
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
            <Col>
              <div className="text-muted">No Copyrights found.</div>
            </Col>
          ) : null}
        </Row>
      )}
    </div>
  );
};

const CopyrightEditor: React.FC<{
  copyright?: GQL.CopyrightDataFragment;
  create?: boolean;
  onSaved?: (copyright: GQL.CopyrightDataFragment) => void;
  onCancel?: () => void;
}> = ({ copyright, create = false, onSaved, onCancel }) => {
  const history = useHistory();
  const Toast = useToast();
  const [createCopyright] = GQL.useCopyrightCreateMutation();
  const [updateCopyright] = GQL.useCopyrightUpdateMutation();
  const [destroyCopyright] = GQL.useCopyrightDestroyMutation();
  const [updateImages] = GQL.useCopyrightImagesUpdateMutation();
  const [updateScenes] = GQL.useCopyrightScenesUpdateMutation();
  const [updatePerformers] = GQL.useCopyrightPerformersUpdateMutation();

  const [values, setValues] = useState<CopyrightFormValues>(emptyValues);
  const [image, setImage] = useState<string | null>();
  const [imageTouched, setImageTouched] = useState(false);
  const [imageIDs, setImageIDs] = useState<string[]>([]);
  const [sceneIDs, setSceneIDs] = useState<string[]>([]);
  const [performerIDs, setPerformerIDs] = useState<string[]>([]);
  const [saving, setSaving] = useState(false);

  const imageChoices = GQL.useFindImagesQuery({
    variables: { filter: relationFilter("title") },
  });
  const sceneChoices = GQL.useFindScenesQuery({
    variables: { filter: relationFilter("title") },
  });
  const performerChoices = GQL.useFindPerformersQuery({
    variables: { filter: relationFilter("name") },
  });

  useEffect(() => {
    if (create || !copyright) {
      setValues(emptyValues);
      setImageIDs([]);
      setSceneIDs([]);
      setPerformerIDs([]);
      setImage(undefined);
      setImageTouched(false);
      return;
    }
    setValues({
      name: copyright.name,
      sortName: copyright.sort_name,
      description: copyright.description,
      aliases: copyright.aliases.join("\n"),
      favorite: copyright.favorite,
      parents: copyright.parents,
      children: copyright.children,
    });
    setImageIDs(copyright.images.map((item) => item.id));
    setSceneIDs(copyright.scenes.map((item) => item.id));
    setPerformerIDs(copyright.performers.map((item) => item.id));
    setImage(undefined);
    setImageTouched(false);
  }, [copyright, create]);

  const imageOptions = useMemo<SelectOption[]>(
    () =>
      (imageChoices.data?.findImages.images ?? []).map((item) => ({
        value: item.id,
        label: item.title?.trim() || `Image #${item.id}`,
      })),
    [imageChoices.data]
  );
  const sceneOptions = useMemo<SelectOption[]>(
    () =>
      (sceneChoices.data?.findScenes.scenes ?? []).map((item) => ({
        value: item.id,
        label: item.title?.trim() || `Video #${item.id}`,
      })),
    [sceneChoices.data]
  );
  const performerOptions = useMemo<SelectOption[]>(
    () =>
      (performerChoices.data?.findPerformers.performers ?? []).map((item) => ({
        value: item.id,
        label: item.disambiguation
          ? `${item.name} (${item.disambiguation})`
          : item.name,
      })),
    [performerChoices.data]
  );

  const encodingImage = ImageUtils.usePasteImage((data) => {
    setImage(data);
    setImageTouched(true);
  });

  function onImageChange(event: React.FormEvent<HTMLInputElement>) {
    ImageUtils.onImageChange(event, (data) => {
      setImage(data);
      setImageTouched(true);
    });
  }

  async function save() {
    const name = values.name.trim();
    if (!name) return;
    setSaving(true);
    try {
      const common = {
        name,
        sort_name: values.sortName.trim(),
        description: values.description,
        favorite: values.favorite,
        aliases: aliasesFromText(values.aliases),
        parent_ids: values.parents.map((item) => item.id),
        child_ids: values.children.map((item) => item.id),
      };

      let saved: GQL.CopyrightDataFragment | undefined;
      if (create) {
        const result = await createCopyright({
          variables: {
            input: {
              ...common,
              image: imageTouched ? image : undefined,
            },
          },
        });
        saved = result.data?.copyrightCreate;
      } else if (copyright) {
        const result = await updateCopyright({
          variables: {
            input: {
              id: copyright.id,
              ...common,
              ...(imageTouched ? { image } : {}),
            },
          },
        });
        saved = result.data?.copyrightUpdate;
      }

      if (!saved) return;
      await Promise.all([
        updateImages({
          variables: { copyrightID: saved.id, imageIDs },
        }),
        updateScenes({
          variables: { copyrightID: saved.id, sceneIDs },
        }),
        updatePerformers({
          variables: { copyrightID: saved.id, performerIDs },
        }),
      ]);

      Toast.success(`Saved Copyright “${saved.name}”.`);
      if (create) {
        history.replace(`/copyrights/${saved.id}`);
      } else {
        onSaved?.(saved);
      }
    } catch (error) {
      Toast.error(error);
    } finally {
      setSaving(false);
    }
  }

  async function destroy() {
    if (!copyright) return;
    if (!window.confirm(`Delete Copyright “${values.name}”?`)) return;
    setSaving(true);
    try {
      await destroyCopyright({ variables: { id: copyright.id } });
      history.replace("/copyrights");
    } catch (error) {
      Toast.error(error);
      setSaving(false);
    }
  }

  const currentImage = imageTouched ? image : copyright?.image_path;

  return (
    <div className="container-fluid py-3">
      {create ? (
        <div className="d-flex align-items-center mb-3">
          <Button
            variant="secondary"
            onClick={() => history.push("/copyrights")}
          >
            Back
          </Button>
          <h2 className="mb-0 ml-3">New Copyright</h2>
        </div>
      ) : null}
      <Row>
        <Col md={4} xl={3} className="mb-3">
          <Card>
            <div
              style={{
                aspectRatio: "3 / 4",
                overflow: "hidden",
                background: "var(--secondary)",
              }}
            >
              {currentImage ? (
                <img
                  src={currentImage}
                  alt=""
                  style={{ width: "100%", height: "100%", objectFit: "cover" }}
                />
              ) : null}
            </div>
            <Card.Body>
              {encodingImage ? <Spinner animation="border" size="sm" /> : null}
              <ImageInput
                isEditing
                onImageChange={onImageChange}
                onImageURL={(url) => {
                  setImage(url);
                  setImageTouched(true);
                }}
                onReset={() => {
                  setImage(null);
                  setImageTouched(true);
                }}
              />
            </Card.Body>
          </Card>
        </Col>
        <Col md={8} xl={9}>
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
              rows={6}
              value={values.description}
              onChange={(event) =>
                setValues({ ...values, description: event.currentTarget.value })
              }
            />
          </Form.Group>
          <Form.Group>
            <Form.Label>Sort name</Form.Label>
            <Form.Control
              value={values.sortName}
              onChange={(event) =>
                setValues({ ...values, sortName: event.currentTarget.value })
              }
            />
          </Form.Group>
          <Form.Check
            className="mb-3"
            type="checkbox"
            label="Favorite"
            checked={values.favorite}
            onChange={(event) =>
              setValues({ ...values, favorite: event.currentTarget.checked })
            }
          />

          <Row>
            <Col lg={6}>
              <Form.Group>
                <Form.Label>Parent Copyrights</Form.Label>
                <CopyrightSelect
                  isMulti
                  values={values.parents}
                  excludeIds={copyright ? [copyright.id] : []}
                  onSelect={(parents) => setValues({ ...values, parents })}
                />
              </Form.Group>
            </Col>
            <Col lg={6}>
              <Form.Group>
                <Form.Label>Child Copyrights</Form.Label>
                <CopyrightSelect
                  isMulti
                  values={values.children}
                  excludeIds={copyright ? [copyright.id] : []}
                  onSelect={(children) => setValues({ ...values, children })}
                />
              </Form.Group>
            </Col>
          </Row>

          <Card className="mt-3">
            <Card.Header>Associated media</Card.Header>
            <Card.Body>
              <Form.Group>
                <Form.Label>Images</Form.Label>
                <Select<SelectOption, true>
                  isMulti
                  isLoading={imageChoices.loading}
                  options={imageOptions}
                  value={imageOptions.filter((item) =>
                    imageIDs.includes(item.value)
                  )}
                  onChange={(items) =>
                    setImageIDs(items.map((item) => item.value))
                  }
                  placeholder="Select Images"
                  classNamePrefix="react-select"
                />
              </Form.Group>
              <Form.Group>
                <Form.Label>Videos</Form.Label>
                <Select<SelectOption, true>
                  isMulti
                  isLoading={sceneChoices.loading}
                  options={sceneOptions}
                  value={sceneOptions.filter((item) =>
                    sceneIDs.includes(item.value)
                  )}
                  onChange={(items) =>
                    setSceneIDs(items.map((item) => item.value))
                  }
                  placeholder="Select Videos"
                  classNamePrefix="react-select"
                />
              </Form.Group>
              <Form.Group className="mb-0">
                <Form.Label>Characters</Form.Label>
                <Select<SelectOption, true>
                  isMulti
                  isLoading={performerChoices.loading}
                  options={performerOptions}
                  value={performerOptions.filter((item) =>
                    performerIDs.includes(item.value)
                  )}
                  onChange={(items) =>
                    setPerformerIDs(items.map((item) => item.value))
                  }
                  placeholder="Select Characters"
                  classNamePrefix="react-select"
                />
              </Form.Group>
            </Card.Body>
          </Card>

          <div className="mt-3 d-flex">
            <Button disabled={saving || !values.name.trim()} onClick={save}>
              Save
            </Button>
            {!create ? (
              <Button
                variant="secondary"
                className="ml-2"
                disabled={saving}
                onClick={onCancel}
              >
                Cancel
              </Button>
            ) : null}
            {copyright ? (
              <Button
                variant="danger"
                className="ml-auto"
                disabled={saving}
                onClick={destroy}
              >
                Delete
              </Button>
            ) : null}
          </div>
        </Col>
      </Row>
    </div>
  );
};

const CopyrightDetail: React.FC = () => {
  const { id } = useParams<{ id: string }>();
  const [editing, setEditing] = useState(false);
  const { data, loading, refetch } = GQL.useFindCopyrightQuery({
    variables: { id },
  });

  const copyright = data?.findCopyright;
  if (loading) return <Spinner animation="border" />;
  if (!copyright) {
    return <div className="alert alert-warning">Copyright not found.</div>;
  }

  if (editing) {
    return (
      <CopyrightEditor
        copyright={copyright}
        onCancel={() => setEditing(false)}
        onSaved={() => {
          void refetch();
          setEditing(false);
        }}
      />
    );
  }

  return (
    <div className="container-fluid py-3">
      <Helmet title={copyright.name} />
      <Card className="mb-3 overflow-hidden">
        <Row noGutters>
          <Col md={4} xl={3}>
            <img
              src={copyright.image_path}
              alt=""
              style={{
                width: "100%",
                height: "100%",
                minHeight: 260,
                objectFit: "cover",
              }}
            />
          </Col>
          <Col md={8} xl={9}>
            <Card.Body className="h-100 d-flex flex-column">
              <div className="d-flex align-items-start">
                <div>
                  <h2 className="mb-1">{copyright.name}</h2>
                  {copyright.aliases.length > 0 ? (
                    <div className="text-muted">
                      {copyright.aliases.join(" · ")}
                    </div>
                  ) : null}
                </div>
                <div className="ml-auto d-flex align-items-center">
                  {copyright.favorite ? <FavoriteIcon favorite /> : null}
                  <Button className="ml-2" onClick={() => setEditing(true)}>
                    Edit
                  </Button>
                </div>
              </div>
              {copyright.description ? (
                <p className="pre mt-3 mb-0">{copyright.description}</p>
              ) : null}
              <div className="mt-auto pt-3">
                {copyright.parents.length > 0 ? (
                  <div className="mb-1">
                    <span className="text-muted mr-2">Parents</span>
                    {copyright.parents.map((item) => (
                      <Badge
                        as={Link}
                        to={`/copyrights/${item.id}`}
                        variant="secondary"
                        className="mr-1"
                        key={item.id}
                      >
                        {item.name}
                      </Badge>
                    ))}
                  </div>
                ) : null}
                {copyright.children.length > 0 ? (
                  <div>
                    <span className="text-muted mr-2">Children</span>
                    {copyright.children.map((item) => (
                      <Badge
                        as={Link}
                        to={`/copyrights/${item.id}`}
                        variant="secondary"
                        className="mr-1"
                        key={item.id}
                      >
                        {item.name}
                      </Badge>
                    ))}
                  </div>
                ) : null}
              </div>
            </Card.Body>
          </Col>
        </Row>
      </Card>

      <Tabs defaultActiveKey="videos" id="copyright-media-tabs" mountOnEnter>
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
            {copyright.images.map((imageItem) => (
              <Col key={imageItem.id} sm={6} lg={4} xl={3} className="mb-3">
                <Card
                  as={Link}
                  to={`/images/${imageItem.id}`}
                  className="h-100 text-reset"
                >
                  <img
                    src={
                      imageItem.paths.thumbnail ?? imageItem.paths.image ?? ""
                    }
                    alt=""
                    style={{
                      width: "100%",
                      aspectRatio: "1 / 1",
                      objectFit: "cover",
                    }}
                  />
                  <Card.Body>
                    <TruncatedText
                      text={imageItem.title || `Image #${imageItem.id}`}
                    />
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
                  <img
                    src={performer.image_path ?? undefined}
                    alt=""
                    style={{
                      width: "100%",
                      aspectRatio: "3 / 4",
                      objectFit: "cover",
                    }}
                  />
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
    </div>
  );
};

const CopyrightRoutes: React.FC = () => (
  <>
    <Helmet title="Copyrights" />
    <Switch>
      <Route exact path="/copyrights" component={CopyrightList} />
      <Route exact path="/copyrights/new">
        <CopyrightEditor create />
      </Route>
      <Route exact path="/copyrights/:id" component={CopyrightDetail} />
    </Switch>
  </>
);

export default CopyrightRoutes;
