import React from "react";
import { Route, Switch } from "react-router-dom";

import CopyrightRoutes from "src/components/Tags/Copyrights";
import CopyrightList from "./CopyrightList";

const CopyrightsRouter: React.FC = () => (
  <Switch>
    <Route exact path="/copyrights" component={CopyrightList} />
    <Route path="/copyrights" component={CopyrightRoutes} />
  </Switch>
);

export default CopyrightsRouter;
