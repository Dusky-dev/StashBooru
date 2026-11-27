import type { ReactNode } from "react";
import { ConfigurationContext, type IContext } from "../../../src/hooks/Config";

// Following is experimental and off by default; fixtures that exercise it
// turn it on the way Settings → Experimental does.
const enabled = {
  configuration: { ui: { followingEnabled: true } },
} as unknown as IContext;

export function FollowingEnabled({ children }: { children: ReactNode }) {
  return (
    <ConfigurationContext.Provider value={enabled}>
      {children}
    </ConfigurationContext.Provider>
  );
}
