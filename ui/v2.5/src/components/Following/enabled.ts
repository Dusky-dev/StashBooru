import { useConfigurationContextOptional } from "src/hooks/Config";

/**
 * Following is experimental and off by default (Settings → Experimental).
 * While off, its navbar link, page, Follow buttons and Followed posts tabs
 * are hidden and nothing is fetched; saved follows are kept.
 */
export function useFollowingEnabled() {
  const context = useConfigurationContextOptional();
  return context?.configuration?.ui?.followingEnabled === true;
}
