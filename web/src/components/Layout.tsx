import { LayoutDashboard, Users, Waypoints, Globe, Settings, LogOut } from "lucide-react";
import type { ReactNode } from "react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";

export const VIEWS = ["dash", "users", "inbounds", "domains", "settings"] as const;
export type View = (typeof VIEWS)[number];

const items: { id: View; label: string; icon: typeof LayoutDashboard }[] = [
  { id: "dash", label: "Dashboard", icon: LayoutDashboard },
  { id: "users", label: "Users", icon: Users },
  { id: "inbounds", label: "Inbounds", icon: Waypoints },
  { id: "domains", label: "Domains", icon: Globe },
  { id: "settings", label: "Settings", icon: Settings },
];

export function parseView(hash = location.hash): View {
  const v = hash.replace(/^#\/?/, "");
  return (VIEWS as readonly string[]).includes(v) ? (v as View) : "dash";
}

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
  const navBtn = (it: (typeof items)[number], compact: boolean) => {
    const Icon = it.icon;
    const active = view === it.id;
    return (
      <button
        key={it.id}
        type="button"
        onClick={() => onView(it.id)}
        className={cn(
          "flex items-center transition-colors",
          compact
            ? "min-h-12 flex-col justify-center gap-0.5 px-1 text-[11px]"
            : "gap-2 rounded-lg px-3 py-2 text-sm text-left",
          active ? "text-primary" : "text-muted-foreground hover:text-foreground",
          !compact && (active ? "bg-primary/15" : "hover:bg-secondary"),
        )}
      >
        <Icon className={compact ? "size-5" : "size-4"} />
        {it.label}
      </button>
    );
  };

  return (
    <div className="min-h-dvh md:grid md:grid-cols-[240px_1fr]">
      <aside className="sticky top-0 hidden h-dvh border-r border-border bg-card/40 p-4 md:flex md:flex-col">
        <div className="mb-6 flex items-center gap-2 px-2 pt-2">
          <img src="./logo.svg" alt="" className="size-9 rounded-full bg-black" />
          <span className="text-lg font-semibold tracking-wide text-primary">soooski</span>
        </div>
        <nav className="flex flex-col gap-1">{items.map((it) => navBtn(it, false))}</nav>
      </aside>
      <div className="flex min-h-dvh flex-col">
        <header className="sticky top-0 z-20 flex items-center gap-3 border-b border-border bg-background/90 px-4 py-3 pt-[max(0.75rem,env(safe-area-inset-top))] backdrop-blur">
          <img src="./logo.svg" alt="" className="size-8 rounded-full bg-black md:hidden" />
          <h1 className="text-lg font-semibold capitalize">{items.find((i) => i.id === view)?.label}</h1>
          {coreUp !== null && (
            <Badge variant={coreUp ? "ok" : "bad"}>{coreUp ? "core up" : "core down"}</Badge>
          )}
          <Button variant="outline" size="sm" className="ml-auto" onClick={onLogout}>
            <LogOut />
            <span className="hidden sm:inline">Log out</span>
          </Button>
        </header>
        <main className="flex-1 p-4 pb-[calc(5.5rem+env(safe-area-inset-bottom))] md:p-8 md:pb-8">{children}</main>
        <nav className="fixed inset-x-0 bottom-0 z-30 grid grid-cols-5 border-t border-border bg-background/95 pb-[env(safe-area-inset-bottom)] md:hidden">
          {items.map((it) => navBtn(it, true))}
        </nav>
      </div>
    </div>
  );
}
