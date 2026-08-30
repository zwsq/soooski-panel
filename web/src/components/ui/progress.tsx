import { cn } from "@/lib/utils";

export function Progress({ value = 0, className }: { value?: number; className?: string }) {
  const pct = Math.min(100, Math.max(0, value));
  return (
    <div
      role="progressbar"
      aria-valuemin={0}
      aria-valuemax={100}
      aria-valuenow={Math.round(pct)}
      className={cn("relative h-2.5 w-full overflow-hidden rounded-full bg-white/25", className)}
    >
      <div className="h-full rounded-full bg-cyan-400 transition-all" style={{ width: `${pct}%` }} />
    </div>
  );
}
