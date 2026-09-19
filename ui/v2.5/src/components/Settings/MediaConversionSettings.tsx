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
}

export const MediaConversionSettings: React.FC = () => {
  const Toast = useToast();
  const [settings, setSettings] = useState<ConversionSettings>();
  const [rules, setRules] = useState<Rule[]>([]);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    const controller = new AbortController();
    void fetch(`${conversionEndpoint}?config=1`, { signal: controller.signal })
      .then(conversionResponse<ConversionSettings>)
      .then((value) => {
        if (controller.signal.aborted) return;
        setSettings(value);
        setRules(
          value.inputFormats
            .filter((f) => value.config.formatDefaults[f.id])
            .map((f) => ({
              input: f.id,
              output: value.config.formatDefaults[f.id],
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
        Choose the output for each input format. Single and batch conversions
        use these defaults unless you choose a different output in the
        converter.
      </div>
      <Card className="p-3">
        <p>
          Conversions automatically prefer the remote inference worker
          configured above when it is reachable, otherwise they run on this
          server. The processor defaults to{" "}
          <strong>Prefer GPU, otherwise CPU</strong>.
        </p>
        <p className="text-muted">
          Animated PNG and WebP are detected separately from still images.
          Formats without a specific rule use Other images or Other videos.
          Actual codec availability depends on the selected worker.
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
                              i === index ? { input, output } : r
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
                    { input, output: outputsFor(input)[0].id },
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
              Saved defaults apply when a new conversion job starts. Existing
              running jobs keep their selected formats and settings.
            </Form.Text>
          </>
        )}
      </Card>
    </div>
  );
};
