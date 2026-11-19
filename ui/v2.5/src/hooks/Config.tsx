import React from "react";
import * as GQL from "src/core/generated-graphql";

export interface IContext {
  configuration: GQL.ConfigDataFragment;
}

export const ConfigurationContext = React.createContext<IContext | null>(null);

export const useConfigurationContext = () => {
  const context = React.useContext(ConfigurationContext);

  if (context === null) {
    throw new Error(
      "useConfigurationContext must be used within a ConfigurationProvider"
    );
  }

  return context;
};

export const useConfigurationContextOptional = () => {
  return React.useContext(ConfigurationContext);
};

/**
 * Whether the upstream Organized flag is shown and used (Settings → Interface).
 * Off unless turned on; saved filters and data keep working either way.
 */
export function useOrganizedEnabled(): boolean {
  const context = React.useContext(ConfigurationContext);
  return !!context?.configuration.ui?.organizedEnabled;
}

/** Renders its children only while the Organized flag is in use. */
export const OrganizedOnly: React.FC<React.PropsWithChildren> = ({
  children,
}) => (useOrganizedEnabled() ? children : null);

export const ConfigurationProvider: React.FC<
  React.PropsWithChildren<IContext>
> = ({ configuration, children }) => {
  return (
    <ConfigurationContext.Provider
      value={{
        configuration,
      }}
    >
      {children}
    </ConfigurationContext.Provider>
  );
};
