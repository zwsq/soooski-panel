import { useEffect, useState } from "react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { api } from "@/lib/api";
import type { Settings } from "@/lib/types";

export function SettingsPage() {
  const [s, setS] = useState<Settings | null>(null);
  const [me, setMe] = useState("");
  const [cur, setCur] = useState("");
  const [pw, setPw] = useState("");
  const [pw2, setPw2] = useState("");

  async function load() {
    const [settings, who] = await Promise.all([api<Settings>("/api/settings"), api<{ username: string }>("/api/me")]);
    setS(settings);
    setMe(who.username);
  }
  useEffect(() => {
    load().catch((e) => toast.error(e.message));
  }, []);

  if (!s) return <p className="text-muted-foreground">Loading…</p>;

  async function saveSettings(extra: Record<string, string> = {}) {
    if (!s) return;
    const prev = s.admin_path || "";
    const data = await api<Settings>("/api/settings", {
      method: "PUT",
      json: {
        public_host: s.public_host,
        acme_email: s.acme_email,
        reality_server_name: s.reality_server_name,
        hy2_obfs: s.hy2_obfs,
        admin_path: s.admin_path,
        telegram_enabled: s.telegram_enabled ? "1" : "0",
        telegram_fake_domain: s.telegram_fake_domain,
        ...extra,
      },
    });
    const next = data.admin_path || prev;
    if (next && next !== prev) {
      toast.message("Admin path changed. Bookmark the new URL.");
      location.href = location.origin + "/" + String(next).replace(/^\/+|\/+$/g, "") + "/";
      return;
    }
    setS(data);
    toast.success("Settings saved");
  }

  return (
    <div className="grid max-w-3xl gap-6">
      <Card className="border-[#2AABEE]/40">
        <CardHeader>
          <CardTitle className="text-[#2AABEE]">Telegram proxy</CardTitle>
          <CardDescription>
            FakeTLS MTProto on 443. Each user has their own secret — bytes count toward that user's quota. Do not use your panel
            domain or the REALITY handshake dest as the fake website. Share links are on each user page, at the top.
          </CardDescription>
        </CardHeader>
        <CardContent className="grid gap-4">
          <label className="flex items-center gap-3 text-sm">
            <Switch checked={s.telegram_enabled} onCheckedChange={(on) => setS({ ...s, telegram_enabled: on })} />
            Enable Telegram MTProto
          </label>
          <div className="grid gap-1.5">
            <Label>Fake TLS domain</Label>
            <Input
              value={s.telegram_fake_domain}
              onChange={(e) => setS({ ...s, telegram_fake_domain: e.target.value })}
              placeholder="www.cloudflare.com"
            />
          </div>
          <div className="flex flex-wrap gap-2">
            <Button onClick={() => saveSettings().catch((e) => toast.error(e.message))}>Save Telegram</Button>
            <Button
              variant="outline"
              onClick={async () => {
                if (!confirm("This issues a new Telegram secret for every user. Existing tg:// links stop working.")) return;
                try {
                  await saveSettings({ telegram_regenerate: "1" });
                } catch (e) {
                  toast.error(e instanceof Error ? e.message : "failed");
                }
              }}
            >
              Regenerate all user secrets
            </Button>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Panel</CardTitle>
        </CardHeader>
        <CardContent className="grid gap-3">
          <div className="grid gap-1.5">
            <Label>Public host / IP used in share links</Label>
            <Input value={s.public_host} onChange={(e) => setS({ ...s, public_host: e.target.value })} />
          </div>
          <div className="grid gap-1.5">
            <Label>Let's Encrypt email</Label>
            <Input type="email" value={s.acme_email} onChange={(e) => setS({ ...s, acme_email: e.target.value })} />
          </div>
          <div className="grid gap-1.5">
            <Label>REALITY handshake dest</Label>
            <Input
              value={s.reality_server_name}
              onChange={(e) => setS({ ...s, reality_server_name: e.target.value })}
              placeholder="gateway.icloud.com"
            />
            <p className="text-xs text-muted-foreground">
              A real HTTPS site whose certificate chain fits in REALITY&apos;s 8KB buffer. Do not use{" "}
              <code>www.microsoft.com</code> (too large) or the Telegram fake domain. If the VPS cannot
              reach this host, try <code>www.samsung.com</code>. Changing dest changes share-link SNI —
              users must re-import REALITY.
            </p>
          </div>
          <div className="grid gap-1.5">
            <Label>Hysteria2 obfs password (empty = off)</Label>
            <Input value={s.hy2_obfs} onChange={(e) => setS({ ...s, hy2_obfs: e.target.value })} />
          </div>
          <div className="grid gap-1.5">
            <Label>Admin secret path</Label>
            <Input value={s.admin_path} onChange={(e) => setS({ ...s, admin_path: e.target.value })} autoComplete="off" />
          </div>
          <p className="text-sm text-muted-foreground">
            Changing the admin path logs you into the new URL. Client path stays <code>{s.client_path}</code>.
          </p>
          <p className="text-xs text-muted-foreground">
            REALITY public key: <code>{s.reality_public_key}</code>
            <br />
            WireGuard server public key: <code>{s.wg_public_key}</code>
          </p>
          <Button onClick={() => saveSettings().catch((e) => toast.error(e.message))}>Save & reload core</Button>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Admin account</CardTitle>
          <CardDescription>
            Env SOOOSKI_ADMIN_USER / SOOOSKI_ADMIN_PASSWORD apply only on first boot. After that, change them here — or{" "}
            <code>soooski reset-admin</code> if you are locked out.
          </CardDescription>
        </CardHeader>
        <CardContent className="grid gap-3">
          <div className="grid gap-1.5">
            <Label>Username</Label>
            <Input value={me} onChange={(e) => setMe(e.target.value)} autoComplete="username" />
          </div>
          <div className="grid gap-1.5">
            <Label>Current password</Label>
            <Input type="password" value={cur} onChange={(e) => setCur(e.target.value)} autoComplete="current-password" />
          </div>
          <div className="grid gap-1.5">
            <Label>New password (leave blank to keep)</Label>
            <Input type="password" value={pw} onChange={(e) => setPw(e.target.value)} autoComplete="new-password" />
          </div>
          <div className="grid gap-1.5">
            <Label>Confirm new password</Label>
            <Input type="password" value={pw2} onChange={(e) => setPw2(e.target.value)} autoComplete="new-password" />
          </div>
          <Button
            onClick={async () => {
              if (pw && pw !== pw2) {
                toast.error("Passwords do not match");
                return;
              }
              try {
                await api("/api/admin", {
                  method: "PUT",
                  json: { current_password: cur, username: me, password: pw, password_confirm: pw2 },
                });
                setCur("");
                setPw("");
                setPw2("");
                toast.success("Admin account updated");
                await load();
              } catch (e) {
                toast.error(e instanceof Error ? e.message : "failed");
              }
            }}
          >
            Save username / password
          </Button>
        </CardContent>
      </Card>
    </div>
  );
}
