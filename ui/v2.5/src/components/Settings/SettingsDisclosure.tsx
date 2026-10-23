import React from "react";

// Native details keeps the original controls and results mounted while collapsed.
export const SettingsDisclosure: React.FC<{
  id: string;
  title: string;
  description: string;
}> = ({ id, title, description, children }) => (
  <details className="settings-details settings-disclosure" id={id}>
    <summary aria-labelledby={`${id}-title`} aria-describedby={`${id}-help`}>
      <span className="settings-disclosure-title" id={`${id}-title`}>
        {title}
      </span>
      <span className="settings-disclosure-help" id={`${id}-help`}>
        {description}
      </span>
    </summary>
    <div className="settings-disclosure-content">{children}</div>
  </details>
);
