import { useEffect, useState } from "react";
import { Dropdown } from "react-bootstrap";
import { useIntl } from "react-intl";
import { Link } from "react-router-dom";
import { identityRequest } from "../Identities/identityApi";
import { canFollowTarget } from "./followContext";

/** The part of a saved follow needed to show Follow / Following. */
export interface FollowSummary {
  id: string;
  instance: string;
  kind: string;
  target: string;
  active: boolean;
  unseen?: number;
}

// One shared, short-lived copy of the follow list: a post preview can show
// dozens of tag menus. Refreshed when the tab regains focus, so a follow
// saved in the Following tab shows here on return.
let cache: { at: number; value: Promise<FollowSummary[]> } | undefined;
const listeners = new Set<() => void>();

function loadFollows() {
  if (!cache || Date.now() - cache.at > 15000) {
    const value = identityRequest<FollowSummary[]>(
      "following/follows",
      new AbortController().signal
    ).catch(() => [] as FollowSummary[]);
    cache = { at: Date.now(), value };
  }
  return cache.value;
}

const changedEvent = "stashbooru-following-changed";

/** Call after seen decisions or follows change, so counts refresh at once. */
export function notifyFollowingChanged() {
  window.dispatchEvent(new Event(changedEvent));
}

if (typeof window !== "undefined")
  for (const name of ["focus", changedEvent])
    window.addEventListener(name, () => {
      cache = undefined;
      for (const listener of listeners) listener();
    });

export function useFollows() {
  const [follows, setFollows] = useState<FollowSummary[]>();
  useEffect(() => {
    let live = true;
    const refresh = () =>
      void loadFollows().then((value) => {
        if (live) setFollows(value);
      });
    refresh();
    listeners.add(refresh);
    return () => {
      live = false;
      listeners.delete(refresh);
    };
  }, []);
  return follows;
}

export function findFollow(
  follows: FollowSummary[] | undefined,
  instance: string,
  kind: string,
  target: string
) {
  return follows?.find(
    (follow) =>
      follow.active &&
      follow.instance === instance &&
      follow.kind === kind &&
      follow.target === target
  );
}

export interface FollowActionProps {
  instance: string;
  engine?: string;
  kind: string;
  target: string;
  /** A stored source name, needed to start a new follow from it. */
  identityID?: string | number;
  /** The site's display name, for the accessible label. */
  siteName: string;
  /** Render as a menu item (Booru browser tag menu) or a small button. */
  as?: "menu" | "button";
}

/**
 * Following → that follow's feed. Follow → a prefilled draft in a new tab
 * (nothing is saved until you press Follow there). Nothing for sites whose
 * feeds cannot be checked.
 */
export function FollowAction({
  instance,
  engine,
  kind,
  target,
  identityID,
  siteName,
  as = "button",
}: FollowActionProps) {
  const intl = useIntl();
  const follows = useFollows();
  if (!follows) return null;
  const follow = findFollow(follows, instance, kind, target);
  let to: string;
  let label: string;
  let newTab = false;
  if (follow) {
    to = `/following?${new URLSearchParams({ follow: follow.id })}`;
    label = follow.unseen
      ? intl.formatMessage(
          { id: "following_action.following_new" },
          { count: follow.unseen }
        )
      : intl.formatMessage({ id: "following_action.following" });
  } else if (identityID && canFollowTarget(engine, kind, target)) {
    to = `/following?fromIdentity=${encodeURIComponent(String(identityID))}#following-manager`;
    label = intl.formatMessage({ id: "following_action.follow" });
    newTab = true;
  } else return null;
  const title = intl.formatMessage(
    {
      id: follow
        ? "following_action.following_title"
        : "following_action.follow_title",
    },
    { target, site: siteName }
  );
  const tab = newTab ? { target: "_blank", rel: "noopener noreferrer" } : {};
  if (as === "menu")
    return (
      <Dropdown.Item
        as={Link}
        role="menuitem"
        to={to}
        title={title}
        aria-label={title}
        {...tab}
      >
        {label}
      </Dropdown.Item>
    );
  return (
    <Link
      className={`btn btn-sm ${follow ? "btn-outline-primary" : "btn-secondary"}`}
      to={to}
      title={title}
      aria-label={title}
      {...tab}
    >
      {label}
    </Link>
  );
}

/** Automatic check status, and new posts across all follows. */
export interface FollowSchedule {
  intervalHours: number;
  markSeenOnView?: boolean;
  unseen: number;
  running: boolean;
  nextRun?: string;
  lastRun?: {
    startedAt: string;
    finishedAt?: string;
    checked: number;
    newPosts: number;
    failed: number;
    note?: string;
  };
}

/**
 * New posts across all follows, for the navbar badge. Refreshed every five
 * minutes and when the tab regains focus; 0 if Following is unavailable.
 */
export function useFollowingUnseen() {
  const [unseen, setUnseen] = useState(0);
  useEffect(() => {
    const controller = new AbortController();
    const refresh = () =>
      void identityRequest<FollowSchedule>(
        "following/schedule",
        controller.signal
      )
        .then((value) => {
          if (!controller.signal.aborted) setUnseen(value.unseen ?? 0);
        })
        .catch(() => {});
    refresh();
    const timer = setInterval(refresh, 5 * 60 * 1000);
    window.addEventListener("focus", refresh);
    window.addEventListener(changedEvent, refresh);
    return () => {
      controller.abort();
      clearInterval(timer);
      window.removeEventListener("focus", refresh);
      window.removeEventListener(changedEvent, refresh);
    };
  }, []);
  return unseen;
}

/** The "Mark posts seen when shown" setting; false until loaded. */
export function useMarkSeenOnView() {
  const [value, setValue] = useState(false);
  useEffect(() => {
    const controller = new AbortController();
    const refresh = () =>
      void identityRequest<FollowSchedule>(
        "following/schedule",
        controller.signal
      )
        .then((schedule) => {
          if (!controller.signal.aborted) setValue(!!schedule.markSeenOnView);
        })
        .catch(() => {});
    refresh();
    window.addEventListener(changedEvent, refresh);
    return () => {
      controller.abort();
      window.removeEventListener(changedEvent, refresh);
    };
  }, []);
  return value;
}
