import React, { useCallback, useEffect, useState } from "react";
import { Helmet } from "react-helmet";
import { Route, Switch } from "react-router-dom";

import { ListFilterModel } from "src/models/list-filter/filter";
import Tag from "./TagDetails/Tag";
import TagCreate from "./TagDetails/TagCreate";
import { FilteredTagList } from "./TagList";
import {
  applyCopyrightNamespaceFilter,
  CopyrightRoot,
} from "./copyrightFilter";

function useCopyrightRoot() {
  const [copyrightRoot, setCopyrightRoot] = useState<CopyrightRoot | null>();

  useEffect(() => {
    void (async () => {
      try {
        const response = await fetch(
          "image/visual-similarity/camie/copyright-root"
        );
        if (!response.ok) throw new Error(await response.text());
        setCopyrightRoot((await response.json()) as CopyrightRoot);
      } catch {
        setCopyrightRoot(null);
      }
    })();
  }, []);

  return copyrightRoot;
}

const CopyrightList: React.FC = () => {
  const copyrightRoot = useCopyrightRoot();

  const filterHook = useCallback(
    (filter: ListFilterModel) =>
      copyrightRoot
        ? applyCopyrightNamespaceFilter(filter, copyrightRoot, true)
        : filter,
    [copyrightRoot]
  );

  if (copyrightRoot === undefined) return null;
  return <FilteredTagList filterHook={filterHook} />;
};

const CopyrightCreate: React.FC = () => {
  const copyrightRoot = useCopyrightRoot();

  if (copyrightRoot === undefined) return null;
  if (copyrightRoot === null) {
    return (
      <div className="alert alert-danger">
        Unable to initialize the Copyright namespace.
      </div>
    );
  }

  return (
    <TagCreate
      basePath="/copyrights"
      requiredParentId={String(copyrightRoot.id)}
      entityName="Copyright"
    />
  );
};

const CopyrightRoutes: React.FC = () => (
  <>
    <Helmet title="Copyrights" />
    <Switch>
      <Route exact path="/copyrights" component={CopyrightList} />
      <Route exact path="/copyrights/new" component={CopyrightCreate} />
      <Route path="/copyrights/:id/:tab?" component={Tag} />
    </Switch>
  </>
);

export default CopyrightRoutes;
