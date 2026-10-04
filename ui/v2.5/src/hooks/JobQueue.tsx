import { useApolloClient } from "@apollo/client";
import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useLayoutEffect,
  useReducer,
  useRef,
  useState,
} from "react";
import { getPlatformURL } from "src/core/createClient";
import { getWSClient } from "src/core/StashService";
import * as GQL from "src/core/generated-graphql";
import {
  JobQueueState,
  jobQueueReducer,
  readJobNotices,
  serializeJobNotices,
} from "./jobQueueState";

interface QueueContext {
  entries: JobQueueState["entries"];
  error: string;
  storageWarning: boolean;
  dismiss: (key?: string) => void;
  refresh: () => void;
}
const Context = createContext<QueueContext | null>(null);

// Stay subscribed outside Settings so failures are also captured while browsing.
// sessionStorage retains them across refreshes in this tab, scoped to the server.
export const JobQueueProvider: React.FC = ({ children }) => {
  const client = useApolloClient();
  const storageKey = `stashbooru.failedJobs:${getPlatformURL().href}`;
  const [storageWarning, setStorageWarning] = useState(false);
  const [state, dispatch] = useReducer(jobQueueReducer, storageKey, (key) => {
    try {
      return readJobNotices(window.sessionStorage.getItem(key));
    } catch {
      return readJobNotices(null);
    }
  });
  const [error, setError] = useState("");
  const refreshRef = useRef<() => void>();
  const refresh = useCallback(() => refreshRef.current?.(), []);
  const dismiss = useCallback(
    (key?: string) => dispatch({ type: "dismiss", key }),
    []
  );
  const serialized = serializeJobNotices(state);

  useLayoutEffect(() => {
    try {
      window.sessionStorage.setItem(storageKey, serialized);
      setStorageWarning(false);
    } catch {
      setStorageWarning(true);
    }
  }, [storageKey, serialized]);

  useEffect(() => {
    let disposed = false;
    let revision = 0;
    let request = 0;
    async function load() {
      const currentRequest = ++request;
      const snapshotRevision = revision;
      try {
        const result = await client.query<GQL.JobQueueQuery>({
          query: GQL.JobQueueDocument,
          fetchPolicy: "no-cache",
        });
        if (disposed || currentRequest !== request) return;
        dispatch({
          type: "snapshot",
          jobs: result.data.jobQueue ?? [],
          revision: snapshotRevision,
        });
        setError("");
      } catch (e) {
        if (!disposed && currentRequest === request) setError(String(e));
      }
    }
    const subscription = client
      .subscribe<GQL.JobsSubscribeSubscription>({
        query: GQL.JobsSubscribeDocument,
      })
      .subscribe({
        next: ({ data }) => {
          if (disposed || !data) return;
          dispatch({
            type: "event",
            job: data.jobsSubscribe.job,
            removed: data.jobsSubscribe.type === GQL.JobStatusUpdateType.Remove,
            revision: ++revision,
            now: Date.now(),
          });
        },
        error: (e) => {
          if (!disposed) setError(String(e));
        },
      });
    refreshRef.current = () => void load();
    const disposeConnected = getWSClient().on("connected", refresh);
    void load();
    return () => {
      disposed = true;
      refreshRef.current = undefined;
      subscription.unsubscribe();
      disposeConnected();
    };
  }, [client, refresh]);

  useEffect(() => {
    const deadlines = state.entries.flatMap(({ expiresAt }) =>
      expiresAt === undefined ? [] : [expiresAt]
    );
    if (!deadlines.length) return;
    const timer = window.setTimeout(
      () => dispatch({ type: "expire", now: Date.now() }),
      Math.max(0, Math.min(...deadlines) - Date.now())
    );
    return () => window.clearTimeout(timer);
  }, [state.entries]);

  return (
    <Context.Provider
      value={{
        entries: state.entries,
        error,
        storageWarning,
        dismiss,
        refresh,
      }}
    >
      {children}
    </Context.Provider>
  );
};

export function useJobNotices() {
  const context = useContext(Context);
  if (!context) throw new Error("useJobNotices requires a JobQueueProvider");
  return context;
}
