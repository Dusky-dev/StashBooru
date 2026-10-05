// Only complete stable versions from the StashBooru release track are comparable.
export function compareStableVersions(
  latest: string,
  current: string
): number | undefined {
  const stable = /^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$/;
  if (!stable.test(latest) || !stable.test(current)) return undefined;
  const a = latest.split(".").map(Number);
  const b = current.split(".").map(Number);
  if (![...a, ...b].every(Number.isSafeInteger)) return undefined;
  for (let i = 0; i < 3; i++) {
    if (a[i] !== b[i]) return a[i] > b[i] ? 1 : -1;
  }
  return 0;
}
