import { useEffect, useLayoutEffect, useRef } from "react";
import { useHistory, useLocation } from "react-router-dom";
import type { ListFilterModel } from "src/models/list-filter/filter";
import type { View } from "./views";

interface ScrollPosition {
  pathname: string;
  query: string;
  top: number;
}

const positions = new Map<string, ScrollPosition>();

function scrollKey(
  view: View,
  { key, pathname, search }: ReturnType<typeof useLocation>
) {
  return `${view}:${pathname}:${key ?? search}`;
}

export function useListScrollRestoration({
  view,
  filter,
  ready,
  active = true,
}: {
  view: View;
  filter: ListFilterModel;
  ready: boolean;
  active?: boolean;
}) {
  const history = useHistory();
  const location = useLocation();
  const { pathname, search } = location;
  const { action } = history;
  const key = scrollKey(view, location);
  const query = filter.makeQueryParameters();
  const list = useRef<HTMLDivElement>(null);
  const pending = useRef<ScrollPosition>();
  const handled = useRef<{ location: typeof location; active: boolean }>();

  useLayoutEffect(() => {
    // Suspense hides a list while a lazy page loads and re-runs its layout
    // effects when Back reveals it. Arm a restore once per history entry.
    if (
      handled.current?.location === location &&
      handled.current.active === active
    ) {
      return;
    }
    handled.current = { location, active };
    if (active && action === "POP") {
      pending.current = positions.get(key);
    } else if (
      !active ||
      action !== "REPLACE" ||
      pending.current?.pathname !== pathname ||
      pending.current?.query !== search.slice(1)
    ) {
      // A REPLACE that normalizes the saved filter can change the history key
      // during Back. Only carry the position into that same list and filter.
      pending.current = undefined;
    }
  }, [active, action, key, location, pathname, search]);

  // biome-ignore lint/correctness/useExhaustiveDependencies: a list revealed by Back keeps its key, query and readiness; the new location restarts the restore.
  useEffect(() => {
    if (!active || !ready) return;
    const element = list.current;
    const saved = pending.current;
    let stopWatching: (() => void) | undefined;
    function isCurrentEntry() {
      return scrollKey(view, history.location) === key;
    }
    function remember() {
      if (pending.current || !isCurrentEntry()) return;
      positions.delete(key);
      positions.set(key, { pathname, query, top: window.scrollY });
      if (positions.size > 32) positions.delete(positions.keys().next().value!);
    }
    if (element && saved && saved.query === query) {
      const top = saved.top;
      let quietTimer: number | undefined;
      function restore() {
        stopWatching?.();
        if (pending.current !== saved || !isCurrentEntry()) return;
        pending.current = undefined;
        window.scrollTo(0, top);
        remember();
      }
      function afterLayout() {
        window.clearTimeout(quietTimer);
        // Grid measures its responsive card widths after mounting. Wait for
        // those layout changes before applying the saved offset.
        quietTimer = window.setTimeout(restore, 100);
      }
      const observer = new ResizeObserver(afterLayout);
      const deadline = window.setTimeout(restore, 500);
      stopWatching = () => {
        observer.disconnect();
        window.clearTimeout(quietTimer);
        window.clearTimeout(deadline);
      };
      observer.observe(element);
      afterLayout();
    }
    function onInteraction() {
      if (!isCurrentEntry()) return;
      pending.current = undefined;
      stopWatching?.();
      remember();
    }

    remember();
    window.addEventListener("scroll", remember, { passive: true });
    // A Link can navigate before the browser dispatches the scroll event for
    // its final focus/position. Save on click, before routing switches views.
    window.addEventListener("click", onInteraction, true);
    window.addEventListener("wheel", onInteraction, { passive: true });
    window.addEventListener("touchstart", onInteraction, { passive: true });
    return () => {
      stopWatching?.();
      window.removeEventListener("scroll", remember);
      window.removeEventListener("click", onInteraction, true);
      window.removeEventListener("wheel", onInteraction);
      window.removeEventListener("touchstart", onInteraction);
    };
  }, [active, history, key, location, pathname, query, ready, view]);

  return list;
}
