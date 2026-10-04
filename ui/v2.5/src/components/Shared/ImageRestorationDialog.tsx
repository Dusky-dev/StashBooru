import React, { useCallback, useEffect, useRef, useState } from "react";
import { Alert, Button, Form, Modal, Spinner } from "react-bootstrap";
import { Link } from "react-router-dom";
import {
  ReferenceComparison,
  ReferenceComparisonMode,
} from "src/hooks/Lightbox/ReferenceComparison";
import { featherMask, maskPNG } from "./restorationMask";
import "./ImageRestorationDialog.scss";

interface Capabilities {
  available: boolean;
  model: string;
  revision: string;
  notice: string;
  cpu: boolean;
  gpu: boolean;
}
interface Session {
  id: string;
  status: string;
  error?: string;
  notice?: string;
  backend: string;
  capabilities: Capabilities;
  derivedImageID?: number;
  receipt: {
    width: number;
    height: number;
    outputSHA256: string;
    normalization: string;
  };
}
const endpoint = "image/restoration";
async function response(value: Response): Promise<Session> {
  if (!value.ok) throw new Error((await value.text()) || value.statusText);
  return value.json();
}
const fileURL = (id: string, role: string, revision = "") =>
  `${endpoint}?sessionID=${encodeURIComponent(id)}&file=${role}&v=${revision}`;

