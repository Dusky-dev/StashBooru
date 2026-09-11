import React, { useCallback, useEffect, useState } from "react";
import { Helmet } from "react-helmet";
import { Route, Switch } from "react-router-dom";

import { ListFilterModel } from "src/models/list-filter/filter";
import Tag from "./TagDetails/Tag";
import { FilteredTagList } from "./TagList";
import {
  applyCopyrightNamespaceFilter,
  CopyrightRoot,
} from "./copyrightFilter";

const CopyrightList: React.FC = () => {
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

const CopyrightRoutes: React.FC = () => (
  <>
    <Helmet title="Copyrights" />
    <Switch>
      <Route exact path="/copyrights" component={CopyrightList} />
      <Route path="/copyrights/:id/:tab?" component={Tag} />
    </Switch>
  </>
);

export default CopyrightRoutes;
