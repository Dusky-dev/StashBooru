import React, { useCallback, useEffect, useState } from "react";
import { Route, Switch } from "react-router-dom";
import { Helmet } from "react-helmet";
import { useTitleProps } from "src/hooks/title";
import { ListFilterModel } from "src/models/list-filter/filter";
import Tag from "./TagDetails/Tag";
import TagCreate from "./TagDetails/TagCreate";
import { FilteredTagList } from "./TagList";
import {
  applyCopyrightNamespaceFilter,
  CopyrightRoot,
} from "./copyrightFilter";

const Tags: React.FC = () => {
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
        ? applyCopyrightNamespaceFilter(filter, copyrightRoot, false)
        : filter,
    [copyrightRoot]
  );

  if (copyrightRoot === undefined) return null;
  return <FilteredTagList filterHook={filterHook} />;
};

const TagRoutes: React.FC = () => {
  const titleProps = useTitleProps({ id: "tags" });
  return (
    <>
      <Helmet {...titleProps} />
      <Switch>
        <Route exact path="/tags" component={Tags} />
        <Route exact path="/tags/new" component={TagCreate} />
        <Route path="/tags/:id/:tab?" component={Tag} />
      </Switch>
    </>
  );
};

export default TagRoutes;
