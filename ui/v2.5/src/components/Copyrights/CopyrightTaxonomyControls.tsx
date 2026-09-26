import React, { useEffect, useState } from "react";
import { Button } from "react-bootstrap";
import { Link } from "react-router-dom";
import * as GQL from "src/core/generated-graphql";
import { useToast } from "src/hooks/Toast";

type CopyrightValue = Pick<
  GQL.Copyright,
  "id" | "name" | "sort_name" | "aliases" | "favorite"
>;

export const CopyrightBreadcrumb: React.FC<{
  items: CopyrightValue[];
}> = ({ items }) => {
  if (items.length <= 1) return null;

  return (
    <nav
      className="copyright-breadcrumb mb-2"
      aria-label="Folder path"
    >
      {items.map((item, index) => (
        <React.Fragment key={item.id}>
          {index > 0 ? <span className="mx-1 text-muted">/</span> : null}
          <Link to={`/copyrights/${item.id}`}>{item.name}</Link>
        </React.Fragment>
      ))}
    </nav>
  );
};

export const CopyrightChildrenOrderControl: React.FC<{
  parentID: string;
  orderedChildren: CopyrightValue[];
}> = ({ parentID, orderedChildren }) => {
  const Toast = useToast();
  const [items, setItems] = useState(orderedChildren);
  const [dirty, setDirty] = useState(false);
  const [saving, setSaving] = useState(false);
  const [updateOrder] = GQL.useCopyrightChildrenOrderUpdateMutation();

  useEffect(() => {
    setItems(orderedChildren);
    setDirty(false);
  }, [orderedChildren]);

  if (orderedChildren.length < 2) return null;

  function move(index: number, delta: number) {
    const target = index + delta;
    if (target < 0 || target >= items.length) return;
    const next = [...items];
    [next[index], next[target]] = [next[target], next[index]];
    setItems(next);
    setDirty(true);
  }

  async function save() {
    setSaving(true);
    try {
      await updateOrder({
        variables: {
          parentID,
          childIDs: items.map((item) => item.id),
        },
      });
      setDirty(false);
    } catch (error) {
      Toast.error(error);
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="copyright-child-order mb-3">
      <div className="font-weight-bold mb-1">Subfolder order</div>
      {items.map((item, index) => (
        <div
          key={item.id}
          className="d-flex align-items-center justify-content-between py-1"
        >
          <Link to={`/copyrights/${item.id}`}>{item.name}</Link>
          <div>
            <Button
              size="sm"
              variant="secondary"
              className="mr-1"
              disabled={saving || index === 0}
              onClick={() => move(index, -1)}
            >
              ↑
            </Button>
            <Button
              size="sm"
              variant="secondary"
              disabled={saving || index === items.length - 1}
              onClick={() => move(index, 1)}
            >
              ↓
            </Button>
          </div>
        </div>
      ))}
      <Button
        className="mt-2"
        size="sm"
        variant="primary"
        disabled={saving || !dirty}
        onClick={() => void save()}
      >
        Save subfolder order
      </Button>
    </div>
  );
};
