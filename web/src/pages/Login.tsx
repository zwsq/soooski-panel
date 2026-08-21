import { useState, type FormEvent } from "react";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { api } from "@/lib/api";

export function Login({ onOk }: { onOk: () => void }) {
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);

  async function onSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setErr("");
    setBusy(true);
    const fd = new FormData(e.currentTarget);
    try {
      await api("/api/login", {
        method: "POST",
        json: { username: fd.get("username"), password: fd.get("password") },
      });
      onOk();
    } catch (ex) {
      setErr(ex instanceof Error ? ex.message : "login failed");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="grid min-h-screen place-items-center p-4">
      <Card className="w-full max-w-sm">
        <CardContent className="pt-8">
          <form className="flex flex-col gap-4" onSubmit={onSubmit}>
            <img src="./logo.svg" alt="" className="mx-auto size-16 rounded-full bg-black" />
            <div className="text-center">
              <h1 className="text-xl font-semibold tracking-wide">soooski</h1>
              <p className="text-sm text-muted-foreground">Cloud-native multi-protocol panel</p>
            </div>
            <div className="grid gap-1.5">
              <Label htmlFor="username">Username</Label>
              <Input id="username" name="username" autoComplete="username" required />
            </div>
            <div className="grid gap-1.5">
              <Label htmlFor="password">Password</Label>
              <Input id="password" name="password" type="password" autoComplete="current-password" required />
            </div>
            {err && <p className="text-sm text-destructive">{err}</p>}
            <Button type="submit" disabled={busy}>
              Sign in
            </Button>
          </form>
        </CardContent>
      </Card>
    </div>
  );
}
