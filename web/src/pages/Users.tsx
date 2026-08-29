import { useEffect, useState } from "react";
import { Copy, ExternalLink, Send } from "lucide-react";
import { toast } from "sonner";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { DatePicker } from "@/components/ui/date-picker";
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Progress } from "@/components/ui/progress";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { api } from "@/lib/api";
import { bytes, lastSeenLabel, remainingDays, trafficLimitFrom, trafficParts, trafficProgress, userStatus, ymd } from "@/lib/format";
import type { Link, User } from "@/lib/types";
import { cn } from "@/lib/utils";

function CopyBtn({ text, label = "Copy" }: { text: string; label?: string }) {
  return (
    <Button
      type="button"
      variant="outline"
      size="sm"
      onClick={async () => {
        try {
          await navigator.clipboard.writeText(text);
          toast.success("Copied");
        } catch {
          toast.error(text);
        }
      }}
    >
      <Copy />
      {label}
    </Button>
  );
}

function TrafficFields({
  id,
  value,
  unit,
  onValue,
  onUnit,
}: {
  id: string;
  value: string;
  unit: string;
  onValue: (v: string) => void;
  onUnit: (u: string) => void;
}) {
  return (
    <div className="flex gap-2">
      <Input
        id={id}
        type="number"
        min={0}
        step="0.1"
        className="min-w-0"
        value={unit === "unlimited" ? "" : value}
        disabled={unit === "unlimited"}
        onChange={(e) => onValue(e.target.value)}
      />
      <Select
        value={unit}
        onValueChange={(v) => {
          onUnit(v);
          if (v === "unlimited") onValue("");
        }}
      >
        <SelectTrigger className="w-32 shrink-0">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="GB">GB</SelectItem>
          <SelectItem value="MB">MB</SelectItem>
          <SelectItem value="TB">TB</SelectItem>
          <SelectItem value="unlimited">unlimited</SelectItem>
        </SelectContent>
      </Select>
    </div>
  );
}

function Connection({ u }: { u: User }) {
  const seen = lastSeenLabel(u);
  return (
    <span
      className={cn(
        "inline-flex items-center gap-1.5 text-sm tabular-nums",
        seen.kind === "online" && "font-medium text-emerald-400",
        seen.kind === "never" && "text-muted-foreground",
      )}
    >
      <span className="relative flex size-2">
        {seen.kind === "online" && <span className="absolute inset-0 animate-ping rounded-full bg-emerald-400/70" />}
        <span
          className={cn(
            "relative size-2 rounded-full",
            seen.kind === "online" ? "bg-emerald-400" : seen.kind === "ago" ? "bg-muted-foreground/70" : "bg-muted-foreground/35",
          )}
        />
      </span>
      {seen.text}
    </span>
  );
}

function Expire({ u }: { u: User }) {
  const left = remainingDays(u.expire_at);
  if (left == null) return <span className="text-muted-foreground">No expiry</span>;
  if (left < 0) return <span className="tabular-nums text-muted-foreground">Expired</span>;
  return <span className={cn("tabular-nums", left < 10 && "text-orange-400")}>{left}d left</span>;
}

function Traffic({ u }: { u: User }) {
  const tr = trafficProgress(u.traffic_up, u.traffic_down, u.traffic_limit);
  if (tr.pct == null) return <span>{bytes(tr.used)} / unlimited</span>;
  return (
    <div className="grid min-w-0 gap-1">
      <span className="text-xs tabular-nums">
        {bytes(tr.used)} / {bytes(tr.limit)}
      </span>
      <Progress value={tr.pct} />
    </div>
  );
}

