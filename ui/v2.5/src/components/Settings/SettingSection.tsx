import React, { PropsWithChildren } from "react";
import { Card } from "react-bootstrap";
import { useIntl } from "react-intl";
import { useSettings } from "./context";

interface ISettingGroup {
  id?: string;
  headingID?: string;
  heading?: React.ReactNode;
  subHeadingID?: string;
  advanced?: boolean;
}

export const SettingSection: React.FC<PropsWithChildren<ISettingGroup>> = ({
  id,
  children,
  headingID,
  heading,
  subHeadingID,
  advanced,
}) => {
  const intl = useIntl();
  const { advancedMode } = useSettings();

  if (advanced && !advancedMode) return null;

  return (
    <div className="setting-section" id={id}>
      {(heading || headingID) && (
        <h1>{heading ?? intl.formatMessage({ id: headingID })}</h1>
      )}
      {subHeadingID ? (
        <div className="sub-heading">
          {intl.formatMessage({ id: subHeadingID })}
        </div>
      ) : undefined}
      <Card>{children}</Card>
    </div>
  );
};
