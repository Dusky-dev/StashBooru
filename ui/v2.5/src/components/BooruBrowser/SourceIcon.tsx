import { useState } from "react";
import { type BooruSource } from "./browser";

export function SourceIcon({ source }: { source: BooruSource }) {
  const [failed, setFailed] = useState(false);
  const origin =
    source.engine === "pixiv"
      ? "https://www.pixiv.net"
      : new URL(source.url).origin;
  return failed ? (
    <span
      className="booru-source-icon booru-source-monogram"
      aria-hidden="true"
    >
      {source.name.slice(0, 1).toUpperCase()}
    </span>
  ) : (
    <img
      className="booru-source-icon"
      src={`${origin}/favicon.ico`}
      alt=""
      loading="lazy"
      referrerPolicy="no-referrer"
      onError={() => setFailed(true)}
    />
  );
}
