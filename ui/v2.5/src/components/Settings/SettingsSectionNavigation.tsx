import { Form, Nav } from "react-bootstrap";
import { LinkContainer } from "react-router-bootstrap";
import { useHistory, useLocation } from "react-router-dom";
import { useIntl } from "react-intl";
import {
  settingsSearch,
  settingsTabs,
  SettingsTab,
} from "./settingsNavigation";

const labels: Record<SettingsTab, { id: string; defaultMessage: string }> = {
  tasks: { id: "config.categories.tasks", defaultMessage: "Tasks" },
  library: { id: "library", defaultMessage: "Library" },
  processing: {
    id: "config.categories.processing",
    defaultMessage: "Processing",
  },
  interface: { id: "config.categories.interface", defaultMessage: "Interface" },
  security: { id: "config.categories.security", defaultMessage: "Security" },
  "metadata-providers": {
    id: "config.categories.metadata_providers",
    defaultMessage: "Metadata Providers",
  },
  services: { id: "config.categories.services", defaultMessage: "Services" },
  system: { id: "config.categories.system", defaultMessage: "System" },
  plugins: { id: "config.categories.plugins", defaultMessage: "Plugins" },
  logs: { id: "config.categories.logs", defaultMessage: "Logs" },
  tools: { id: "config.categories.tools", defaultMessage: "Tools" },
  changelog: { id: "config.categories.changelog", defaultMessage: "Changelog" },
  about: { id: "config.categories.about", defaultMessage: "About" },
};

export function SettingsSectionNavigation({ tab }: { tab: SettingsTab }) {
  const intl = useIntl();
  const history = useHistory();
  const location = useLocation();
  const destination = (next: SettingsTab) => ({
    pathname: location.pathname,
    search: settingsSearch(location.search, next),
    hash: "",
  });

  return (
    <>
      <Form.Group className="d-md-none mb-2" controlId="settings-section">
        <Form.Label>
          {intl.formatMessage({
            id: "config.settings_section",
            defaultMessage: "Settings section",
          })}
        </Form.Label>
        <Form.Control
          as="select"
          className="input-control"
          value={tab}
          onChange={(event) => {
            const next = event.currentTarget.value as SettingsTab;
            if (next !== tab && settingsTabs.includes(next))
              history.push(destination(next));
          }}
        >
          {settingsTabs.map((key) => (
            <option key={key} value={key}>
              {intl.formatMessage(labels[key])}
            </option>
          ))}
        </Form.Control>
      </Form.Group>
      <Nav variant="pills" className="d-none d-md-flex flex-column">
        {settingsTabs.map((key) => (
          <Nav.Item key={key}>
            <LinkContainer to={destination(key)} isActive={() => key === tab}>
              <Nav.Link eventKey={key}>
                {intl.formatMessage(labels[key])}
              </Nav.Link>
            </LinkContainer>
          </Nav.Item>
        ))}
      </Nav>
    </>
  );
}
