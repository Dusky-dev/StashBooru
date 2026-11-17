import React from "react";
import { Button } from "react-bootstrap";
import { defineMessages, FormattedMessage, useIntl } from "react-intl";
import { Link } from "react-router-dom";
import { Setting } from "./Inputs";
import { SettingSection } from "./SettingSection";
import { SettingsDisclosure } from "./SettingsDisclosure";
import { MetadataHealth } from "./MetadataHealth";
import { PatchContainerComponent } from "src/patch";
import { ExternalLink } from "../Shared/ExternalLink";

const SettingsToolsSection = PatchContainerComponent("SettingsToolsSection");

const sections = defineMessages({
  health: {
    id: "config.tools.sections.health",
    defaultMessage: "Metadata Health",
  },
  health_help: {
    id: "config.tools.sections.health_help",
    defaultMessage:
      "Check library metadata for conflicts and broken associations without changing it.",
  },
  developer: {
    id: "config.tools.sections.developer",
    defaultMessage: "Developer tools",
  },
  developer_help: {
    id: "config.tools.sections.developer_help",
    defaultMessage:
      "Open the GraphQL playground to inspect or run API operations.",
  },
});

export const SettingsToolsPanel: React.FC = () => {
  const intl = useIntl();
  return (
    <>
      <SettingSection headingID="config.tools.heading">
        <SettingsToolsSection>
          <SettingsDisclosure
            id="tools-health"
            title={intl.formatMessage(sections.health)}
            description={intl.formatMessage(sections.health_help)}
          >
            <MetadataHealth />
          </SettingsDisclosure>
          <SettingsDisclosure
            id="tools-developer"
            title={intl.formatMessage(sections.developer)}
            description={intl.formatMessage(sections.developer_help)}
          >
            <Setting
              heading={
                <ExternalLink href="/playground">
                  <Button>
                    <FormattedMessage id="config.tools.graphql_playground" />
                  </Button>
                </ExternalLink>
              }
            />
          </SettingsDisclosure>
        </SettingsToolsSection>
      </SettingSection>
      <SettingSection headingID="config.tools.scene_tools">
        <SettingsToolsSection>
          <Setting
            heading={
              <Link to="/sceneDuplicateChecker">
                <Button>
                  <FormattedMessage id="config.tools.scene_duplicate_checker" />
                </Button>
              </Link>
            }
          />
        </SettingsToolsSection>
      </SettingSection>
    </>
  );
};
