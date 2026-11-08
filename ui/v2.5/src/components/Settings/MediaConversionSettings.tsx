import React, { useCallback, useEffect, useRef, useState } from "react";
import { Alert, Button, Card, Form, Spinner, Table } from "react-bootstrap";
import { useToast } from "src/hooks/Toast";
import { ProcessingBackendOptions } from "../Shared/ProcessingBackendOptions";
import {
  conversionEndpoint,
  conversionResponse,
  ConversionConfig,
  ConversionFormat,
  ConversionSettings,
  SavingsThresholds,
} from "../Shared/mediaConversion";
import {
  SavingsThresholdFields,
  validSavings,
} from "../Shared/SavingsThresholdFields";

interface Rule {
  input: string;
  output: string;
  enabled: boolean;
  quality: number;
  effort: number;
  decodingSpeed: number;
  allowLarger: boolean;
  lossless: boolean;
  dropAudio: boolean;
  allowAlphaLoss: boolean;
}

export const MediaConversionSettings: React.FC = () => {
  const Toast = useToast();
  const [settings, setSettings] = useState<ConversionSettings>();
  const loadController = useRef<AbortController>();
  const [loadingSettings, setLoadingSettings] = useState(true);
  const [loadError, setLoadError] = useState("");
  const [workerFormats, setWorkerFormats] = useState<ConversionFormat[]>([]);
  const [loadingWorkerFormats, setLoadingWorkerFormats] = useState(false);
  const [workerCapabilityError, setWorkerCapabilityError] = useState("");
  const [rules, setRules] = useState<Rule[]>([]);
  const [saving, setSaving] = useState(false);
  const [showAdvancedRules, setShowAdvancedRules] = useState(false);
  const [error, setError] = useState("");
  const [backend, setBackend] = useState("auto");
  const [savings, setSavings] = useState<SavingsThresholds>({
    minimumSavedBytes: 0,
    minimumSavedPercent: 0,
  });
  const [trialCacheGiB, setTrialCacheGiB] = useState(10);
  const [trialTTLHours, setTrialTTLHours] = useState(24);

  const loadSettings = useCallback(() => {
    loadController.current?.abort();
    const controller = new AbortController();
    loadController.current = controller;
    setLoadingSettings(true);
    void fetch(`${conversionEndpoint}?config=1`, { signal: controller.signal })
      .then(conversionResponse<ConversionSettings>)
      .then((value) => {
        if (controller.signal.aborted) return;
        setLoadError("");
        setSettings(value);
        setBackend(value.config.backend);
        setSavings(value.config.savings);
        setTrialCacheGiB(value.config.trialCacheLimitBytes / 1024 ** 3);
        setTrialTTLHours(value.config.trialTTLHours || 24);
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
                enabled: value.config.formatRulesEnabled?.[f.id] !== false,
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
                allowLarger:
                  saved?.allowLarger ?? fallback?.allowLarger ?? false,
                lossless: saved?.lossless ?? fallback?.lossless ?? false,
                dropAudio: saved?.dropAudio ?? fallback?.dropAudio ?? false,
                allowAlphaLoss:
                  saved?.allowAlphaLoss ?? fallback?.allowAlphaLoss ?? false,
              };
            })
        );
      })
      .catch((e: Error) => {
        if (!controller.signal.aborted) setLoadError(e.message);
      })
      .finally(() => {
        if (!controller.signal.aborted) setLoadingSettings(false);
      });
  }, []);

  useEffect(() => {
    loadSettings();
    return () => loadController.current?.abort();
  }, [loadSettings]);

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

  const supportsSavedControl = (output: string, control: string) =>
    settings?.outputFormats
      .find((f) => f.id === output)
      ?.controls.includes(control) ?? false;

  // Saved defaults describe user preferences, so keep them editable even when
  // the selected worker is older or temporarily unavailable. Conversion jobs
  // still filter options against that worker's actual capabilities.
  const decodingSpeedLevels = catalogDecodingSpeedLevels;

  const workerSupportsDecodingSpeed = (output: string) => {
    const worker = workerFormats.find((f) => f.id === output);
    return (
      !!worker &&
      (worker.controls.includes("decodingSpeed") ||
        ((output === "jxl" || output === "ajxl") &&
          worker.controls.includes("fasterDecoding")))
    );
  };

  const clampDecodingSpeed = (speed: number, output: string) => {
    const limit = decodingSpeedLevels(output);
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
          savings,
          trialCacheLimitBytes: Math.round(trialCacheGiB * 1024 ** 3),
          trialTTLHours,
          encodingDefaults: Object.fromEntries(
            rules.map((r) => [
              r.input,
              {
                quality: r.quality,
                effort: r.effort,
                decodingSpeed: r.decodingSpeed,
                allowLarger: r.allowLarger,
                lossless: supportsSavedControl(r.output, "lossless")
                  ? r.lossless
                  : false,
                dropAudio: r.dropAudio,
                allowAlphaLoss: r.allowAlphaLoss,
              },
            ])
          ),
          formatDefaults: Object.fromEntries(
            rules.map((r) => [r.input, r.output])
          ),
          formatRulesEnabled: Object.fromEntries(
            rules.map((r) => [r.input, r.enabled])
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
        Configure conversion defaults. Start conversions from selected media.
      </div>
      <Card className="p-3">
        {!settings && (
          <div className="mb-3">
            {loadError && (
              <Alert variant="danger">
                Could not load saved conversion defaults: {loadError}
                <div className="mt-2">
                  <Button
                    variant="secondary"
                    disabled={loadingSettings}
                    onClick={loadSettings}
                  >
                    Retry loading defaults
                  </Button>
                </div>
              </Alert>
            )}
            {loadingSettings && (
              <div role="status">
                <Spinner animation="border" size="sm" aria-hidden="true" />{" "}
                Loading conversion defaults…
              </div>
            )}
          </div>
        )}
        <details
          id="converter-advanced-defaults"
          className="settings-details mb-3"
        >
          <summary>Advanced conversion defaults</summary>
          <div className="pt-3">
            <h2 className="h5">Compression savings</h2>
            <SavingsThresholdFields
              value={savings}
              onChange={setSavings}
              disabled={saving || !settings}
              prefix="converter-global"
            />
            <details className="settings-details mb-3">
              <summary>Saved trial storage and expiry</summary>
              <div className="pt-3">
                <Form.Group controlId="converter-trial-cache">
                  <Form.Label>Trial cache limit (GiB)</Form.Label>
                  <Form.Control
                    className="text-input"
                    type="number"
                    min={0}
                    step={0.25}
                    value={trialCacheGiB}
                    disabled={saving || !settings}
                    onChange={(e) => setTrialCacheGiB(Number(e.target.value))}
                  />
                </Form.Group>
                <Form.Group controlId="converter-trial-expiry">
                  <Form.Label>Trial expiry (hours)</Form.Label>
                  <Form.Control
                    className="text-input"
                    type="number"
                    min={1}
                    max={168}
                    step={1}
                    value={trialTTLHours}
                    disabled={saving || !settings}
                    onChange={(e) => setTrialTTLHours(Number(e.target.value))}
                  />
                </Form.Group>
              </div>
            </details>
          </div>
        </details>
        <Form.Group controlId="converter-default-backend">
          <Form.Label>Run on</Form.Label>
          <Form.Control
            as="select"
            className="input-control"
            value={backend}
            disabled={saving || !settings}
            onChange={(e) => setBackend(e.target.value)}
          >
            <ProcessingBackendOptions />
          </Form.Control>
        </Form.Group>

        {error && <Alert variant="danger">{error}</Alert>}
        {workerCapabilityError && (
          <Alert variant="warning">
            Could not read codec support from the selected worker. Saved
            preferences remain editable, but a worker must support a codec
            option to apply it during conversion.
          </Alert>
        )}
        {settings && (
          <>
            <div className="d-flex flex-wrap align-items-center justify-content-between mb-2">
              <div>
                <h2 className="h5">Format rules</h2>
                <p className="small text-muted mb-2">
                  Choose what to convert automatically. Disabled rules keep
                  their settings.
                </p>
              </div>
              <Button
                size="sm"
                variant="outline-secondary"
                className="mb-2"
                disabled={saving}
                aria-expanded={showAdvancedRules}
                onClick={() => setShowAdvancedRules((value) => !value)}
              >
                {showAdvancedRules
                  ? "Hide advanced rule options"
                  : "Show advanced rule options"}
              </Button>
            </div>
            <Table responsive size="sm">
              <thead>
                <tr>
                  <th>Enabled</th>
                  <th>Input format</th>
                  <th>Default output format</th>
                  <th className={showAdvancedRules ? undefined : "d-none"}>
                    Quality (0–100)
                  </th>
                  <th className={showAdvancedRules ? undefined : "d-none"}>
                    Effort (1–9)
                  </th>
                  <th className={showAdvancedRules ? undefined : "d-none"}>
                    Decode speed
                  </th>
                  <th className={showAdvancedRules ? undefined : "d-none"}>
                    Conversion options
                  </th>
                  <th>
                    <span className="sr-only">Remove rule</span>
                  </th>
                </tr>
              </thead>
              <tbody>
                {rules.map((rule, index) => (
                  <tr key={rule.input}>
                    <td>
                      <Form.Check
                        type="checkbox"
                        aria-label={`Enable ${rule.input} format rule`}
                        checked={rule.enabled}
                        disabled={saving}
                        onChange={(event) => {
                          const enabled = event.currentTarget.checked;
                          setRules((previous) =>
                            previous.map((r, i) =>
                              i === index ? { ...r, enabled } : r
                            )
                          );
                        }}
                      />
                    </td>
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
                                    lossless:
                                      supportsSavedControl(
                                        output,
                                        "lossless"
                                      ) && r.lossless,
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
                                    lossless:
                                      supportsSavedControl(
                                        output,
                                        "lossless"
                                      ) && r.lossless,
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
                      <td
                        key={key}
                        className={showAdvancedRules ? undefined : "d-none"}
                      >
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
                    <td className={showAdvancedRules ? undefined : "d-none"}>
                      {decodingSpeedLevels(rule.output) > 1 ? (
                        <Form.Control
                          type="number"
                          className="text-input"
                          min={0}
                          max={decodingSpeedLevels(rule.output)}
                          step={1}
                          value={rule.decodingSpeed}
                          disabled={saving}
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
                          disabled={saving}
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
                      {decodingSpeedLevels(rule.output) > 0 &&
                        !loadingWorkerFormats &&
                        !workerCapabilityError &&
                        workerFormats.length > 0 &&
                        !workerSupportsDecodingSpeed(rule.output) && (
                          <Form.Text className="d-block text-warning">
                            Not advertised by this worker
                          </Form.Text>
                        )}
                    </td>
                    <td className={showAdvancedRules ? undefined : "d-none"}>
                      <Form.Check
                        type="checkbox"
                        label="Keep larger"
                        checked={rule.allowLarger}
                        disabled={saving}
                        onChange={(e) =>
                          setRules(
                            rules.map((r, i) =>
                              i === index
                                ? { ...r, allowLarger: e.target.checked }
                                : r
                            )
                          )
                        }
                      />
                      {supportsSavedControl(rule.output, "lossless") && (
                        <Form.Check
                          type="checkbox"
                          label="Lossless"
                          checked={rule.lossless}
                          disabled={saving}
                          onChange={(e) =>
                            setRules(
                              rules.map((r, i) =>
                                i === index
                                  ? { ...r, lossless: e.target.checked }
                                  : r
                              )
                            )
                          }
                        />
                      )}
                      <Form.Check
                        type="checkbox"
                        label="Allow audio loss"
                        checked={rule.dropAudio}
                        disabled={saving}
                        onChange={(e) =>
                          setRules(
                            rules.map((r, i) =>
                              i === index
                                ? { ...r, dropAudio: e.target.checked }
                                : r
                            )
                          )
                        }
                      />
                      <Form.Check
                        type="checkbox"
                        label="Allow alpha loss"
                        checked={rule.allowAlphaLoss}
                        disabled={saving}
                        onChange={(e) =>
                          setRules(
                            rules.map((r, i) =>
                              i === index
                                ? { ...r, allowAlphaLoss: e.target.checked }
                                : r
                            )
                          )
                        }
                      />
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
                      enabled: true,
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
                      allowLarger: false,
                      lossless: false,
                      dropAudio: false,
                      allowAlphaLoss: false,
                    },
                  ]);
                }}
              >
                Add format rule
              </Button>
              <Button
                className="mb-2"
                disabled={
                  saving ||
                  !validSavings(savings) ||
                  !Number.isSafeInteger(
                    Math.round(trialCacheGiB * 1024 ** 3)
                  ) ||
                  trialCacheGiB < 0 ||
                  !Number.isInteger(trialTTLHours) ||
                  trialTTLHours < 1 ||
                  trialTTLHours > 168
                }
                onClick={() => void save()}
              >
                {saving ? "Saving…" : "Save conversion defaults"}
              </Button>
            </div>
          </>
        )}
      </Card>
    </div>
  );
};
