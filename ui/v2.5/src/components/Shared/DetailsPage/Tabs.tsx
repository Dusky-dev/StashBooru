import { FormattedMessage } from "react-intl";
import { Counter } from "../Counter";
import { useCallback, useEffect } from "react";
import { useHistory } from "react-router-dom";
import { PatchComponent } from "src/patch";

const LAST_MEDIA_TAB_STORAGE_KEY = "stashbooru.details.lastMediaTab";
const rememberedMediaTabs = new Set([
  "scenes",
  "galleries",
  "images",
  "groups",
]);

function readRememberedMediaTab(validTabs: readonly string[]) {
  if (typeof window === "undefined") return undefined;
  try {
    const tab = window.localStorage.getItem(LAST_MEDIA_TAB_STORAGE_KEY);
    if (tab && rememberedMediaTabs.has(tab) && validTabs.includes(tab)) {
      return tab;
    }
  } catch {
    // localStorage can be unavailable in hardened/private browser contexts.
  }
  return undefined;
}

function rememberMediaTab(tab: string) {
  if (typeof window === "undefined" || !rememberedMediaTabs.has(tab)) return;
  try {
    window.localStorage.setItem(LAST_MEDIA_TAB_STORAGE_KEY, tab);
  } catch {
    // Treat persistence as best-effort; navigation should still work normally.
  }
}

export const TabTitleCounter: React.FC<{
  messageID: string;
  count: number;
  abbreviateCounter: boolean;
}> = PatchComponent(
  "TabTitleCounter",
  ({ messageID, count, abbreviateCounter }) => {
    return (
      <>
        <FormattedMessage id={messageID} />
        <Counter count={count} abbreviateCounter={abbreviateCounter} hideZero />
      </>
    );
  }
);

export function useTabKey(props: {
  tabKey: string | undefined;
  validTabs: readonly string[];
  defaultTabKey: string;
  baseURL: string;
}) {
  const { tabKey, validTabs, defaultTabKey, baseURL } = props;

  const history = useHistory();
  const explicitTabKey =
    tabKey && tabKey !== "default" && validTabs.includes(tabKey)
      ? tabKey
      : undefined;
  const activeTabKey =
    explicitTabKey ?? readRememberedMediaTab(validTabs) ?? defaultTabKey;

  const setTabKey = useCallback(
    (newTabKey: string | null) => {
      if (
        !newTabKey ||
        newTabKey === "default" ||
        !validTabs.includes(newTabKey)
      ) {
        newTabKey = defaultTabKey;
      }
      rememberMediaTab(newTabKey);
      if (newTabKey === activeTabKey) return;

      history.replace(`${baseURL}/${newTabKey}`);
    },
    [activeTabKey, defaultTabKey, validTabs, history, baseURL]
  );

  useEffect(() => {
    rememberMediaTab(activeTabKey);
    if (tabKey !== activeTabKey) {
      history.replace(`${baseURL}/${activeTabKey}`);
    }
  }, [activeTabKey, baseURL, history, tabKey]);

  return { activeTabKey, setTabKey };
}
