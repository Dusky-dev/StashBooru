import {
  RefObject,
  useContext,
  useEffect,
  useLayoutEffect,
  useRef,
} from "react";
import { LightboxVisibilityContext } from "./Lightbox/context";

// Card/wall clips belong to the visible page. They must not play underneath
// a foreground viewer or resume because a queued observer fires after opening.
export function usePreviewPlayback(
  ref: RefObject<HTMLVideoElement>,
  enabled: boolean,
  source?: string
) {
  const viewerOpen = useContext(LightboxVisibilityContext);
  const allowed = useRef(false);
  const synchronize = useRef<() => void>(() => {});

  useLayoutEffect(() => {
    allowed.current = enabled && !viewerOpen;
    synchronize.current();
  }, [enabled, viewerOpen]);

  useEffect(() => {
    const video = ref.current;
    if (!video || !source) return;
    let visible = false;
    let disposed = false;
    const sync = () => {
      if (disposed) return;
      if (allowed.current && visible && !document.hidden && video.isConnected) {
        void video.play()?.catch(() => {});
      } else {
        video.pause();
      }
    };
    synchronize.current = sync;
    const observer = new IntersectionObserver((entries) => {
      visible = entries.some((entry) => entry.isIntersecting);
      sync();
    });
    // Grid videos stay offscreen until their first decoded frame is ready.
    observer.observe(video.parentElement ?? video);
    document.addEventListener("visibilitychange", sync);
    sync();
    return () => {
      disposed = true;
      observer.disconnect();
      document.removeEventListener("visibilitychange", sync);
      synchronize.current = () => {};
      video.pause();
    };
  }, [ref, source]);
}
