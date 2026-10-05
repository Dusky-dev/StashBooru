import React, { useEffect } from "react";
import { Tab, Nav, Row, Col, Form } from "react-bootstrap";
import { Redirect, useLocation } from "react-router-dom";
import { LinkContainer } from "react-router-bootstrap";
import { FormattedMessage } from "react-intl";
import { Helmet } from "react-helmet";
import { useTitleProps } from "src/hooks/title";
import { SettingsAboutPanel } from "./SettingsAboutPanel";
import { SettingsConfigurationPanel } from "./SettingsSystemPanel";
import { SettingsInterfacePanel } from "./SettingsInterfacePanel/SettingsInterfacePanel";
import { SettingsLogsPanel } from "./SettingsLogsPanel";
import { SettingsTasksPanel } from "./Tasks/SettingsTasksPanel";
import { SettingsPluginsPanel } from "./SettingsPluginsPanel";
import { SettingsScrapingPanel } from "./SettingsScrapingPanel";
import { SettingsToolsPanel } from "./SettingsToolsPanel";
import { SettingsServicesPanel } from "./SettingsServicesPanel";
import { SettingsContext, useSettings } from "./context";
import { SettingsLibraryPanel } from "./SettingsLibraryPanel";
import { SettingsSecurityPanel } from "./SettingsSecurityPanel";
import { SettingsProcessingPanel } from "./SettingsProcessingPanel";
import {
  resolveSettingsTab,
  settingsSearch,
  SettingsTab,
} from "./settingsNavigation";
import Changelog from "../Changelog/Changelog";
import { TroubleshootingModeButton } from "../TroubleshootingMode/TroubleshootingModeButton";
import { useTroubleshootingMode } from "../TroubleshootingMode/useTroubleshootingMode";

