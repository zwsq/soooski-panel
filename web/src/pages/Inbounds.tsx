import { useEffect, useState } from "react";
import { toast } from "sonner";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Switch } from "@/components/ui/switch";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { api } from "@/lib/api";
import type { Inbound } from "@/lib/types";

export function InboundsPage() {
  const [rows, setRows] = useState<Inbound[]>([]);
  async function load() {
    setRows(await api<Inbound[]>("/api/inbounds"));
  }
  useEffect(() => {
    load().catch((e) => toast.error(e.message));
  }, []);

  return (
    <Card>
      <CardHeader>
        <CardTitle>Inbounds</CardTitle>
        <CardDescription>
          Direct: REALITY / Hysteria2 / TUIC / WireGuard / raw TLS, plus path-muxed transports on 443. CDN: those HTTP transports
          on a Cloudflare / Arvan / Gcore hostname. Toggle each independently.
        </CardDescription>
      </CardHeader>
      <CardContent className="p-0">
        <div className="grid gap-3 p-3 md:hidden">
          {rows.map((r) => (
            <div key={r.id} className="rounded-lg border border-border p-3">
              <div className="flex items-start justify-between gap-3">
                <div className="min-w-0">
                  <code className="break-all text-sm">{r.tag}</code>
                  <div className="text-xs text-muted-foreground">
                    {r.protocol}/{r.transport}/{r.security}
                  </div>
                </div>
                <Switch
                  checked={r.enable}
                  onCheckedChange={async (on) => {
                    try {
                      await api(`/api/inbounds/${r.id}`, { method: "PUT", json: { enable: on } });
                      setRows((prev) => prev.map((x) => (x.id === r.id ? { ...x, enable: on } : x)));
                    } catch (e) {
                      toast.error(e instanceof Error ? e.message : "update failed");
                    }
                  }}
                />
              </div>
              <div className="mt-2 flex flex-wrap items-center gap-2 text-xs">
                <Badge variant={r.mode === "cdn" ? "cdn" : "direct"}>{r.mode}</Badge>
                <span>port {r.listen_port || r.internal_port}</span>
                {r.path && <code className="break-all">{r.path}</code>}
              </div>
              {r.remark && <p className="mt-1 text-xs text-muted-foreground">{r.remark}</p>}
            </div>
          ))}
        </div>
        <div className="hidden md:block">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>On</TableHead>
              <TableHead>Tag</TableHead>
              <TableHead>Mode</TableHead>
              <TableHead>Listen</TableHead>
              <TableHead>Path</TableHead>
              <TableHead>Remark</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {rows.map((r) => (
              <TableRow key={r.id}>
                <TableCell>
                  <Switch
                    checked={r.enable}
                    onCheckedChange={async (on) => {
                      try {
                        await api(`/api/inbounds/${r.id}`, { method: "PUT", json: { enable: on } });
                        setRows((prev) => prev.map((x) => (x.id === r.id ? { ...x, enable: on } : x)));
                      } catch (e) {
                        toast.error(e instanceof Error ? e.message : "update failed");
                      }
                    }}
                  />
                </TableCell>
                <TableCell>
                  <code>{r.tag}</code>
                  <div className="text-xs text-muted-foreground">
                    {r.protocol}/{r.transport}/{r.security}
                  </div>
                </TableCell>
                <TableCell>
                  <Badge variant={r.mode === "cdn" ? "cdn" : "direct"}>{r.mode}</Badge>
                </TableCell>
                <TableCell>{r.listen_port || r.internal_port}</TableCell>
                <TableCell>
                  <code>{r.path || ""}</code>
                </TableCell>
                <TableCell>{r.remark}</TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
        </div>
      </CardContent>
    </Card>
  );
}
