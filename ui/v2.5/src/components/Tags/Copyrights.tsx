import React, { useEffect, useState } from "react";
import { Helmet } from "react-helmet";
import {
  Button,
  Card,
  Col,
  Form,
  ListGroup,
  Row,
  Spinner,
} from "react-bootstrap";
import { Link, Route, Switch, useHistory, useParams } from "react-router-dom";

import * as GQL from "src/core/generated-graphql";
import {
  Copyright,
  CopyrightSelect,
} from "src/components/Copyrights/CopyrightSelect";
import { useToast } from "src/hooks/Toast";

interface CopyrightFormValues {
  name: string;
  sortName: string;
  description: string;
  aliases: string;
  favorite: boolean;
  parents: Copyright[];
  children: Copyright[];
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
        <ListGroup>
          {copyrights.map((copyright) => (
            <ListGroup.Item
              key={copyright.id}
              as={Link}
              action
              to={`/copyrights/${copyright.id}`}
              className="d-flex align-items-center"
            >
              <span>{copyright.name}</span>
              <small className="text-muted ml-auto">
                {copyright.scene_count} Videos · {copyright.image_count} Images
              </small>
            </ListGroup.Item>
          ))}
          {copyrights.length === 0 && (
            <ListGroup.Item className="text-muted">
              No Copyrights found.
            </ListGroup.Item>
          )}
        </ListGroup>
      )}
    </div>
  );
};

const CopyrightEditor: React.FC<{ create?: boolean }> = ({ create = false }) => {
  const params = useParams<{ id?: string }>();
  const id = params.id ?? "";
  const history = useHistory();
  const Toast = useToast();
  const { data, loading } = GQL.useFindCopyrightQuery({
    variables: { id },
    skip: create || !id,
  });
  const [createCopyright] = GQL.useCopyrightCreateMutation();
  const [updateCopyright] = GQL.useCopyrightUpdateMutation();
  const [destroyCopyright] = GQL.useCopyrightDestroyMutation();
  const [values, setValues] = useState<CopyrightFormValues>(emptyValues);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (create) {
      setValues(emptyValues);
      return;
    }
    const copyright = data?.findCopyright;
    if (!copyright) return;
    setValues({
      name: copyright.name,
      sortName: copyright.sort_name,
      description: copyright.description,
      aliases: copyright.aliases.join("\n"),
      favorite: copyright.favorite,
      parents: copyright.parents,
      children: copyright.children,
    });
  }, [create, data]);

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
      if (create) {
        const result = await createCopyright({ variables: { input: common } });
        const created = result.data?.copyrightCreate;
        if (created) history.replace(`/copyrights/${created.id}`);
      } else {
        await updateCopyright({ variables: { input: { id, ...common } } });
      }
    } catch (error) {
      Toast.error(error);
    } finally {
      setSaving(false);
    }
  }

  async function destroy() {
    if (!id || create) return;
    if (!window.confirm(`Delete Copyright “${values.name}”?`)) return;
    setSaving(true);
    try {
      await destroyCopyright({ variables: { id } });
      history.replace("/copyrights");
    } catch (error) {
      Toast.error(error);
      setSaving(false);
    }
  }

  if (!create && loading) return <Spinner animation="border" />;
  if (!create && !data?.findCopyright) {
    return <div className="alert alert-warning">Copyright not found.</div>;
  }

  return (
    <div className="container-fluid py-3">
      <div className="d-flex align-items-center mb-3">
        <Button variant="secondary" onClick={() => history.push("/copyrights")}>
          Back
        </Button>
        <h2 className="mb-0 ml-3">
          {create ? "New Copyright" : values.name || "Copyright"}
        </h2>
      </div>
      <Card>
        <Card.Body>
          <Row>
            <Col lg={7}>
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
                <Form.Label>Sort name</Form.Label>
                <Form.Control
                  value={values.sortName}
                  onChange={(event) =>
                    setValues({ ...values, sortName: event.currentTarget.value })
                  }
                />
              </Form.Group>
              <Form.Group>
                <Form.Label>Aliases</Form.Label>
                <Form.Control
                  as="textarea"
                  rows={4}
                  value={values.aliases}
                  onChange={(event) =>
                    setValues({ ...values, aliases: event.currentTarget.value })
                  }
                  placeholder="One alias per line"
                />
              </Form.Group>
              <Form.Group>
                <Form.Label>Description</Form.Label>
                <Form.Control
                  as="textarea"
                  rows={5}
                  value={values.description}
                  onChange={(event) =>
                    setValues({
                      ...values,
                      description: event.currentTarget.value,
                    })
                  }
                />
              </Form.Group>
              <Form.Check
                type="checkbox"
                label="Favorite"
                checked={values.favorite}
                onChange={(event) =>
                  setValues({ ...values, favorite: event.currentTarget.checked })
                }
              />
            </Col>
            <Col lg={5}>
              <Form.Group>
                <Form.Label>Parent Copyrights</Form.Label>
                <CopyrightSelect
                  isMulti
                  values={values.parents}
                  excludeIds={id ? [id] : []}
                  onSelect={(parents) => setValues({ ...values, parents })}
                />
              </Form.Group>
              <Form.Group>
                <Form.Label>Child Copyrights</Form.Label>
                <CopyrightSelect
                  isMulti
                  values={values.children}
                  excludeIds={id ? [id] : []}
                  onSelect={(children) => setValues({ ...values, children })}
                />
              </Form.Group>
              {!create && data?.findCopyright && (
                <div className="text-muted mt-3">
                  <div>{data.findCopyright.scene_count} Videos</div>
                  <div>{data.findCopyright.image_count} Images</div>
                </div>
              )}
            </Col>
          </Row>
          <div className="mt-3 d-flex">
            <Button disabled={saving || !values.name.trim()} onClick={save}>
              Save
            </Button>
            {!create && (
              <Button
                variant="danger"
                className="ml-2"
                disabled={saving}
                onClick={destroy}
              >
                Delete
              </Button>
            )}
          </div>
        </Card.Body>
      </Card>
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
      <Route exact path="/copyrights/:id" component={CopyrightEditor} />
    </Switch>
  </>
);

export default CopyrightRoutes;