export function ImageRestorationDialog({
  imageID,
  onHide,
}: {
  imageID: string;
  onHide: () => void;
}) {
  const [session, setSession] = useState<Session>();
  const current = useRef<Session>();
  const mounted = useRef(true);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [backend, setBackend] = useState("auto");
  const [hardware, setHardware] = useState("auto");
  const [prompt, setPrompt] = useState(
    "Restore the damaged area naturally, consistent with its surroundings"
  );
  const [negativePrompt, setNegativePrompt] = useState("");
  const [seed, setSeed] = useState(42);
  const [steps, setSteps] = useState(20);
  const [guidance, setGuidance] = useState(7.5);
  const [strength, setStrength] = useState(1);
  const [reference, setReference] = useState("");
  const [referenceName, setReferenceName] = useState("");
  const [stack, setStack] = useState(false);
  const [tool, setTool] = useState("brush");
  const [brush, setBrush] = useState(24);
  const [feather, setFeather] = useState(0);
  const [zoom, setZoom] = useState(1);
  const [loaded, setLoaded] = useState(false);
  const [dirty, setDirty] = useState(true);
  const [painted, setPainted] = useState(false);
  const [undoCount, setUndoCount] = useState(0);
  const [comparison, setComparison] =
    useState<Exclude<ReferenceComparisonMode, "selected">>("both");
  const [comparisonZoom, setComparisonZoom] = useState(1);
  const canvas = useRef<HTMLCanvasElement>(null);
  const overlay = useRef<HTMLCanvasElement>(null);
  const image = useRef<HTMLImageElement>(null);
  const history = useRef<ImageData[]>([]);
  const pointer = useRef<{
    x: number;
    y: number;
    startX: number;
    startY: number;
    before: ImageData;
  }>();

  const post = useCallback(
    async (action: string, values: Record<string, unknown> = {}) => {
      return response(
        await fetch(endpoint, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            action,
            sessionID: current.current?.id,
            ...values,
          }),
        })
      );
    },
    []
  );
  const updateSession = useCallback(
    (next: Session) => {
      current.current = next;
      if (mounted.current) setSession(next);
      else if (next.status !== "saved" && next.status !== "saving") {
        // A prepare/generate response can arrive after dismissal and its cleanup.
        void post(
          next.status === "queued" || next.status === "running"
            ? "cancel"
            : "discard"
        ).catch(() => {});
      }
    },
    [post]
  );
  const running = session?.status === "queued" || session?.status === "running";
  const editable =
    loaded &&
    !busy &&
    !running &&
    (session?.status === "ready" || session?.status === "preview");

  useEffect(() => {
    mounted.current = true;
    return () => {
      mounted.current = false;
      const active = current.current;
      if (active && active.status !== "saved") {
        void post(
          active.status === "queued" || active.status === "running"
            ? "cancel"
            : "discard"
        ).catch(() => {});
      }
    };
  }, [post]);
  useEffect(() => {
    const sessionID = current.current?.id;
    if (!sessionID || !running) return;
    let stopped = false;
    let timer: ReturnType<typeof setTimeout>;
    const refresh = async () => {
      try {
        const next = await response(
          await fetch(`${endpoint}?sessionID=${sessionID}`, {
            cache: "no-store",
          })
        );
        if (!stopped) {
          updateSession(next);
          if (next.status === "preview") setDirty(false);
          if (next.status === "queued" || next.status === "running")
            timer = setTimeout(refresh, 800);
        }
      } catch (e) {
        if (!stopped) {
          setError(String(e));
          timer = setTimeout(refresh, 2000);
        }
      }
    };
    timer = setTimeout(refresh, 300);
    return () => {
      stopped = true;
      clearTimeout(timer);
    };
  }, [running, updateSession]);

  const redraw = useCallback(() => {
    if (!canvas.current || !overlay.current) return;
    const { width, height } = canvas.current;
    if (!width || !height) return;
    const raw = canvas.current
      .getContext("2d")!
      .getImageData(0, 0, width, height);
    const input = new Uint8Array(width * height);
    for (let i = 0; i < input.length; i++) input[i] = raw.data[i * 4 + 3];
    const effective = featherMask(input, width, height, feather);
    const context = overlay.current.getContext("2d")!;
    const paint = context.createImageData(width, height);
    let hasPaint = false;
    for (let i = 0; i < input.length; i++) {
      paint.data[i * 4] = 255;
      paint.data[i * 4 + 1] = 60;
      paint.data[i * 4 + 2] = 90;
      paint.data[i * 4 + 3] = Math.round(effective[i] * 0.6);
      hasPaint ||= effective[i] > 0;
    }
    context.putImageData(paint, 0, 0);
    setPainted(hasPaint);
  }, [feather]);
  useEffect(() => {
    if (loaded) redraw();
  }, [redraw, loaded]);

  function sourceLoaded() {
    if (!image.current || !canvas.current || !overlay.current) return;
    const { naturalWidth: width, naturalHeight: height } = image.current;
    if (width * height > 16 * 1024 * 1024 || width > 8192 || height > 8192) {
      setError("Source exceeds mask editor limits");
      return;
    }
    canvas.current.width = overlay.current.width = width;
    canvas.current.height = overlay.current.height = height;
    history.current = [];
    setUndoCount(0);
    setLoaded(true);
    setPainted(false);
    setDirty(true);
  }
  function coordinates(event: React.PointerEvent<HTMLCanvasElement>) {
    const node = overlay.current!;
    const bounds = node.getBoundingClientRect();
    return {
      x: Math.max(
        0,
        Math.min(
          node.width,
          ((event.clientX - bounds.left) * node.width) / bounds.width
        )
      ),
      y: Math.max(
        0,
        Math.min(
          node.height,
          ((event.clientY - bounds.top) * node.height) / bounds.height
        )
      ),
    };
  }
  function draw(event: React.PointerEvent<HTMLCanvasElement>) {
    const active = pointer.current;
    if (!active || !canvas.current) return;
    const context = canvas.current.getContext("2d")!;
    const point = coordinates(event);
    context.globalCompositeOperation =
      tool === "erase" ? "destination-out" : "source-over";
    context.strokeStyle = context.fillStyle = "white";
    if (tool === "box") {
      context.putImageData(active.before, 0, 0);
      context.fillRect(
        Math.min(active.startX, point.x),
        Math.min(active.startY, point.y),
        Math.abs(point.x - active.startX),
        Math.abs(point.y - active.startY)
      );
    } else {
      context.lineWidth = brush;
      context.lineCap = context.lineJoin = "round";
      context.beginPath();
      context.moveTo(active.x, active.y);
      context.lineTo(point.x, point.y);
      context.stroke();
      context.beginPath();
      context.arc(point.x, point.y, brush / 2, 0, Math.PI * 2);
      context.fill();
    }
    active.x = point.x;
    active.y = point.y;
    setDirty(true);
    redraw();
  }
  function down(event: React.PointerEvent<HTMLCanvasElement>) {
    if (!editable || !canvas.current || event.button !== 0) return;
    const point = coordinates(event);
    const before = canvas.current
      .getContext("2d")!
      .getImageData(0, 0, canvas.current.width, canvas.current.height);
    // Bound undo memory to 128 MiB, including the current stroke snapshot.
    const maximum = Math.max(
      1,
      Math.min(20, Math.floor((128 * 1024 ** 2) / before.data.length) - 1)
    );
    history.current.push(before);
    history.current = history.current.slice(-maximum);
    setUndoCount(history.current.length);
    pointer.current = { ...point, startX: point.x, startY: point.y, before };
    event.currentTarget.setPointerCapture(event.pointerId);
    draw(event);
  }
  function undo() {
    const before = history.current.pop();
    if (!before || !canvas.current) return;
    canvas.current.getContext("2d")!.putImageData(before, 0, 0);
    setUndoCount(history.current.length);
    setDirty(true);
    redraw();
  }
  async function act(action: string) {
    setBusy(true);
    setError("");
    try {
      if (action === "prepare") {
        const old = current.current;
        if (old && old.status !== "saved") await post("discard");
        setLoaded(false);
        setDirty(true);
        history.current = [];
        const next = await post("prepare", {
          imageID: Number(imageID),
          backend,
        });
        updateSession(next);
      } else if (action === "generate") {
        if (!canvas.current || !painted) throw new Error("Paint a mask first");
        updateSession(
          await post("generate", {
            mask: maskPNG(canvas.current),
            reference,
            feather,
            options: {
              hardware,
              prompt,
              negativePrompt,
              seed,
              steps,
              guidance,
              strength,
            },
          })
        );
      } else
        updateSession(await post(action, action === "save" ? { stack } : {}));
    } catch (e) {
      if (mounted.current) setError(String(e));
    } finally {
      if (mounted.current) setBusy(false);
    }
  }
  async function referenceFile(file?: File) {
    if (!file) {
      setReference("");
      setReferenceName("");
      setDirty(true);
      return;
    }
    if (file.size > 8 * 1024 ** 2) {
      setError("Reference must be at most 8 MiB");
      return;
    }
    const reader = new FileReader();
    reader.onload = () => {
      if (mounted.current) {
        setReference(String(reader.result).split(",")[1]);
        setReferenceName(file.name);
        setStrength(0.75);
        setDirty(true);
      }
    };
    reader.readAsDataURL(file);
  }
  const preview = session?.status === "preview";
  const source = session ? fileURL(session.id, "source") : "";
  const output = session
    ? fileURL(session.id, "output", session.receipt.outputSHA256)
    : "";

  return (
    <Modal
      show
      onHide={onHide}
      size="xl"
      dialogClassName="image-restoration-dialog"
      onKeyDown={(event: React.KeyboardEvent) => event.stopPropagation()}
    >
      <Modal.Header closeButton>
        <Modal.Title>Mask restoration</Modal.Title>
      </Modal.Header>
      <Modal.Body>
        <Alert variant="info">
          Generate plausible new pixels for damaged, blurred or redacted areas
          in non-explicit still images. This does not recover hidden original
          detail. The original is preserved.
        </Alert>
        {(error || session?.error) && (
          <Alert variant="danger">{error || session?.error}</Alert>
        )}
        {session?.notice && <Alert variant="warning">{session.notice}</Alert>}
        <div className="restoration-controls">
          <Form.Label>
            Processing backend
            <Form.Control
              as="select"
              value={backend}
              disabled={busy || running}
              onChange={(e) => setBackend(e.target.value)}
              className="input-control"
            >
              <option value="auto">Auto (remote preferred)</option>
              <option value="remote">Remote only</option>
              <option value="local">Local only</option>
            </Form.Control>
          </Form.Label>
          <Button
            disabled={busy || running}
            onClick={() => void act("prepare")}
          >
            Prepare source
          </Button>
          {running && (
            <>
              <Spinner animation="border" size="sm" />
              <span>Processing…</span>
              <Button onClick={() => void act("cancel")} disabled={busy}>
                Cancel processing
              </Button>
            </>
          )}
        </div>
        {session?.capabilities && (
          <p>
            Selected worker: {session.backend}. Model:{" "}
            {session.capabilities.model} ·{" "}
            {session.capabilities.revision || "not installed"}
          </p>
        )}
        {session?.capabilities && !session.capabilities.available && (
          <Alert variant="warning">{session.capabilities.notice}</Alert>
        )}
        {(loaded || session?.status === "ready" || preview) &&
          session?.status !== "saved" && (
            <>
              <div className="restoration-controls">
                {[
                  ["brush", "Brush"],
                  ["erase", "Erase"],
                  ["box", "Box"],
                ].map(([value, label]) => (
                  <Button
                    key={value}
                    variant={tool === value ? "primary" : "secondary"}
                    disabled={!editable}
                    onClick={() => setTool(value)}
                  >
                    {label}
                  </Button>
                ))}
                <Button disabled={!editable || !undoCount} onClick={undo}>
                  Undo
                </Button>
                <Button
                  disabled={!editable}
                  onClick={() => {
                    canvas
                      .current!.getContext("2d")!
                      .clearRect(
                        0,
                        0,
                        canvas.current!.width,
                        canvas.current!.height
                      );
                    history.current = [];
                    setUndoCount(0);
                    setDirty(true);
                    redraw();
                  }}
                >
                  Clear mask
                </Button>
                <Form.Label>
                  Brush size (source pixels)
                  <Form.Control
                    type="number"
                    className="text-input"
                    min={1}
                    max={256}
                    value={brush}
                    disabled={!editable}
                    onChange={(e) =>
                      setBrush(
                        Math.max(1, Math.min(256, Number(e.target.value)))
                      )
                    }
                  />
                </Form.Label>
                <Form.Label>
                  Feather (source pixels)
                  <Form.Control
                    type="number"
                    className="text-input"
                    min={0}
                    max={16}
                    value={feather}
                    disabled={!editable}
                    onChange={(e) => {
                      setFeather(
                        Math.max(
                          0,
                          Math.min(16, Math.round(Number(e.target.value)))
                        )
                      );
                      setDirty(true);
                    }}
                  />
                </Form.Label>
                <Form.Label>
                  Zoom
                  <Form.Control
                    as="select"
                    className="input-control"
                    value={zoom}
                    onChange={(e) => setZoom(Number(e.target.value))}
                  >
                    <option value={1}>Fit</option>
                    <option value={2}>2× fit</option>
                    <option value={4}>4× fit</option>
                  </Form.Control>
                </Form.Label>
              </div>
              <p>
                Pink shows the exact mask and feather support. Only those RGB
                pixels can change; alpha remains unchanged. Drag to paint.
                Scroll the zoomed canvas to pan.
              </p>
              <div className="restoration-canvas-scroll">
                <div
                  className="restoration-canvas"
                  style={{ width: `${zoom * 100}%` }}
                >
                  <img
                    ref={image}
                    src={source}
                    alt="Original for mask painting"
                    onLoad={sourceLoaded}
                  />
                  <canvas ref={canvas} hidden />
                  <canvas
                    ref={overlay}
                    aria-label="Paint restoration mask"
                    onPointerDown={down}
                    onPointerMove={draw}
                    onPointerUp={() => {
                      pointer.current = undefined;
                    }}
                    onPointerCancel={() => {
                      pointer.current = undefined;
                    }}
                  />
                </div>
              </div>
              <fieldset disabled={!editable} onChange={() => setDirty(true)}>
                <div className="restoration-controls mt-3">
                  <Form.Label>
                    Hardware
                    <Form.Control
                      as="select"
                      className="input-control"
                      value={hardware}
                      onChange={(e) => setHardware(e.target.value)}
                    >
                      <option value="auto">Prefer GPU</option>
                      <option value="gpu">GPU only</option>
                      <option value="cpu">CPU only</option>
                    </Form.Control>
                  </Form.Label>
                  <Form.Label>
                    Seed
                    <Form.Control
                      className="text-input"
                      type="number"
                      min={0}
                      max={2147483647}
                      value={seed}
                      onChange={(e) => setSeed(Number(e.target.value))}
                    />
                  </Form.Label>
                  <Form.Label>
                    Steps
                    <Form.Control
                      className="text-input"
                      type="number"
                      min={5}
                      max={50}
                      value={steps}
                      onChange={(e) => setSteps(Number(e.target.value))}
                    />
                  </Form.Label>
                  <Form.Label>
                    Guidance
                    <Form.Control
                      className="text-input"
                      type="number"
                      min={1}
                      max={15}
                      step={0.5}
                      value={guidance}
                      onChange={(e) => setGuidance(Number(e.target.value))}
                    />
                  </Form.Label>
                  <Form.Label>
                    Strength
                    <Form.Control
                      className="text-input"
                      type="number"
                      min={0.05}
                      max={1}
                      step={0.05}
                      value={strength}
                      onChange={(e) => setStrength(Number(e.target.value))}
                    />
                  </Form.Label>
                </div>
                <Form.Label className="d-block">
                  Prompt
                  <Form.Control
                    className="text-input"
                    as="textarea"
                    value={prompt}
                    maxLength={2000}
                    onChange={(e) => setPrompt(e.target.value)}
                  />
                </Form.Label>
                <Form.Label className="d-block">
                  Negative prompt
                  <Form.Control
                    className="text-input"
                    value={negativePrompt}
                    maxLength={2000}
                    onChange={(e) => setNegativePrompt(e.target.value)}
                  />
                </Form.Label>
                <Form.Label className="d-block">
                  Optional starting-pixel reference (PNG, JPEG or WebP)
                  <Form.Control
                    type="file"
                    accept="image/png,image/jpeg,image/webp"
                    onChange={(e: React.ChangeEvent<HTMLInputElement>) =>
                      void referenceFile(e.target.files?.[0])
                    }
                  />
                </Form.Label>
                {referenceName && (
                  <p>
                    {referenceName}{" "}
                    <Button size="sm" onClick={() => void referenceFile()}>
                      Remove reference
                    </Button>
                  </p>
                )}
                <p>
                  The reference is fitted to the mask's context crop as initial
                  pixels, not an identity/style adapter. Use strength below 1.
                  Inference uses a 512×512 crop; generated pixels are resized
                  back inside the mask. Orientation is baked into the new PNG;
                  the source ICC profile is retained. Other embedded metadata is
                  not copied.
                </p>
              </fieldset>
              <Button
                disabled={
                  !editable || !painted || !session?.capabilities.available
                }
                onClick={() => void act("generate")}
              >
                Generate preview
              </Button>
            </>
          )}
        {preview && (
          <div className="mt-3">
            {dirty && (
              <Alert variant="warning">
                Mask or settings changed. Generate another preview before
                saving.
              </Alert>
            )}
            <div className="restoration-controls">
              <Form.Label>
                Comparison
                <Form.Control
                  as="select"
                  className="input-control"
                  value={comparison}
                  onChange={(e) =>
                    setComparison(
                      e.target.value as Exclude<
                        ReferenceComparisonMode,
                        "selected"
                      >
                    )
                  }
                >
                  <option value="both">Side by side</option>
                  <option value="slider">Wipe</option>
                  <option value="blink">Blink</option>
                  <option value="difference">Difference map and boxes</option>
                </Form.Control>
              </Form.Label>
              <Button onClick={() => setComparisonZoom(1)}>
                Reset comparison zoom
              </Button>
            </div>
            <div className="restoration-review">
              <ReferenceComparison
                referenceImage={{
                  id: `${session!.id}-source`,
                  title: "Original",
                  paths: { image: source },
                  visual_files: [
                    {
                      path: "source.png",
                      width: session!.receipt.width,
                      height: session!.receipt.height,
                    },
                  ],
                }}
                selectedImage={{
                  id: `${session!.id}-${session!.receipt.outputSHA256}`,
                  title: "Generated restoration",
                  paths: { image: output },
                  visual_files: [
                    {
                      path: "restored.png",
                      width: session!.receipt.width,
                      height: session!.receipt.height,
                    },
                  ],
                }}
                mode={comparison}
                direction="left"
                animateSelected={false}
                zoom={comparisonZoom}
                resetPosition
                setZoom={setComparisonZoom}
              />
            </div>
            <Form.Check
              id="restoration-stack"
              className="mt-3"
              type="checkbox"
              label="Add derivative to the source's visual stack"
              checked={stack}
              onChange={(e) => setStack(e.target.checked)}
              disabled={busy}
            />
            <Button
              className="mt-2"
              disabled={busy || dirty}
              onClick={() => void act("save")}
            >
              Save generated derivative
            </Button>
          </div>
        )}
        {session?.status === "saved" && (
          <Alert variant="success">
            Saved a new generated derivative.{" "}
            <Link to={`/images/${session.derivedImageID}`} onClick={onHide}>
              Open Image
            </Link>
          </Alert>
        )}
      </Modal.Body>
      <Modal.Footer>
        <Button variant="secondary" onClick={onHide}>
          Close
        </Button>
      </Modal.Footer>
    </Modal>
  );
}
