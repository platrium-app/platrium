import React, { createContext, useContext, useEffect, useState } from 'react';

type ServerInfo = {
  version: string;
  edition: string;
  isMultiTenant: boolean;
};

type ServerInfoContextType = {
  info: ServerInfo | null;
  loading: boolean;
  error: string | null;
};

const ServerInfoContext = createContext<ServerInfoContextType>({
  info: null,
  loading: true,
  error: null,
});

export const ServerInfoProvider = ({ children }: { children: React.ReactNode }) => {
  const [info, setInfo] = useState<ServerInfo | null>(null);
  const [loading, setLoading] = useState(true);
  const [error] = useState<string | null>(null);

  useEffect(() => {
    // For now, we mock the API response until the backend is built.
    // Set isMultiTenant to true to test the EE flow, false for CE flow.
    setTimeout(() => {
      setInfo({
        version: "0.1.0",
        edition: "ee",
        isMultiTenant: false, // change this to false to see the single-tenant fallback!
      });
      setLoading(false);
    }, 500);
  }, []);

  return (
    <ServerInfoContext.Provider value={{ info, loading, error }}>
      {children}
    </ServerInfoContext.Provider>
  );
};

export const useServerInfo = () => useContext(ServerInfoContext);
