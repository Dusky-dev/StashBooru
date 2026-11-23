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
 * Between the details panel and the Image on wide screens: drag (or use the
 * arrow keys) to resize the panel, or use the button to hide it. On small
 * screens, where the panel sits below the Image, only hiding is offered.
 */
export const ImageDetailsDivider: React.FC<{
  width: number;
  setWidth: (width: number) => void;
  collapsed: boolean;
  setCollapsed: (collapsed: boolean) => void;
  panelRef: React.RefObject<HTMLElement>;
}> = ({ width, setWidth, collapsed, setCollapsed, panelRef }) => {
  const intl = useIntl();
  const drag = useRef<{ start: number; width: number }>();
  const label = intl.formatMessage({
    id: collapsed ? "image_details.show" : "image_details.hide",
  });
  const toggle = (
    <Button
      variant="minimal"
      className="image-details-toggle"
      onClick={() => setCollapsed(!collapsed)}
      aria-expanded={!collapsed}
      aria-label={label}
      title={`${label} (,)`}
    >
      <span className="d-none d-xl-inline">
        <Icon icon={collapsed ? faChevronRight : faChevronLeft} fixedWidth />
      </span>
      <span className="d-xl-none">
        <Icon icon={collapsed ? faChevronDown : faChevronUp} fixedWidth />{" "}
        {label}
      </span>
    </Button>
  );
  return (
    <div className="image-details-divider">
      {!collapsed && (
        <div
          className="image-details-resize d-none d-xl-block"
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
          }}
          onPointerMove={(event) => {
            if (!drag.current) return;
            setWidth(drag.current.width + event.clientX - drag.current.start);
          }}
          onPointerUp={() => {
            drag.current = undefined;
          }}
          onPointerCancel={() => {
            drag.current = undefined;
          }}
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
      {toggle}
    </div>
  );
};