export function UsersPage() {
  const [users, setUsers] = useState<User[]>([]);
  const [q, setQ] = useState("");
  const [filter, setFilter] = useState<"all" | "online" | "expiring">("all");
  const [addOpen, setAddOpen] = useState(false);
  const [edit, setEdit] = useState<User | null>(null);
  const [links, setLinks] = useState<{ user: User; sub: string; clash: string; sing_box: string; links: Link[] } | null>(null);
  const [addName, setAddName] = useState("");
  const [addNote, setAddNote] = useState("");
  const [addExp, setAddExp] = useState("");
  const [addLim, setAddLim] = useState("");
  const [addUnit, setAddUnit] = useState("unlimited");
  const [editName, setEditName] = useState("");
  const [editNote, setEditNote] = useState("");
  const [editEn, setEditEn] = useState("1");
  const [editExp, setEditExp] = useState("");
  const [editLim, setEditLim] = useState("");
  const [editUnit, setEditUnit] = useState("unlimited");

  async function load() {
    setUsers(await api<User[]>("/api/users"));
  }
  useEffect(() => {
    load().catch((e) => toast.error(e.message));
    const t = setInterval(() => load().catch(() => {}), 8_000);
    return () => clearInterval(t);
  }, []);

  const shown = users.filter((u) => {
    const s = q.trim().toLowerCase();
    if (s && !u.username.toLowerCase().includes(s) && !(u.note || "").toLowerCase().includes(s)) return false;
    if (filter === "online") return !!u.online;
    if (filter === "expiring") {
      const left = remainingDays(u.expire_at);
      return left != null && left >= 0 && left < 10;
    }
    return true;
  });

  function openEdit(u: User) {
    const p = trafficParts(u.traffic_limit);
    setEdit(u);
    setEditName(u.username);
    setEditNote(u.note || "");
    setEditEn(u.enable || userStatus(u).t === "quota" ? "1" : "0");
    setEditExp(ymd(u.expire_at));
    setEditLim(p.unit === "unlimited" ? "" : String(p.value));
    setEditUnit(p.unit);
  }

  async function openPage(u: User) {
    try {
      const data = await api<{ sub: string }>(`/api/users/${u.id}/links`);
      window.open(`${location.origin}${data.sub}`, "_blank", "noopener");
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "open failed");
    }
  }

  async function openLinks(u: User) {
    try {
      const data = await api<{ user: User; sub: string; clash: string; sing_box: string; links: Link[] }>(`/api/users/${u.id}/links`);
      setLinks(data);
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "links");
    }
  }

  async function removeUser(u: User) {
    if (!confirm("Delete user?")) return;
    await api(`/api/users/${u.id}`, { method: "DELETE" });
    await load();
  }

  const actions = (u: User) => (
    <div className="flex flex-wrap gap-1">
      <Button variant="outline" size="sm" onClick={() => openPage(u)}>
        <ExternalLink />
        Open
      </Button>
      <Button variant="outline" size="sm" onClick={() => openLinks(u)}>
        Links
      </Button>
      <Button variant="outline" size="sm" onClick={() => openEdit(u)}>
        Edit
      </Button>
      <Button variant="ghost" size="sm" onClick={() => removeUser(u)}>
        Delete
      </Button>
    </div>
  );

  return (
    <div className="grid gap-4">
      <div className="flex flex-col gap-2 sm:flex-row sm:flex-wrap sm:items-center">
        <Button
          className="h-11 sm:h-9"
          onClick={() => {
            setAddName("");
            setAddNote("");
            setAddExp("");
            setAddLim("");
            setAddUnit("unlimited");
            setAddOpen(true);
          }}
        >
          Add user
        </Button>
        <Input className="h-11 sm:h-9 sm:max-w-xs" placeholder="Search users…" value={q} onChange={(e) => setQ(e.target.value)} />
        <div className="flex gap-1">
          {(
            [
              ["all", "All"],
              ["online", "Online"],
              ["expiring", "Expiring"],
            ] as const
          ).map(([id, label]) => (
            <Button key={id} type="button" size="sm" variant={filter === id ? "default" : "outline"} onClick={() => setFilter(id)}>
              {label}
            </Button>
          ))}
        </div>
      </div>

      <div className="grid gap-3 md:hidden">
        {shown.length === 0 && <p className="text-sm text-muted-foreground">No users match.</p>}
        {shown.map((u) => {
          const st = userStatus(u);
          return (
            <Card key={u.id}>
              <CardContent className="grid gap-3 p-4">
                <div className="flex items-start justify-between gap-3">
                  <div className="min-w-0">
                    <div className="flex items-center gap-2 font-medium">
                      {u.username}
                      {u.telegram_secret && <Send className="size-3.5 shrink-0 text-[#2AABEE]" aria-label="Telegram proxy" />}
                    </div>
                    {u.note && <div className="text-xs text-muted-foreground">{u.note}</div>}
                  </div>
                  <Badge variant={st.cls === "ok" ? "ok" : "bad"}>{st.t}</Badge>
                </div>
                <div className="flex flex-wrap items-center justify-between gap-2 text-sm">
                  <Connection u={u} />
                  <Expire u={u} />
                </div>
                <Traffic u={u} />
                {actions(u)}
              </CardContent>
            </Card>
          );
        })}
      </div>

      <Card className="hidden md:block">
        <CardContent className="p-0">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>User</TableHead>
                <TableHead>Connection</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Expire</TableHead>
                <TableHead>Traffic</TableHead>
                <TableHead />
              </TableRow>
            </TableHeader>
            <TableBody>
              {shown.map((u) => {
                const st = userStatus(u);
                return (
                  <TableRow key={u.id}>
                    <TableCell>
                      <div className="flex items-center gap-2 font-medium">
                        {u.username}
                        {u.telegram_secret && <Send className="size-3.5 text-[#2AABEE]" aria-label="Telegram proxy" />}
                      </div>
                      {u.note && <div className="text-xs text-muted-foreground">{u.note}</div>}
                    </TableCell>
                    <TableCell>
                      <Connection u={u} />
                    </TableCell>
                    <TableCell>
                      <Badge variant={st.cls === "ok" ? "ok" : "bad"}>{st.t}</Badge>
                    </TableCell>
                    <TableCell>
                      <Expire u={u} />
                    </TableCell>
                    <TableCell className="min-w-44">
                      <Traffic u={u} />
                    </TableCell>
                    <TableCell className="text-right">{actions(u)}</TableCell>
                  </TableRow>
                );
              })}
            </TableBody>
          </Table>
        </CardContent>
      </Card>

      <Dialog open={addOpen} onOpenChange={setAddOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>New user</DialogTitle>
          </DialogHeader>
          <div className="grid gap-3">
            <div className="grid gap-1.5">
              <Label>Username</Label>
              <Input value={addName} onChange={(e) => setAddName(e.target.value)} required />
            </div>
            <div className="grid gap-1.5">
              <Label>Note</Label>
              <Input value={addNote} onChange={(e) => setAddNote(e.target.value)} />
            </div>
            <div className="grid gap-1.5">
              <Label>Traffic limit</Label>
              <TrafficFields id="add-lim" value={addLim} unit={addUnit} onValue={setAddLim} onUnit={setAddUnit} />
            </div>
            <div className="grid gap-1.5">
              <Label>Expire (end of that UTC day)</Label>
              <DatePicker value={addExp} onChange={setAddExp} />
            </div>
          </div>
          <DialogFooter>
            <Button
              className="w-full sm:w-auto"
              onClick={async () => {
                try {
                  await api("/api/users", {
                    method: "POST",
                    json: {
                      username: addName,
                      note: addNote,
                      traffic_limit: trafficLimitFrom(addLim, addUnit),
                      expire_at: addExp,
                    },
                  });
                  setAddOpen(false);
                  await load();
                } catch (e) {
                  toast.error(e instanceof Error ? e.message : "create failed");
                }
              }}
            >
              Save
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={!!edit} onOpenChange={(o) => !o && setEdit(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Edit {edit?.username}</DialogTitle>
          </DialogHeader>
          {edit && (
            <div className="grid gap-3">
              <div className="grid gap-1.5">
                <Label>Username</Label>
                <Input value={editName} onChange={(e) => setEditName(e.target.value)} />
              </div>
              <div className="grid gap-1.5">
                <Label>Note</Label>
                <Input value={editNote} onChange={(e) => setEditNote(e.target.value)} />
              </div>
              <div className="grid gap-1.5">
                <Label>Enabled</Label>
                <Select value={editEn} onValueChange={setEditEn}>
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="1">on</SelectItem>
                    <SelectItem value="0">off</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div className="grid gap-1.5">
                <Label>Traffic limit</Label>
                <TrafficFields id="edit-lim" value={editLim} unit={editUnit} onValue={setEditLim} onUnit={setEditUnit} />
              </div>
              <div className="grid gap-1.5">
                <Label>Expire (end of that UTC day)</Label>
                <DatePicker value={editExp} onChange={setEditExp} />
              </div>
              {edit.telegram_secret && (
                <div className="rounded-lg border border-[#2AABEE]/40 bg-[#2AABEE]/10 p-3 text-sm">
                  <div className="mb-1 font-medium text-[#2AABEE]">Telegram secret</div>
                  <code className="block break-all text-xs">{edit.telegram_secret}</code>
                  <Button
                    variant="outline"
                    size="sm"
                    className="mt-2"
                    onClick={async () => {
                      if (!confirm("New Telegram secret for this user. Their old tg:// link stops working.")) return;
                      try {
                        await api(`/api/users/${edit.id}`, { method: "PUT", json: { telegram_regenerate: true } });
                        setEdit(null);
                        await load();
                      } catch (e) {
                        toast.error(e instanceof Error ? e.message : "regen failed");
                      }
                    }}
                  >
                    Regenerate Telegram secret
                  </Button>
                </div>
              )}
            </div>
          )}
          <DialogFooter>
            <Button
              className="w-full sm:w-auto"
              onClick={async () => {
                if (!edit) return;
                try {
                  await api(`/api/users/${edit.id}`, {
                    method: "PUT",
                    json: {
                      username: editName,
                      note: editNote,
                      enable: editEn === "1",
                      traffic_limit: trafficLimitFrom(editLim, editUnit),
                      expire_at: editExp,
                    },
                  });
                  setEdit(null);
                  await load();
                } catch (e) {
                  toast.error(e instanceof Error ? e.message : "save failed");
                }
              }}
            >
              Save
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={!!links} onOpenChange={(o) => !o && setLinks(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Subscription</DialogTitle>
          </DialogHeader>
          {links && (
            <div className="grid gap-3 text-sm">
              <p className="text-muted-foreground">Import this URL in Hiddify / v2rayNG / Clash Meta / sing-box:</p>
              <div className="flex flex-wrap items-start gap-2">
                <code className="min-w-0 flex-1 break-all rounded-md bg-background p-2">{location.origin + links.sub}</code>
                <CopyBtn text={location.origin + links.sub} />
                <Button asChild variant="outline" size="sm">
                  <a href={location.origin + links.sub} target="_blank" rel="noopener">
                    <ExternalLink />
                    Open page
                  </a>
                </Button>
              </div>
              <div className="flex flex-wrap items-start gap-2">
                <code className="min-w-0 flex-1 break-all rounded-md bg-background p-2">{location.origin + links.clash}</code>
                <CopyBtn text={location.origin + links.clash} label="Clash" />
              </div>
              <div className="flex flex-wrap items-start gap-2">
                <code className="min-w-0 flex-1 break-all rounded-md bg-background p-2">{location.origin + links.sing_box}</code>
                <CopyBtn text={location.origin + links.sing_box} label="sing-box" />
              </div>
              <p className="font-medium">Share links</p>
              {links.links.map((l) => (
                <div key={l.tag + l.uri} className="grid gap-1">
                  <div>
                    <Badge variant={l.protocol === "mtproto" ? "default" : l.mode === "cdn" ? "cdn" : "direct"}>
                      {l.protocol === "mtproto" ? "telegram" : l.mode}
                    </Badge>{" "}
                    {l.tag}
                  </div>
                  <code className="block break-all rounded-md bg-background p-2 text-xs">{l.uri}</code>
                  <div>
                    <CopyBtn text={l.uri} />
                  </div>
                </div>
              ))}
            </div>
          )}
        </DialogContent>
      </Dialog>
    </div>
  );
}
