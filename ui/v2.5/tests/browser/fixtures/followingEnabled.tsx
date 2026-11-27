import type { ReactNode } from "react";
import { ConfigurationContext, type IContext } from "../../../src/hooks/Config";

// Following is experimental and off by default; fixtures that exercise it
// turn it on the way Settings → Experimental does. ?fixtureFollowing=off
// shows the default state.
const enabled = {
  configuration: {
    ui: {
      followingEnabled:
        new URLSearchParams(window.location.search).get("fixtureFollowing") !==
        "off",
    },
  },
} as unknown as IContext;

export function FollowingEnabled({ children }: { children: ReactNode }) {
  return (
    <ConfigurationContext.Provider value={enabled}>
      {children}
    </ConfigurationContext.Provider>
  );
}
