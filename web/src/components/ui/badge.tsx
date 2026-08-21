import * as React from "react";
import { cva, type VariantProps } from "class-variance-authority";
import { cn } from "@/lib/utils";

const badgeVariants = cva(
  "inline-flex items-center rounded-full border px-2 py-0.5 text-xs font-medium",
  {
    variants: {
      variant: {
        default: "border-transparent bg-primary/15 text-primary",
        secondary: "border-border text-muted-foreground",
        ok: "border-emerald-500/40 text-emerald-400",
        bad: "border-rose-500/40 text-rose-400",
        cdn: "border-sky-500/40 text-sky-400",
        direct: "border-teal-500/40 text-teal-300",
        outline: "text-foreground",
      },
    },
    defaultVariants: { variant: "secondary" },
  },
);

export function Badge({ className, variant, ...props }: React.ComponentProps<"span"> & VariantProps<typeof badgeVariants>) {
  return <span className={cn(badgeVariants({ variant }), className)} {...props} />;
}
