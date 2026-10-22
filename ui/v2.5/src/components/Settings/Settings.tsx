import React, { useEffect } from "react";
import { Tab, Row, Col, Form } from "react-bootstrap";
import { Redirect, useLocation } from "react-router-dom";
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
import { SettingsSectionNavigation } from "./SettingsSectionNavigation";
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
        <Col id="settings-menu-container" md={3} xl={2}>
          <SettingsSectionNavigation tab={tab} />
          <div className="settings-menu-options">
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
            {!troubleshootingModeActive && <TroubleshootingModeButton />}
            <hr className="d-md-none" />
          </div>
        </Col>
        <Col id="settings-container" md={{ offset: 3 }} xl={{ offset: 2 }}>
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
