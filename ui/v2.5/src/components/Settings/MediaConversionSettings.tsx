import React, { useEffect, useState } from "react";
import { Alert, Button, Card, Form, Spinner, Table } from "react-bootstrap";
import { useToast } from "src/hooks/Toast";
import {
  conversionEndpoint,
  conversionResponse,
  ConversionConfig,
  ConversionSettings,
} from "../Shared/mediaConversion";

interface Rule {
  input: string;
  output: string;
  quality: number;
  effort: number;
}

export const MediaConversionSettings: React.FC = () => {
  const Toast = useToast();
  const [settings, setSettings] = useState<ConversionSettings>();
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
            .map((f) => ({
              input: f.id,
              output: value.config.formatDefaults[f.id],
              quality:
                value.config.encodingDefaults[f.id]?.quality ??
                value.config.encodingDefaults[
                  f.family === "video" ? "video" : "image"
                ]?.quality ??
                (f.family === "video" ? 80 : 90),
              effort:
                value.config.encodingDefaults[f.id]?.effort ??
                value.config.encodingDefaults[
                  f.family === "video" ? "video" : "image"
                ]?.effort ??
                7,
            }))
        );
      })
      .catch((e: Error) => {
        if (!controller.signal.aborted) setError(e.message);
      });
    return () => controller.abort();
  }, []);

  const outputsFor = (input: string) => {
    const family = settings?.inputFormats.find((f) => f.id === input)?.family;
    return (settings?.outputFormats ?? []).filter(
      (f) =>
        (family !== "video" || f.family === "video") &&
        (family !== "animation" || f.family !== "image")
    );
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
              { quality: r.quality, effort: r.effort },
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
        Choose the output, quality and effort for each input format. Single and
        batch conversions use these saved defaults.
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
                              i === index ? { ...r, input, output } : r
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
                        onChange={(e) =>
                          setRules(
                            rules.map((r, i) =>
                              i === index ? { ...r, output: e.target.value } : r
                            )
                          )
                        }
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
              Controls that a codec does not use are ignored. JPEG XL quality
              100 is lossless. The processor prefers GPU, otherwise CPU. Saved
              defaults apply when a new conversion job starts. Existing running
              jobs keep their selected formats and settings.
            </Form.Text>
          </>
        )}
      </Card>
    </div>
  );
};
