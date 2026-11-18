import React, { useCallback } from "react";
import { useSettingsOptional } from "./context";

const storageKey = (id: string) => `settings-group-open:${id}`;

function readOpen(id: string): boolean {
  try {
    return window.localStorage.getItem(storageKey(id)) === "1";
  } catch {
    return false;
  }
}

function writeOpen(id: string, open: boolean) {
  try {
    if (open) window.localStorage.setItem(storageKey(id), "1");
    else window.localStorage.removeItem(storageKey(id));
  } catch {
    // Private browsing or full storage: the group still works, unremembered.
  }
}

// Native details keeps the original controls and results mounted while collapsed.
export const SettingsDisclosure: React.FC<{
  id: string;
  title: string;
  description?: string;
  /** Restore the group's last open/closed state in this browser. */
  remember?: boolean;
  /** Show only in Advanced mode. */
  advanced?: boolean;
  children?: React.ReactNode;
}> = ({ id, title, description, remember, advanced, children }) => {
  const { advancedMode } = useSettingsOptional();
  const hidden = !!advanced && !advancedMode;
  // Restore when the element mounts; a hash link may already have opened it.
  const restore = useCallback(
    (details: HTMLDetailsElement | null) => {
      if (remember && details && readOpen(id)) details.open = true;
    },
    [id, remember]
  );

  if (hidden) return null;

  return (
    <details
      className="settings-details settings-disclosure"
      id={id}
      ref={restore}
      onToggle={
        remember
          ? (event) => writeOpen(id, event.currentTarget.open)
          : undefined
      }
    >
      <summary
        aria-labelledby={`${id}-title`}
        aria-describedby={description ? `${id}-help` : undefined}
      >
        <span className="settings-disclosure-title" id={`${id}-title`}>
          {title}
        </span>
        {description && (
          <span className="settings-disclosure-help" id={`${id}-help`}>
            {description}
          </span>
        )}
      </summary>
      <div className="settings-disclosure-content">{children}</div>
    </details>
  );
};
