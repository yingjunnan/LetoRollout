import { motion, useReducedMotion } from "motion/react";
import { EASE_OUT } from "../../lib/motion";

interface ReplicaGridProps {
  replicas: number;
  ready: number;
  size?: "sm" | "lg";
}

// The signature element of the console. Each replica is a pod tile: ready pods
// glow teal, the rest pulse amber as in-progress. On mount the tiles pop in
// with a stagger so a freshly-selected deployment reads as "coming online".
//
// Beyond 24 replicas a grid gets noisy, so it collapses to a compact bar plus
// a count - the same information, legible at scale.
export function ReplicaGrid({ replicas, ready, size = "lg" }: ReplicaGridProps) {
  const reduce = useReducedMotion();
  const total = Math.max(replicas, 0);
  const readyCount = Math.min(ready, total);

  const tile = size === "lg" ? "h-7 w-7" : "h-4 w-4";
  const radius = size === "lg" ? "rounded-md" : "rounded-sm";

  if (total === 0) {
    return <div className="font-mono text-xs text-muted">0 replicas</div>;
  }

  // Compact bar for high-replica deployments.
  if (total > 24) {
    const pct = (readyCount / total) * 100;
    return (
      <div className="flex flex-col gap-2.5">
        <div className="flex items-baseline gap-2">
          <span className="font-mono text-2xl font-semibold text-text">
            {readyCount}
            <span className="text-muted">/{total}</span>
          </span>
          <span className="font-mono text-[10px] uppercase tracking-wider text-muted">
            replicas ready
          </span>
        </div>
        <div className="h-2 w-full overflow-hidden rounded-full bg-border/60">
          <motion.div
            className="h-full rounded-full bg-primary"
            initial={reduce ? false : { width: 0 }}
            animate={{ width: `${pct}%` }}
            transition={{ duration: 0.6, ease: EASE_OUT }}
            style={{ boxShadow: "0 0 12px rgba(45,212,191,0.6)" }}
          />
        </div>
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-3">
      <div className="flex flex-wrap gap-1.5">
        {Array.from({ length: total }).map((_, i) => {
          const isReady = i < readyCount;
          return (
            <motion.span
              key={i}
              className={`${tile} ${radius} relative overflow-hidden border ${
                isReady
                  ? "border-primary/50 bg-primary/20"
                  : "border-amber/40 bg-amber/5"
              }`}
              initial={reduce ? false : { opacity: 0, scale: 0.6 }}
              animate={{ opacity: 1, scale: 1 }}
              transition={{
                duration: 0.3,
                ease: EASE_OUT,
                delay: reduce ? 0 : i * 0.035,
              }}
            >
              {isReady && (
                <span
                  className="absolute inset-0 rounded-[inherit] bg-primary/25"
                  style={{ boxShadow: "inset 0 0 8px rgba(45,212,191,0.55)" }}
                />
              )}
              {!isReady && !reduce && (
                <motion.span
                  className="absolute inset-0 rounded-[inherit] bg-amber/20"
                  animate={{ opacity: [0.15, 0.5, 0.15] }}
                  transition={{ duration: 1.6, repeat: Infinity, ease: "easeInOut" }}
                />
              )}
            </motion.span>
          );
        })}
      </div>
      <div className="flex items-center gap-2 font-mono text-[10px] uppercase tracking-wider text-muted">
        <span className="text-primary">{readyCount} ready</span>
        <span className="text-borderHi">·</span>
        <span className="text-amber">{total - readyCount} pending</span>
      </div>
    </div>
  );
}
