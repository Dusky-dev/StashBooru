import React, { useCallback } from "react";
import { useSettingsOptional } from "./context";
import { CompactHelpProvider } from "./Inputs";

const storageKey = (id: string) => `settings-group-open:${id}`;

function readOpen(id: string, defaultOpen: boolean): boolean {
  try {
    const value = window.localStorage.getItem(storageKey(id));
    return value === null ? defaultOpen : value === "1";
  } catch {
    return defaultOpen;
  }
}

function writeOpen(id: string, open: boolean, defaultOpen: boolean) {
  try {
    if (open === defaultOpen) window.localStorage.removeItem(storageKey(id));
    else window.localStorage.setItem(storageKey(id), open ? "1" : "0");
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
  /** Open on first visit, before the user has toggled the group. */
  defaultOpen?: boolean;
  /** Show only in Advanced mode. */
  advanced?: boolean;
  /** Show setting descriptions as help buttons beside their headings. */
  compactHelp?: boolean;
  children?: React.ReactNode;
}> = ({
  id,
  title,
  description,
  remember,
  defaultOpen = false,
  advanced,
  compactHelp,
  children,
}) => {
  const { advancedMode } = useSettingsOptional();
  const hidden = !!advanced && !advancedMode;
  // Restore when the element mounts; a hash link may already have opened it.
  const restore = useCallback(
    (details: HTMLDetailsElement | null) => {
      if (details && (remember ? readOpen(id, defaultOpen) : defaultOpen))
        details.open = true;
    },
    [id, remember, defaultOpen]
  );

  if (hidden) return null;

  return (
    <details
      className="settings-details settings-disclosure"
      id={id}
      ref={restore}
      onToggle={
        remember
          ? (event) => writeOpen(id, event.currentTarget.open, defaultOpen)
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
      <div className="settings-disclosure-content">
        <CompactHelpProvider value={!!compactHelp}>
          {children}
        </CompactHelpProvider>
      </div>
    </details>
  );
};
