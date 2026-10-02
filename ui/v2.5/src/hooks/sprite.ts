import { useEffect, useState } from "react";
import { WebVTT } from "videojs-vtt.js";

export interface ISceneSpriteInfo {
  url: string;
  start: number;
  end: number;
  x: number;
  y: number;
  w: number;
  h: number;
}

function getSpriteInfo(vttPath: string, response: string) {
  const sprites: ISceneSpriteInfo[] = [];

  const parser = new WebVTT.Parser(window, WebVTT.StringDecoder());
  parser.oncue = (cue: VTTCue) => {
    const match = cue.text.match(/^([^#]*)#xywh=(\d+),(\d+),(\d+),(\d+)$/i);
    if (!match) return;

    sprites.push({
      url: new URL(match[1], vttPath).href,
      start: cue.startTime,
      end: cue.endTime,
      x: Number(match[2]),
      y: Number(match[3]),
      w: Number(match[4]),
      h: Number(match[5]),
    });
  };
  parser.parse(response);
  parser.flush();

  return sprites;
}

// useSpriteInfo is a hook that fetches a VTT file and parses it for sprite information.
// If the vttPath is undefined, the hook will return undefined.
// If the response is not ok, the hook will return null. This usually indicates missing sprite.
export function useSpriteInfo(vttPath: string | undefined) {
  const [result, setResult] = useState<{
    path: string;
    sprites: ISceneSpriteInfo[] | null;
  }>();

  useEffect(() => {
    if (!vttPath) return;
    const controller = new AbortController();
    async function loadSprites(path: string) {
      try {
        const response = await fetch(path, { signal: controller.signal });
        const sprites = response.ok
          ? getSpriteInfo(path, await response.text())
          : null;
        if (!controller.signal.aborted) setResult({ path, sprites });
      } catch {
        if (!controller.signal.aborted) setResult({ path, sprites: null });
      }
    }
    void loadSprites(vttPath);
    return () => controller.abort();
  }, [vttPath]);

  // Clear old thumbnails during render, before the new fetch effect runs.
  return result && result.path === vttPath ? result.sprites : undefined;
}
