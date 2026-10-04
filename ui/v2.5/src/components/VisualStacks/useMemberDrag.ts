import { PointerEvent, useCallback, useEffect, useRef, useState } from "react";

interface Gesture {
  source: string;
  target?: string;
  pointerID: number;
  handle: HTMLElement;
  startX: number;
  startY: number;
  x: number;
  y: number;
  started: boolean;
}

// Pointer capture works on mouse, pen and touch; only the handle owns a gesture.
// Rows do not move until a valid drop, so Escape/outside/cancel leave the draft alone.
export function useMemberDrag(
  disabled: boolean,
  onDrop: (source: string, target: string) => void
) {
  const editorRef = useRef<HTMLDivElement>(null);
  const gesture = useRef<Gesture>();
  const frame = useRef<number>();
  const latest = useRef({ disabled, onDrop });
  latest.current = { disabled, onDrop };
  const [drag, setDrag] = useState<{ source: string; target?: string }>();

  const stop = useCallback(() => {
    const current = gesture.current;
    gesture.current = undefined;
    if (frame.current !== undefined) cancelAnimationFrame(frame.current);
    frame.current = undefined;
    if (current?.handle.hasPointerCapture(current.pointerID))
      current.handle.releasePointerCapture(current.pointerID);
  }, []);
  const cancel = useCallback(() => {
    stop();
    setDrag(undefined);
  }, [stop]);

  function targetAt(current: Gesture) {
    const editor = editorRef.current;
    if (!editor) return undefined;
    const box = editor.getBoundingClientRect();
    if (
      current.x < box.left ||
      current.x > box.right ||
      current.y < box.top ||
      current.y > box.bottom
    )
      return undefined;
    const row = document
      .elementFromPoint(current.x, current.y)
      ?.closest<HTMLElement>(".visual-stack-editor-member");
    return row && editor.contains(row) ? row.dataset.memberId : undefined;
  }

  function tick() {
    const current = gesture.current;
    const editor = editorRef.current;
    if (!current?.started || !editor) return;
    const box = editor.getBoundingClientRect();
    if (
      current.x >= box.left &&
      current.x <= box.right &&
      current.y >= box.top &&
      current.y <= box.bottom
    ) {
      if (current.y < box.top + 32) editor.scrollTop -= 8;
      else if (current.y > box.bottom - 32) editor.scrollTop += 8;
    }
    const target = targetAt(current);
    if (target !== current.target) {
      current.target = target;
      setDrag({ source: current.source, target });
    }
    frame.current = requestAnimationFrame(tick);
  }

  useEffect(() => {
    if (disabled) cancel();
  }, [disabled, cancel]);
  useEffect(() => stop, [stop]);

  function onPointerDown(source: string, e: PointerEvent<HTMLElement>) {
    if (
      latest.current.disabled ||
      e.button !== 0 ||
      !e.isPrimary ||
      gesture.current
    )
      return;
    e.preventDefault();
    e.currentTarget.focus();
    e.currentTarget.setPointerCapture(e.pointerId);
    gesture.current = {
      source,
      pointerID: e.pointerId,
      handle: e.currentTarget,
      startX: e.clientX,
      startY: e.clientY,
      x: e.clientX,
      y: e.clientY,
      started: false,
    };
  }
  function onPointerMove(e: PointerEvent<HTMLElement>) {
    const current = gesture.current;
    if (!current || current.pointerID !== e.pointerId) return;
    current.x = e.clientX;
    current.y = e.clientY;
    if (
      !current.started &&
      Math.hypot(current.x - current.startX, current.y - current.startY) >= 6
    ) {
      current.started = true;
      setDrag({ source: current.source });
      tick();
    }
  }
  function onPointerUp(e: PointerEvent<HTMLElement>) {
    const current = gesture.current;
    if (!current || current.pointerID !== e.pointerId) return;
    current.x = e.clientX;
    current.y = e.clientY;
    const target = targetAt(current);
    stop();
    setDrag(undefined);
    if (current.started && target && !latest.current.disabled)
      latest.current.onDrop(current.source, target);
  }
  return { editorRef, drag, cancel, onPointerDown, onPointerMove, onPointerUp };
}
