import { useEffect, useState } from "react";
import { toast } from "sonner";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { api } from "@/lib/api";
import type { Domain } from "@/lib/types";

export function DomainsPage() {
  const [rows, setRows] = useState<Domain[]>([]);
  const [open, setOpen] = useState(false);
  const [domain, setDomain] = useState("");
  const [mode, setMode] = useState("direct");
  const [provider, setProvider] = useState("none");

  async function load() {
    setRows(await api<Domain[]>("/api/domains"));
  }
  useEffect(() => {
    load().catch((e) => toast.error(e.message));
  }, []);

  return (
    <div className="grid gap-4">
      <Card>
        <CardHeader className="flex flex-col items-stretch gap-3 sm:flex-row sm:items-start sm:justify-between">
          <div>
            <CardTitle>Domains</CardTitle>
            <CardDescription>
              <b>direct</b> domains are used in Reality/Hysteria/TUIC links. <b>cdn</b> domains are used in WS/gRPC links (orange-cloud
              / Arvan / Gcore in front of origin 443 or 80).
            </CardDescription>
          </div>
          <Button className="w-full sm:w-auto" onClick={() => setOpen(true)}>
            Add domain
          </Button>
        </CardHeader>
        <CardContent className="p-0">
          <div className="grid gap-3 p-3 md:hidden">
            {rows.length === 0 && <p className="text-sm text-muted-foreground">No domains yet — Settings → public host is used instead.</p>}
            {rows.map((d) => (
              <div key={d.id} className="flex items-start justify-between gap-3 rounded-lg border border-border p-3">
                <div className="min-w-0">
                  <div className="break-all font-medium">{d.domain}</div>
                  <div className="mt-1 flex flex-wrap items-center gap-2 text-xs">
                    <Badge variant={d.mode === "cdn" ? "cdn" : "direct"}>{d.mode}</Badge>
                    <span className="text-muted-foreground">{d.provider}</span>
                  </div>
                </div>
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={async () => {
                    await api(`/api/domains/${d.id}`, { method: "DELETE" });
                    await load();
                  }}
                >
                  Delete
                </Button>
              </div>
            ))}
          </div>
          <div className="hidden md:block">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Domain</TableHead>
                <TableHead>Mode</TableHead>
                <TableHead>CDN</TableHead>
                <TableHead />
              </TableRow>
            </TableHeader>
            <TableBody>
              {rows.length === 0 && (
                <TableRow>
                  <TableCell colSpan={4} className="text-muted-foreground">
                    No domains yet — Settings → public host is used instead.
                  </TableCell>
                </TableRow>
              )}
              {rows.map((d) => (
                <TableRow key={d.id}>
                  <TableCell>{d.domain}</TableCell>
                  <TableCell>
                    <Badge variant={d.mode === "cdn" ? "cdn" : "direct"}>{d.mode}</Badge>
                  </TableCell>
                  <TableCell>{d.provider}</TableCell>
                  <TableCell className="text-right">
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={async () => {
                        await api(`/api/domains/${d.id}`, { method: "DELETE" });
                        await load();
                      }}
                    >
                      Delete
                    </Button>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
          </div>
        </CardContent>
      </Card>
      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Add domain</DialogTitle>
          </DialogHeader>
          <div className="grid gap-3">
            <div className="grid gap-1.5">
              <Label>Domain</Label>
              <Input value={domain} onChange={(e) => setDomain(e.target.value)} placeholder="cdn.example.com" />
            </div>
            <div className="grid gap-1.5">
              <Label>Mode</Label>
              <Select value={mode} onValueChange={setMode}>
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="direct">direct</SelectItem>
                  <SelectItem value="cdn">cdn</SelectItem>
                  <SelectItem value="camouflage">camouflage</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="grid gap-1.5">
              <Label>Provider</Label>
              <Select value={provider} onValueChange={setProvider}>
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="none">none</SelectItem>
                  <SelectItem value="cloudflare">cloudflare</SelectItem>
                  <SelectItem value="arvan">arvan</SelectItem>
                  <SelectItem value="gcore">gcore</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>
          <DialogFooter>
            <Button
              onClick={async () => {
                try {
                  await api("/api/domains", { method: "POST", json: { domain, mode, provider } });
                  setOpen(false);
                  setDomain("");
                  await load();
                } catch (e) {
                  toast.error(e instanceof Error ? e.message : "add failed");
                }
              }}
            >
              Save
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