const SettingTabs: React.FC<{ tab: SettingsTab }> = ({ tab }) => {
  const { advancedMode, setAdvancedMode, loading } = useSettings();
  const { hash } = useLocation();
  const { isActive: troubleshootingModeActive } = useTroubleshootingMode();

  useEffect(() => {
    if (!hash || loading) return;
    const pane = document.getElementById(`configuration-tabs-tabpane-${tab}`);
    if (!pane) return;
    let frame = 0;
    const align = () => {
      cancelAnimationFrame(frame);
      frame = requestAnimationFrame(() => {
        const target = document.getElementById(hash.slice(1));
        if (target && pane.contains(target))
          target.scrollIntoView({ block: "start" });
      });
    };
    // Async worker/configuration responses can change section heights. Keep a
    // bookmark aligned through that initial layout, until the user takes over.
    const observer = new ResizeObserver(align);
    observer.observe(pane);
    align();
    const stop = () => {
      observer.disconnect();
      cancelAnimationFrame(frame);
    };
    const events = ["pointerdown", "wheel", "touchstart", "keydown"] as const;
    for (const event of events)
      window.addEventListener(event, stop, { passive: true });
    return () => {
      stop();
      for (const event of events) window.removeEventListener(event, stop);
    };
  }, [hash, tab, loading]);

  const titleProps = useTitleProps({ id: "settings" });

  return (
    <Tab.Container activeKey={tab} id="configuration-tabs" mountOnEnter>
      <Helmet {...titleProps} />
      <Row>
        <Col id="settings-menu-container" sm={3} md={3} xl={2}>
          <Nav variant="pills" className="flex-column">
            <Nav.Item>
              <LinkContainer to="/settings?tab=tasks">
                <Nav.Link eventKey="tasks">
                  <FormattedMessage id="config.categories.tasks" />
                </Nav.Link>
              </LinkContainer>
            </Nav.Item>
            <Nav.Item>
              <LinkContainer to="/settings?tab=library">
                <Nav.Link eventKey="library">
                  <FormattedMessage id="library" />
                </Nav.Link>
              </LinkContainer>
            </Nav.Item>
            <Nav.Item>
              <LinkContainer to="/settings?tab=processing">
                <Nav.Link eventKey="processing">
                  <FormattedMessage
                    id="config.categories.processing"
                    defaultMessage="Processing"
                  />
                </Nav.Link>
              </LinkContainer>
            </Nav.Item>
            <Nav.Item>
              <LinkContainer to="/settings?tab=interface">
                <Nav.Link eventKey="interface">
                  <FormattedMessage id="config.categories.interface" />
                </Nav.Link>
              </LinkContainer>
            </Nav.Item>
            <Nav.Item>
              <LinkContainer to="/settings?tab=security">
                <Nav.Link eventKey="security">
                  <FormattedMessage id="config.categories.security" />
                </Nav.Link>
              </LinkContainer>
            </Nav.Item>
            <Nav.Item>
              <LinkContainer to="/settings?tab=metadata-providers">
                <Nav.Link eventKey="metadata-providers">
                  <FormattedMessage id="config.categories.metadata_providers" />
                </Nav.Link>
              </LinkContainer>
            </Nav.Item>
            <Nav.Item>
              <LinkContainer to="/settings?tab=services">
                <Nav.Link eventKey="services">
                  <FormattedMessage id="config.categories.services" />
                </Nav.Link>
              </LinkContainer>
            </Nav.Item>
            <Nav.Item>
              <LinkContainer to="/settings?tab=system">
                <Nav.Link eventKey="system">
                  <FormattedMessage id="config.categories.system" />
                </Nav.Link>
              </LinkContainer>
            </Nav.Item>
            <Nav.Item>
              <LinkContainer to="/settings?tab=plugins">
                <Nav.Link eventKey="plugins">
                  <FormattedMessage id="config.categories.plugins" />
                </Nav.Link>
              </LinkContainer>
            </Nav.Item>
            <Nav.Item>
              <LinkContainer to="/settings?tab=logs">
                <Nav.Link eventKey="logs">
                  <FormattedMessage id="config.categories.logs" />
                </Nav.Link>
              </LinkContainer>
            </Nav.Item>
            <Nav.Item>
              <LinkContainer to="/settings?tab=tools">
                <Nav.Link eventKey="tools">
                  <FormattedMessage id="config.categories.tools" />
                </Nav.Link>
              </LinkContainer>
            </Nav.Item>
            <Nav.Item>
              <LinkContainer to="/settings?tab=changelog">
                <Nav.Link eventKey="changelog">
                  <FormattedMessage id="config.categories.changelog" />
                </Nav.Link>
              </LinkContainer>
            </Nav.Item>
            <Nav.Item>
              <LinkContainer to="/settings?tab=about">
                <Nav.Link eventKey="about">
                  <FormattedMessage id="config.categories.about" />
                </Nav.Link>
              </LinkContainer>
            </Nav.Item>
            <Nav.Item>
              <div className="advanced-switch">
                <Form.Label htmlFor="advanced-settings">
                  <FormattedMessage id="config.advanced_mode" />
                </Form.Label>
                <Form.Switch
                  id="advanced-settings"
                  checked={advancedMode}
                  onChange={() => setAdvancedMode(!advancedMode)}
                />
              </div>
            </Nav.Item>
            {!troubleshootingModeActive && <TroubleshootingModeButton />}
            <hr className="d-sm-none" />
          </Nav>
        </Col>
        <Col
          id="settings-container"
          sm={{ offset: 3 }}
          md={{ offset: 3 }}
          xl={{ offset: 2 }}
        >
          <Tab.Content className="mx-auto">
            <Tab.Pane eventKey="library">
              <SettingsLibraryPanel />
            </Tab.Pane>
            <Tab.Pane eventKey="processing" mountOnEnter>
              <SettingsProcessingPanel />
            </Tab.Pane>
            <Tab.Pane eventKey="interface">
              <SettingsInterfacePanel />
            </Tab.Pane>
            <Tab.Pane eventKey="security">
              <SettingsSecurityPanel />
            </Tab.Pane>
            <Tab.Pane eventKey="tasks">
              <SettingsTasksPanel />
            </Tab.Pane>
            <Tab.Pane eventKey="services" unmountOnExit>
              <SettingsServicesPanel />
            </Tab.Pane>
            <Tab.Pane eventKey="tools" unmountOnExit>
              <SettingsToolsPanel />
            </Tab.Pane>
            <Tab.Pane eventKey="metadata-providers" unmountOnExit>
              <SettingsScrapingPanel />
            </Tab.Pane>
            <Tab.Pane eventKey="system">
              <SettingsConfigurationPanel />
            </Tab.Pane>
            <Tab.Pane eventKey="plugins" unmountOnExit>
              <SettingsPluginsPanel />
            </Tab.Pane>
            <Tab.Pane eventKey="logs" unmountOnExit>
              <SettingsLogsPanel />
            </Tab.Pane>
            <Tab.Pane eventKey="changelog" unmountOnExit>
              <Changelog />
            </Tab.Pane>
            <Tab.Pane eventKey="about" unmountOnExit>
              <SettingsAboutPanel />
            </Tab.Pane>
          </Tab.Content>
        </Col>
      </Row>
    </Tab.Container>
  );
};

export const Settings: React.FC = () => {
  const location = useLocation();
  const tab = resolveSettingsTab(location.search, location.hash);

  if (new URLSearchParams(location.search).get("tab") !== tab) {
    return (
      <Redirect
        to={{
          ...location,
          search: settingsSearch(location.search, tab),
        }}
      />
    );
  }

  return (
    <SettingsContext>
      <SettingTabs tab={tab} />
    </SettingsContext>
  );
};

export default Settings;
