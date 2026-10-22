export type IdentityKind = "artist" | "character" | "copyright";
export const identityLabels = {
  artist: "Artist",
  character: "Character",
  copyright: "Copyright",
};
export const identityPaths = {
  artist: "/studios",
  character: "/performers",
  copyright: "/copyrights",
};
export interface ProviderIdentity {
  id: string;
  kind: IdentityKind;
  instance: string;
  sourceID: string;
  sourceName: string;
  engine: string;
  sourceURL: string;
  form: string;
  target: string;
  label: string;
  context: string;
  version: number;
  entityID: number;
  entityName: string;
}
export interface IdentityPage {
  items: ProviderIdentity[];
  next: string;
}
export interface IdentityCandidate {
  id: number;
  name: string;
  disambiguation: string;
  rejected: boolean;
}

export function identityName(value: string, kind: IdentityKind) {
  const name = value.replace(/_/g, " ").trim();
  return kind === "artist" ? name.replace(/\s*\(artist\)$/i, "").trim() : name;
}
export class IdentityRequestError extends Error {
  constructor(
    public status: number,
    message: string
  ) {
    super(message);
  }
}
export async function identityRequest<T>(
  path: string,
  signal: AbortSignal,
  input?: unknown
): Promise<T> {
  const response = await fetch(new URL(path, document.baseURI), {
    signal,
    credentials: "same-origin",
    ...(input === undefined
      ? {}
      : {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(input),
        }),
  });
  const data = await response.json();
  if (!response.ok)
    throw new IdentityRequestError(
      response.status,
      typeof data.error === "string"
        ? data.error
        : "Identity request failed. Reload and retry."
    );
  return data as T;
}
