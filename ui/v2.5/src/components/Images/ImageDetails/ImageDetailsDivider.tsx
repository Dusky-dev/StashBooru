import React, { useCallback, useEffect, useRef, useState } from "react";
import { Button } from "react-bootstrap";
import { useIntl } from "react-intl";
import {
  faChevronDown,
  faChevronLeft,
  faChevronRight,
  faChevronUp,
} from "@fortawesome/free-solid-svg-icons";
import { Icon } from "src/components/Shared/Icon";

const widthKey = "stashbooru.imageDetails.width";
const collapsedKey = "stashbooru.imageDetails.collapsed";
export const defaultDetailsWidth = 450;
const minWidth = 320;

function maxWidth() {
  return Math.max(minWidth, Math.round(window.innerWidth * 0.6));
}

function clampWidth(width: number) {
  return Math.min(maxWidth(), Math.max(minWidth, Math.round(width)));
}

function readNumber(key: string) {
  try {
    const value = Number(localStorage.getItem(key));
    return Number.isFinite(value) && value > 0 ? value : undefined;
  } catch {
    return undefined;
  }
}

function store(key: string, value: string) {
  try {
    localStorage.setItem(key, value);
  } catch {
    // Private windows may refuse storage; the layout still works.
  }
}

/** Width and collapsed state of the Image details panel, kept between visits. */
export function useImageDetailsLayout() {
  const [width, setWidthState] = useState(
    () => readNumber(widthKey) ?? defaultDetailsWidth
  );
  const [collapsed, setCollapsedState] = useState(() => {
    try {
      return localStorage.getItem(collapsedKey) === "true";
    } catch {
      return false;
    }
  });
  const setWidth = useCallback((next: number) => {
    const clamped = clampWidth(next);
    setWidthState(clamped);
    store(widthKey, String(clamped));
  }, []);
  const setCollapsed = useCallback((next: boolean) => {
    setCollapsedState(next);
    store(collapsedKey, String(next));
  }, []);
  useEffect(() => {
    // A narrower window must not leave the panel wider than allowed.
    const fit = () => setWidthState((current) => clampWidth(current));
    window.addEventListener("resize", fit);
    return () => window.removeEventListener("resize", fit);
  }, []);
  return { width, setWidth, collapsed, setCollapsed };
}

/**
 * Between the details panel and the Image. On wide screens one button at the
 * top hides the panel and, in the same place, brings it back; while the panel
 * is customized a handle below it resizes the panel (drag, or the arrow keys).
 * On small screens, where the panel sits below the Image, a full-width button
 * hides and shows it.
 */
export const ImageDetailsDivider: React.FC<{
  width: number;
  setWidth: (width: number) => void;
  collapsed: boolean;
  setCollapsed: (collapsed: boolean) => void;
  panelRef: React.RefObject<HTMLElement>;
  resizable: boolean;
}> = ({ width, setWidth, collapsed, setCollapsed, panelRef, resizable }) => {
  const intl = useIntl();
  const drag = useRef<{ start: number; width: number }>();
  const [dragging, setDragging] = useState(false);
  const label = intl.formatMessage({
    id: collapsed ? "image_details.show" : "image_details.hide",
  });
  const stop = () => {
    drag.current = undefined;
    setDragging(false);
  };
  return (
    <div className={`image-details-divider${collapsed ? " collapsed" : ""}`}>
      {/* One element for both states, so focus stays on it. */}
      <Button
        variant="secondary"
        size="sm"
        className={`image-details-collapse d-none d-xl-inline-flex ${collapsed ? "image-details-show" : "image-details-hide"}`}
        onClick={() => setCollapsed(!collapsed)}
        aria-expanded={!collapsed}
        aria-label={label}
        title={`${label} (,)`}
      >
        <Icon icon={collapsed ? faChevronRight : faChevronLeft} />
        {collapsed && (
          <span>{intl.formatMessage({ id: "image_details.show_short" })}</span>
        )}
      </Button>
      {!collapsed && resizable && (
        <div
          className={`image-details-resize d-none d-xl-block${dragging ? " dragging" : ""}`}
          role="separator"
          aria-orientation="vertical"
          aria-label={intl.formatMessage({ id: "image_details.resize" })}
          aria-valuemin={minWidth}
          aria-valuemax={maxWidth()}
          aria-valuenow={width}
          tabIndex={0}
          onPointerDown={(event) => {
            event.preventDefault();
            event.currentTarget.setPointerCapture(event.pointerId);
            drag.current = {
              start: event.clientX,
              width: panelRef.current?.getBoundingClientRect().width ?? width,
            };
            setDragging(true);
          }}
          onPointerMove={(event) => {
            if (!drag.current) return;
            setWidth(drag.current.width + event.clientX - drag.current.start);
          }}
          onPointerUp={stop}
          onPointerCancel={stop}
          onLostPointerCapture={stop}
          onDoubleClick={() => setWidth(defaultDetailsWidth)}
          onKeyDown={(event) => {
            const step = event.shiftKey ? 80 : 20;
            if (event.key === "ArrowLeft") setWidth(width - step);
            else if (event.key === "ArrowRight") setWidth(width + step);
            else if (event.key === "Home") setWidth(minWidth);
            else if (event.key === "End") setWidth(maxWidth());
            else return;
            event.preventDefault();
          }}
        />
      )}
      <Button
        variant="minimal"
        className="image-details-toggle d-xl-none"
        onClick={() => setCollapsed(!collapsed)}
        aria-expanded={!collapsed}
        aria-label={label}
        title={`${label} (,)`}
      >
        <Icon icon={collapsed ? faChevronDown : faChevronUp} fixedWidth />{" "}
        {label}
      </Button>
    </div>
  );
};
