import React from "react";
import { Helmet } from "react-helmet";
import { Route, Switch } from "react-router-dom";
import { useTitleProps } from "src/hooks/title";

import CopyrightRoutes from "src/components/Tags/Copyrights";
import CopyrightList from "./CopyrightList";

const CopyrightsRouter: React.FC = () => {
  const titleProps = useTitleProps({ id: "copyrights" });
  return (
    <>
      <Helmet defer={false} {...titleProps} />
      <Switch>
        <Route exact path="/copyrights" component={CopyrightList} />
        <Route path="/copyrights" component={CopyrightRoutes} />
      </Switch>
    </>
  );
};

export default CopyrightsRouter;
