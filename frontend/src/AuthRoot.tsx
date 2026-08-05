import { useEffect, useState } from "react";
import App from "./App";
import { api } from "./api";
import { AuthScreen } from "./components/AuthScreen";
import { BrandMark } from "./components/Brand";
import type { AuthStatus } from "./types";

export function AuthRoot() {
  const [status, setStatus] = useState<AuthStatus | null>(null);
  const [error, setError] = useState<string | null>(null);

  const loadStatus = async () => {
    setError(null);
    try {
      setStatus(await api.authStatus());
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : "无法连接 Paperwing 服务");
    }
  };

  useEffect(() => {
    void loadStatus();
    const unauthorized = () => {
      setStatus((current) => ({
        configured: true,
        authenticated: false,
        username: current?.username || "",
      }));
    };
    window.addEventListener("paperwing:unauthorized", unauthorized);
    return () => window.removeEventListener("paperwing:unauthorized", unauthorized);
  }, []);

  if (!status) {
    return (
      <div className="paper-texture flex h-dvh min-h-[480px] items-center justify-center bg-[#f4f3ee] px-6">
        <div className="text-center">
          <BrandMark className="mx-auto size-12" />
          <div className="mt-5 text-sm font-semibold tracking-[-0.02em] text-[#353632]">
            {error ? "无法打开 Paperwing" : "正在安全连接…"}
          </div>
          {error && (
            <>
              <p className="mt-2 max-w-xs text-xs leading-5 text-[#888780]">{error}</p>
              <button
                type="button"
                onClick={() => void loadStatus()}
                className="focus-ring mt-5 rounded-xl bg-[#292a27] px-4 py-2.5 text-xs font-semibold text-white"
              >
                重新连接
              </button>
            </>
          )}
        </div>
      </div>
    );
  }

  if (!status.authenticated) {
    return (
      <AuthScreen
        setup={!status.configured}
        initialUsername={status.username}
        onAuthenticated={setStatus}
      />
    );
  }

  return (
    <App
      onLogout={async () => {
        await api.logout();
        setStatus({ configured: true, authenticated: false, username: status.username });
      }}
    />
  );
}
