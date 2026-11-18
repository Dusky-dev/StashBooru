import React from "react";
import { Button } from "react-bootstrap";
import { defineMessages, FormattedMessage, useIntl } from "react-intl";
import { Link } from "react-router-dom";
import { Setting } from "./Inputs";
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

/**
 * Tools now live in a Tasks group. The plugin patch point and the
 * #tools-health / #tools-developer bookmarks are unchanged.
 */
export const SettingsTools: React.FC = () => {
  const intl = useIntl();
  return (
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
      <Setting
        heading={
          <Link to="/sceneDuplicateChecker">
            <Button variant="secondary">
              <FormattedMessage id="config.tools.scene_duplicate_checker" />
            </Button>
          </Link>
        }
      />
    </SettingsToolsSection>
  );
};
