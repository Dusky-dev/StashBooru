import React, { useEffect, useState } from "react";
import { Alert, Button, Card, Form, Spinner, Table } from "react-bootstrap";
import { useToast } from "src/hooks/Toast";
import {
  conversionEndpoint,
  conversionResponse,
  ConversionConfig,
  ConversionFormat,
  ConversionSettings,
} from "../Shared/mediaConversion";

interface Rule {
  input: string;
  output: string;
  quality: number;
  effort: number;
  decodingSpeed: number;
}

export const MediaConversionSettings: React.FC = () => {
  const Toast = useToast();
  const [settings, setSettings] = useState<ConversionSettings>();
  const [workerFormats, setWorkerFormats] = useState<ConversionFormat[]>([]);
  const [loadingWorkerFormats, setLoadingWorkerFormats] = useState(false);
  const [workerCapabilityError, setWorkerCapabilityError] = useState("");
  const [rules, setRules] = useState<Rule[]>([]);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const [backend, setBackend] = useState("auto");

  useEffect(() => {
    const controller = new AbortController();
    void fetch(`${conversionEndpoint}?config=1`, { signal: controller.signal })
      .then(conversionResponse<ConversionSettings>)
      .then((value) => {
        if (controller.signal.aborted) return;
        setSettings(value);
        setBackend(value.config.backend);
        setRules(
          value.inputFormats
            .filter((f) => value.config.formatDefaults[f.id])
            .map((f) => {
              const fallback =
                value.config.encodingDefaults[
                  f.family === "video" ? "video" : "image"
                ];
              const saved = value.config.encodingDefaults[f.id];
              const output = value.config.formatDefaults[f.id];
              const legacyJXLSpeed =
                output === "jxl" || output === "ajxl"
                  ? (saved?.fasterDecoding ?? fallback?.fasterDecoding)
                  : undefined;
              return {
                input: f.id,
                output,
                quality:
                  saved?.quality ??
                  fallback?.quality ??
                  (f.family === "video" ? 80 : 90),
                effort: saved?.effort ?? fallback?.effort ?? 7,
                decodingSpeed:
                  saved?.decodingSpeed ??
                  fallback?.decodingSpeed ??
                  legacyJXLSpeed ??
                  (f.family === "animation" &&
                  (output === "jxl" || output === "ajxl")
                    ? 2
                    : 0),
              };
            })
        );
      })
      .catch((e: Error) => {
        if (!controller.signal.aborted) setError(e.message);
      });
    return () => controller.abort();
  }, []);

  useEffect(() => {
    if (!settings) return;
    const controller = new AbortController();
    setWorkerFormats([]);
    setWorkerCapabilityError("");
    setLoadingWorkerFormats(true);
    void fetch(
      `${conversionEndpoint}?capabilities=1&backend=${encodeURIComponent(backend)}`,
      { signal: controller.signal }
    )
      .then(
        conversionResponse<{
          formats: ConversionFormat[];
        }>
      )
      .then((value) => {
        if (!controller.signal.aborted) setWorkerFormats(value.formats);
      })
      .catch((e: Error) => {
        if (!controller.signal.aborted) setWorkerCapabilityError(e.message);
      })
      .finally(() => {
        if (!controller.signal.aborted) setLoadingWorkerFormats(false);
      });
    return () => controller.abort();
  }, [backend, settings]);

  const outputsFor = (input: string) => {
    const family = settings?.inputFormats.find((f) => f.id === input)?.family;
    return (settings?.outputFormats ?? []).filter(
      (f) =>
        (family !== "video" || f.family === "video") &&
        (family !== "animation" || f.family !== "image")
    );
  };

  const catalogDecodingSpeedLevels = (output: string) =>
    settings?.outputFormats.find((f) => f.id === output)?.decodingSpeedLevels ??
    0;

  const decodingSpeedLevels = (output: string) => {
    const format = settings?.outputFormats.find((f) => f.id === output);
    const worker = workerFormats.find((f) => f.id === output);
    if (!format || !worker) return 0;
    const generic = worker.controls.includes("decodingSpeed");
    const legacyJXL =
      (output === "jxl" || output === "ajxl") &&
      worker.controls.includes("fasterDecoding");
    if (!generic && !legacyJXL) return 0;
    return (
      worker.decodingSpeedLevels ??
      (legacyJXL ? 4 : (format.decodingSpeedLevels ?? 0))
    );
  };

  const decodingSpeedLimit = (output: string) => {
    const workerLimit = decodingSpeedLevels(output);
    if (workerLimit > 0) return workerLimit;
    // Keep a saved preference editable while worker capabilities are loading
    // or temporarily unavailable. The converter will omit unsupported values.
    return loadingWorkerFormats || workerCapabilityError
      ? catalogDecodingSpeedLevels(output)
      : 0;
  };

  const clampDecodingSpeed = (speed: number, output: string) => {
    const limit = decodingSpeedLimit(output);
    return limit > 0 ? Math.min(speed, limit) : 0;
  };

  async function save() {
    setSaving(true);
    setError("");
    try {
      const config = await fetch(conversionEndpoint, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          action: "save-defaults",
          backend,
          encodingDefaults: Object.fromEntries(
            rules.map((r) => [
              r.input,
              {
                quality: r.quality,
                effort: r.effort,
                decodingSpeed: r.decodingSpeed,
              },
            ])
          ),
          formatDefaults: Object.fromEntries(
            rules.map((r) => [r.input, r.output])
          ),
        }),
      }).then(conversionResponse<ConversionConfig>);
      setSettings((old) => old && { ...old, config });
      Toast.success("Saved media conversion defaults.");
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setSaving(false);
    }
  }

  const unused = settings?.inputFormats.filter(
    (f) => !rules.some((r) => r.input === f.id)
  );

  return (
    <div className="setting-section" id="media-converter">
      <h1>Media converter</h1>
      <div className="sub-heading">
        Choose the output, quality, effort and decode speed for each input
        format. Single and batch conversions use these saved defaults.
      </div>
      <Card className="p-3">
        <Form.Group controlId="converter-default-backend">
          <Form.Label>Run on</Form.Label>
          <Form.Control
            as="select"
            className="input-control"
            value={backend}
            disabled={saving || !settings}
            onChange={(e) => setBackend(e.target.value)}
          >
            <option value="auto">Automatic — prefer remote worker</option>
            <option value="local">This StashBooru server</option>
            <option value="remote">Remote inference worker</option>
          </Form.Control>
        </Form.Group>
        <p>
          Automatic mode prefers the shared remote inference worker configured
          above when it is reachable. Otherwise, conversions run on this server.
          The processor defaults to <strong>Prefer GPU, otherwise CPU</strong>.
        </p>
        <p className="text-muted">
          Animated PNG and WebP are detected separately from still images.
          Formats without a specific rule use Other images or Other videos.
          Actual codec availability depends on the selected worker. Existing
          library animation inspection is available from Tasks.
        </p>
        {error && <Alert variant="danger">{error}</Alert>}
        {workerCapabilityError && (
          <Alert variant="warning">
            Could not read codec support from the selected worker. Decode-speed
            controls are hidden until the worker is reachable.
          </Alert>
        )}
        {!settings && !error && <Spinner animation="border" role="status" />}
        {settings && (
          <>
            <Table responsive size="sm">
              <thead>
                <tr>
                  <th>Input format</th>
                  <th>Default output format</th>
                  <th>Quality (0–100)</th>
                  <th>Effort (1–9)</th>
                  <th>Decode speed</th>
                  <th>
                    <span className="sr-only">Remove rule</span>
                  </th>
                </tr>
              </thead>
              <tbody>
                {rules.map((rule, index) => (
                  <tr key={rule.input}>
                    <td>
                      <Form.Control
                        as="select"
                        className="input-control"
                        aria-label={`Input format for rule ${index + 1}`}
                        value={rule.input}
                        disabled={
                          saving ||
                          rule.input === "image" ||
                          rule.input === "video"
                        }
                        onChange={(e) => {
                          const input = e.target.value;
                          const allowed = outputsFor(input);
                          const output = allowed.some(
                            (f) => f.id === rule.output
                          )
                            ? rule.output
                            : allowed[0].id;
                          setRules(
                            rules.map((r, i) =>
                              i === index
                                ? {
                                    ...r,
                                    input,
                                    output,
                                    decodingSpeed: clampDecodingSpeed(
                                      r.decodingSpeed,
                                      output
                                    ),
                                  }
                                : r
                            )
                          );
                        }}
                      >
                        {settings.inputFormats
                          .filter(
                            (f) =>
                              f.id === rule.input ||
                              unused?.some((u) => u.id === f.id)
                          )
                          .map((f) => (
                            <option key={f.id} value={f.id}>
                              {f.label}
                            </option>
                          ))}
                      </Form.Control>
                    </td>
                    <td>
                      <Form.Control
                        as="select"
                        className="input-control"
                        aria-label={`Output format for ${settings.inputFormats.find((f) => f.id === rule.input)?.label ?? rule.input}`}
                        value={rule.output}
                        disabled={saving}
                        onChange={(e) => {
                          const output = e.target.value;
                          setRules(
                            rules.map((r, i) =>
                              i === index
                                ? {
                                    ...r,
                                    output,
                                    decodingSpeed: clampDecodingSpeed(
                                      r.decodingSpeed,
                                      output
                                    ),
                                  }
                                : r
                            )
                          );
                        }}
                      >
                        {outputsFor(rule.input).map((f) => (
                          <option key={f.id} value={f.id}>
                            {f.label}
                          </option>
                        ))}
                      </Form.Control>
                    </td>
                    {(["quality", "effort"] as const).map((key) => (
                      <td key={key}>
                        <Form.Control
                          type="number"
                          className="text-input"
                          min={key === "quality" ? 0 : 1}
                          max={key === "quality" ? 100 : 9}
                          step={1}
                          value={rule[key]}
                          disabled={saving}
                          aria-label={`${key} for ${rule.input}`}
                          onChange={(e) =>
                            setRules(
                              rules.map((r, i) =>
                                i === index
                                  ? { ...r, [key]: Number(e.target.value) }
                                  : r
                              )
                            )
                          }
                        />
                      </td>
                    ))}
                    <td>
                      {loadingWorkerFormats ? (
                        <span className="text-muted">Checking worker…</span>
                      ) : decodingSpeedLevels(rule.output) > 1 ? (
                        <Form.Control
                          type="number"
                          className="text-input"
                          min={0}
                          max={4}
                          step={1}
                          value={rule.decodingSpeed}
                          disabled={saving || loadingWorkerFormats}
                          aria-label={`Decode speed tier for ${rule.input}`}
                          onChange={(e) =>
                            setRules(
                              rules.map((r, i) =>
                                i === index
                                  ? {
                                      ...r,
                                      decodingSpeed: Number(e.target.value),
                                    }
                                  : r
                              )
                            )
                          }
                        />
                      ) : decodingSpeedLevels(rule.output) === 1 ? (
                        <Form.Check
                          type="checkbox"
                          checked={rule.decodingSpeed > 0}
                          disabled={saving || loadingWorkerFormats}
                          aria-label={`AV1 low-complexity decode for ${rule.input}`}
                          onChange={(e) =>
                            setRules(
                              rules.map((r, i) =>
                                i === index
                                  ? {
                                      ...r,
                                      decodingSpeed: e.target.checked ? 1 : 0,
                                    }
                                  : r
                              )
                            )
                          }
                        />
                      ) : (
                        <span className="text-muted">—</span>
                      )}
                    </td>
                    <td>
                      {rule.input !== "image" && rule.input !== "video" && (
                        <Button
                          size="sm"
                          variant="secondary"
                          disabled={saving}
                          aria-label={`Remove ${rule.input} rule`}
                          onClick={() =>
                            setRules(rules.filter((_, i) => i !== index))
                          }
                        >
                          Remove
                        </Button>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </Table>
            <div className="d-flex flex-wrap justify-content-between">
              <Button
                variant="secondary"
                className="mb-2"
                disabled={saving || !unused?.length}
                onClick={() => {
                  const input = unused![0].id;
                  setRules([
                    ...rules,
                    {
                      input,
                      output: outputsFor(input)[0].id,
                      quality:
                        settings.inputFormats.find((f) => f.id === input)
                          ?.family === "video"
                          ? 80
                          : 90,
                      effort: 7,
                      decodingSpeed:
                        settings.inputFormats.find((f) => f.id === input)
                          ?.family === "animation" &&
                        (settings.outputFormats.find(
                          (f) => f.id === outputsFor(input)[0].id
                        )?.decodingSpeedLevels ?? 0) > 1
                          ? 2
                          : 0,
                    },
                  ]);
                }}
              >
                Add format rule
              </Button>
              <Button
                className="mb-2"
                disabled={saving}
                onClick={() => void save()}
              >
                {saving ? "Saving…" : "Save format defaults"}
              </Button>
            </div>
            <Form.Text className="text-muted">
              Higher quality retains more detail; higher effort takes longer.
              JPEG XL quality 100 is lossless. Decode speed is codec-specific:
              JPEG XL offers tiers 0–4, while compatible AV1/libaom workers
              offer an on/off low-complexity decode mode that uses CPU. Higher
              JXL tiers can reduce compression or quality. These encoding hints
              do not guarantee faster playback in every browser. The processor
              prefers GPU, otherwise CPU. Saved defaults apply when a new
              conversion job starts. Existing running jobs keep their selected
              formats and settings.
            </Form.Text>
          </>
        )}
      </Card>
    </div>
  );
};
