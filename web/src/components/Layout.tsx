import { LayoutDashboard, Users, Waypoints, Globe, Settings, LogOut, Menu, X } from "lucide-react";
import { useState, type ReactNode } from "react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";

const items = [
  { id: "dash", label: "Dashboard", icon: LayoutDashboard },
  { id: "users", label: "Users", icon: Users },
  { id: "inbounds", label: "Inbounds", icon: Waypoints },
  { id: "domains", label: "Domains", icon: Globe },
  { id: "settings", label: "Settings", icon: Settings },
] as const;

export type View = (typeof items)[number]["id"];

export function Shell({
  view,
  onView,
  coreUp,
  onLogout,
  children,
}: {
  view: View;
  onView: (v: View) => void;
  coreUp: boolean | null;
  onLogout: () => void;
  children: ReactNode;
}) {
  const [open, setOpen] = useState(false);
  const nav = (
    <nav className="flex flex-1 flex-col gap-1">
      {items.map((it) => {
        const Icon = it.icon;
        const active = view === it.id;
        return (
          <button
            key={it.id}
            onClick={() => {
              onView(it.id);
              setOpen(false);
            }}
            className={cn(
              "flex items-center gap-2 rounded-lg px-3 py-2 text-sm text-left transition-colors",
              active ? "bg-primary/15 text-primary" : "text-muted-foreground hover:bg-secondary hover:text-foreground",
            )}
          >
            <Icon className="size-4" />
            {it.label}
          </button>
        );
      })}
    </nav>
  );
  return (
    <div className="min-h-screen md:grid md:grid-cols-[240px_1fr]">
      <aside className="hidden border-r border-border bg-card/40 p-4 md:flex md:flex-col">
        <div className="mb-6 flex items-center gap-2 px-2 pt-2">
          <img src="./logo.svg" alt="" className="size-9 rounded-full bg-black" />
          <span className="text-lg font-semibold tracking-wide text-primary">soooski</span>
        </div>
        {nav}
        <Button variant="ghost" className="mt-auto justify-start" onClick={onLogout}>
          <LogOut /> Log out
        </Button>
      </aside>
      <div className="flex min-h-screen flex-col">
        <header className="flex items-center gap-3 border-b border-border px-4 py-3">
          <Button variant="ghost" size="icon" className="md:hidden" onClick={() => setOpen((v) => !v)}>
            {open ? <X /> : <Menu />}
          </Button>
          <h1 className="text-lg font-semibold capitalize">{items.find((i) => i.id === view)?.label}</h1>
          {coreUp !== null && (
            <Badge variant={coreUp ? "ok" : "bad"}>{coreUp ? "core up" : "core down"}</Badge>
          )}
        </header>
        {open && (
          <div className="border-b border-border p-3 md:hidden">
            {nav}
            <Button variant="ghost" className="mt-2 w-full justify-start" onClick={onLogout}>
              <LogOut /> Log out
            </Button>
          </div>
        )}
        <main className="flex-1 p-4 md:p-8">{children}</main>
      </div>
    </div>
  );
}
