import React from "react";
import { useIntl } from "react-intl";
import { Link } from "react-router-dom";
import * as GQL from "src/core/generated-graphql";
import { BooleanSetting } from "./Inputs";
import { SettingSection } from "./SettingSection";
import { useSettings } from "./context";

const defaultInheritance: GQL.AssociationInheritanceSettingsInput = {
  characters: true,
  artists: true,
  copyrights: true,
  tags: true,
};

export const AssociationInheritanceSettings: React.FC = () => {
  const intl = useIntl();
  const { defaults, saveDefaults } = useSettings();
  const inheritance = {
    ...defaultInheritance,
    ...defaults.associationInheritance,
  };

  function saveInheritance(
    key: keyof GQL.AssociationInheritanceSettingsInput,
    value: boolean
  ) {
    saveDefaults({
      associationInheritance: {
        ...inheritance,
        [key]: value,
      },
    });
  }

  return (
    <SettingSection
      id="association-inheritance-settings"
      headingID="config.association_inheritance.heading"
      subHeadingID="config.association_inheritance.description"
    >
      <p>
        <Link to="/settings?tab=tasks#association-inheritance-task">
          {intl.formatMessage({
            id: "config.association_inheritance.review.preview_link",
          })}
        </Link>
      </p>
      <BooleanSetting
        id="inherit-character-ancestors"
        headingID="config.association_inheritance.characters.heading"
        subHeadingID="config.association_inheritance.characters.description"
        checked={inheritance.characters ?? true}
        onChange={(value) => saveInheritance("characters", value)}
      />
      <BooleanSetting
        id="inherit-artist-ancestors"
        headingID="config.association_inheritance.artists.heading"
        subHeadingID="config.association_inheritance.artists.description"
        checked={inheritance.artists ?? true}
        onChange={(value) => saveInheritance("artists", value)}
      />
      <BooleanSetting
        id="inherit-copyright-ancestors"
        headingID="config.association_inheritance.copyrights.heading"
        subHeadingID="config.association_inheritance.copyrights.description"
        checked={inheritance.copyrights ?? true}
        onChange={(value) => saveInheritance("copyrights", value)}
      />
      <BooleanSetting
        id="inherit-tag-ancestors"
        headingID="config.association_inheritance.tags.heading"
        subHeadingID="config.association_inheritance.tags.description"
        checked={inheritance.tags ?? true}
        onChange={(value) => saveInheritance("tags", value)}
      />
    </SettingSection>
  );
};
