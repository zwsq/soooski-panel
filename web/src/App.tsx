import { useEffect, useState } from "react";
import { Shell, parseView, type View } from "@/components/Layout";
import { Toaster } from "@/components/ui/sonner";
import { api } from "@/lib/api";
import { DashboardPage } from "@/pages/Dashboard";
import { DomainsPage } from "@/pages/Domains";
import { InboundsPage } from "@/pages/Inbounds";
import { Login } from "@/pages/Login";
import { SettingsPage } from "@/pages/Settings";
import { UsersPage } from "@/pages/Users";

export default function App() {
  const [authed, setAuthed] = useState<boolean | null>(null);
  const [view, setView] = useState<View>(() => parseView());
  const [coreUp, setCoreUp] = useState<boolean | null>(null);

  useEffect(() => {
    api("/api/me")
      .then(() => setAuthed(true))
      .catch(() => setAuthed(false));
  }, []);

  useEffect(() => {
    const onHash = () => setView(parseView());
    window.addEventListener("hashchange", onHash);
    return () => window.removeEventListener("hashchange", onHash);
  }, []);

  if (authed === null) return null;
  if (!authed) return <Login onOk={() => setAuthed(true)} />;

  return (
    <>
      <Toaster />
      <Shell
        view={view}
        onView={(v) => {
          const next = `#${v}`;
          if (location.hash !== next) location.hash = next;
          else setView(v);
        }}
        coreUp={coreUp}
        onLogout={async () => {
          await api("/api/logout", { method: "POST" });
          setAuthed(false);
        }}
      >
        {view === "dash" && <DashboardPage onCore={setCoreUp} />}
        {view === "users" && <UsersPage />}
        {view === "inbounds" && <InboundsPage />}
        {view === "domains" && <DomainsPage />}
        {view === "settings" && <SettingsPage />}
      </Shell>
    </>
  );
}
