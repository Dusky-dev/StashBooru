import React, { useCallback, useContext, useMemo, useState } from "react";
import { Button } from "react-bootstrap";
import { FormattedMessage, useIntl } from "react-intl";
import { faEye, faXmark } from "@fortawesome/free-solid-svg-icons";
import { Icon } from "src/components/Shared/Icon";

const hiddenKey = "stashbooru.imageDetails.hidden";

function readHidden(): ReadonlySet<string> {
  try {
    const value: unknown = JSON.parse(localStorage.getItem(hiddenKey) ?? "[]");
    return new Set(
      Array.isArray(value)
        ? value.filter((item): item is string => typeof item === "string")
        : []
    );
  } catch {
    return new Set();
  }
}

function storeHidden(hidden: ReadonlySet<string>) {
  try {
    localStorage.setItem(hiddenKey, JSON.stringify([...hidden]));
  } catch {
    // Private windows may refuse storage; the choice lasts for this visit.
  }
}

interface IDetailsCustomize {
  editing: boolean;
  hidden: ReadonlySet<string>;
  toggle: (id: string) => void;
}

// Without a provider (e.g. the Video page) items render normally.
const DetailsCustomizeContext = React.createContext<
  IDetailsCustomize | undefined
>(undefined);

/** Whether the Image details panel is being customized, and what it hides. */
export function useImageDetailsCustomize() {
  const [editing, setEditing] = useState(false);
  const [hidden, setHidden] = useState(readHidden);
  const toggle = useCallback((id: string) => {
    setHidden((current) => {
      const next = new Set(current);
      if (!next.delete(id)) next.add(id);
      storeHidden(next);
      return next;
    });
  }, []);
  const reset = useCallback(() => {
    const next = new Set<string>();
    storeHidden(next);
    setHidden(next);
  }, []);
  const context = useMemo(
    () => ({ editing, hidden, toggle }),
    [editing, hidden, toggle]
  );
  return { editing, setEditing, hidden, reset, context };
}

export const ImageDetailsCustomizeProvider: React.FC<
  React.PropsWithChildren<{ value: IDetailsCustomize }>
> = ({ value, children }) => (
  <DetailsCustomizeContext.Provider value={value}>
    {children}
  </DetailsCustomizeContext.Provider>
);

/**
 * One removable part of the Image details panel. Removed parts are gone
 * outside customize mode; while customizing they are shown faded with a
 * button to bring them back. `row` marks a label/value pair in a details
 * list.
 */
export const DetailsItem: React.FC<
  React.PropsWithChildren<{ id: string; label: string; row?: boolean }>
> = ({ id, label, row, children }) => {
  const intl = useIntl();
  const customize = useContext(DetailsCustomizeContext);
  const removed = !!customize?.hidden.has(id);
  if (!customize || (!customize.editing && !removed)) return <>{children}</>;
  if (!customize.editing) return null;
  const action = intl.formatMessage(
    {
      id: removed
        ? "image_details.customize.restore"
        : "image_details.customize.remove",
    },
    { item: label }
  );
  return (
    <div
      className={`image-details-item${row ? " image-details-item-row" : ""}${removed ? " removed" : ""}`}
      data-details-item={id}
    >
      <div className="image-details-item-content" aria-hidden={removed}>
        {children}
      </div>
      <Button
        variant="secondary"
        size="sm"
        className="image-details-item-toggle"
        aria-label={action}
        title={action}
        aria-pressed={removed}
        onClick={() => customize.toggle(id)}
      >
        <Icon icon={removed ? faEye : faXmark} />
      </Button>
    </div>
  );
};

/** Shown above the Details tab while customizing. */
export const ImageDetailsCustomizeBar: React.FC<{
  hiddenCount: number;
  onReset: () => void;
  onDone: () => void;
}> = ({ hiddenCount, onReset, onDone }) => (
  <div className="image-details-customize-bar" role="status">
    <p className="mb-2 small">
      <FormattedMessage id="image_details.customize.help" />
    </p>
    <div className="d-flex align-items-center">
      <Button
        variant="secondary"
        size="sm"
        disabled={hiddenCount === 0}
        onClick={onReset}
      >
        <FormattedMessage
          id="image_details.customize.reset"
          values={{ count: hiddenCount }}
        />
      </Button>
      <Button
        variant="primary"
        size="sm"
        className="ml-auto image-details-customize-done"
        onClick={onDone}
      >
        <FormattedMessage id="image_details.customize.done" />
      </Button>
    </div>
  </div>
);
