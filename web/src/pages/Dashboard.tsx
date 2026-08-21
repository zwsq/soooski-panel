import { useEffect, useState } from "react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { api } from "@/lib/api";
import { bytes } from "@/lib/format";
import type { Dashboard } from "@/lib/types";

export function DashboardPage({ onCore }: { onCore: (up: boolean) => void }) {
  const [d, setD] = useState<Dashboard | null>(null);

  async function load() {
    const data = await api<Dashboard>("/api/dashboard");
    setD(data);
    onCore(data.core_running);
  }

  useEffect(() => {
    load().catch((e) => toast.error(e.message));
  }, []);

  if (!d) return <p className="text-muted-foreground">Loading…</p>;

  const stats = [
    { label: "Users", value: `${d.users_active}/${d.users_total}` },
    { label: "Traffic up", value: bytes(d.traffic_up) },
    { label: "Traffic down", value: bytes(d.traffic_down) },
    { label: "Inbounds on", value: String(d.inbounds_on) },
  ];

  return (
    <div className="grid gap-6">
      <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
        {stats.map((s) => (
          <Card key={s.label}>
            <CardHeader>
              <CardDescription>{s.label}</CardDescription>
              <CardTitle className="text-2xl">{s.value}</CardTitle>
            </CardHeader>
            <CardContent />
          </Card>
        ))}
      </div>
      <Card>
        <CardHeader>
          <CardTitle>Host</CardTitle>
          <CardDescription>Public host: {d.public_host || "(set in Settings)"}</CardDescription>
        </CardHeader>
        <CardContent className="grid gap-2 text-sm text-muted-foreground">
          <p>
            HTTPS: if public host is a real domain pointing at this VPS, Let's Encrypt is requested automatically (port 80).
            Cloudflare SSL mode <b className="text-foreground">Full</b> works with self-signed;{" "}
            <b className="text-foreground">Full (strict)</b> needs Let's Encrypt.
          </p>
          <p>
            SQLite + certs live in <code className="text-foreground">{d.data_dir}</code>. Mount that folder to keep users when the
            container is replaced.
          </p>
          {d.core_error && <p className="text-destructive">{d.core_error}</p>}
          <p>
            Admin path (bookmark this): <code className="text-foreground">{d.admin_url}</code>
          </p>
          <p>
            User subscription base: <code className="text-foreground">{d.client_path}/&lt;token&gt;</code>
          </p>
          {d.traffic_error && <p className="text-destructive">Traffic counter: {d.traffic_error}</p>}
        </CardContent>
      </Card>
      <Card>
        <CardHeader>
          <CardTitle>Certificates</CardTitle>
          <CardDescription>
            Let's Encrypt HTTP-01 on port 80. Renews automatically ~30 days before expiry. Issued cert is used for the panel and
            TLS inbounds.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Name</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Issuer</TableHead>
                <TableHead>Expires</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {(d.certs || []).length === 0 && (
                <TableRow>
                  <TableCell colSpan={4} className="text-muted-foreground">
                    No hostnames yet
                  </TableCell>
                </TableRow>
              )}
              {(d.certs || []).map((c) => (
                <TableRow key={c.domain}>
                  <TableCell>
                    <code>{c.domain}</code>
                  </TableCell>
                  <TableCell>
                    <Badge variant={c.state === "issued" ? "ok" : c.state === "failed" ? "bad" : "secondary"}>{c.state}</Badge>
                  </TableCell>
                  <TableCell>{c.issuer || "—"}</TableCell>
                  <TableCell>
                    {c.not_after ? `${c.not_after} (${c.days_left}d)` : "—"}
                    {c.error && <div className="text-destructive">{c.error}</div>}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
          <Button
            className="mt-4"
            onClick={async () => {
              try {
                const r = await api<{ error?: string }>("/api/certs/issue", { method: "POST", json: {} });
                if (r && r.error) toast.error(r.error);
                await load();
              } catch (e) {
                toast.error(e instanceof Error ? e.message : "issue failed");
              }
            }}
          >
            Issue / renew now
          </Button>
        </CardContent>
      </Card>
    </div>
  );
}
