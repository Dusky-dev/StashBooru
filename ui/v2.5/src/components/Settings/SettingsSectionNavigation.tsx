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
  experimental: {
    id: "config.categories.experimental",
    defaultMessage: "Experimental",
  },
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
  about: { id: "config.categories.about", defaultMessage: "About" },
};

export function SettingsSectionNavigation({
  tab,
  showExperimental,
}: {
  tab: SettingsTab;
  showExperimental: boolean;
}) {
  const intl = useIntl();
  // Experimental stays listed while open so the current section is never blank.
  const tabs = settingsTabs.filter(
    (key) => key !== "experimental" || showExperimental || tab === key
  );
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
          {tabs.map((key) => (
            <option key={key} value={key}>
              {intl.formatMessage(labels[key])}
            </option>
          ))}
        </Form.Control>
      </Form.Group>
      <Nav variant="pills" className="d-none d-md-flex flex-column">
        {tabs.map((key) => (
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
