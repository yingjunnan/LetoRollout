import { useState } from "react";
import { motion, useReducedMotion } from "motion/react";
import { api, ApiError } from "../../api/client";
import { useSession, useToasts } from "../../state/store";
import { Button } from "../ui/Button";
import { ReplicaGrid } from "../ui/ReplicaGrid";
import { cardVariants, EASE_OUT } from "../../lib/motion";

export function TokenGate() {
  const [token, setToken] = useState("");
  const [loading, setLoading] = useState(false);
  const setSession = useSession((s) => s.setSession);
  const push = useToasts((s) => s.push);
  const reduce = useReducedMotion();

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!token.trim()) return;
    setLoading(true);
    try {
      const v = await api.verify(token.trim());
      setSession(token.trim(), v);
      push("success", v.isAdmin ? "Logged in as admin" : "Logged in");
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) {
        push("error", "Invalid or expired token");
      } else {
        push("error", `Login failed: ${(err as Error).message}`);
      }
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="relative z-10 flex min-h-screen items-center justify-center px-4 py-10">
      <div className="grid w-full max-w-4xl grid-cols-1 items-center gap-10 md:grid-cols-2">
        {/* Left - brand + signature replica motif (teaches the visual language
            before login: pods light up as they roll out). */}
        <motion.div
          initial={reduce ? false : { opacity: 0, x: -16 }}
          animate={{ opacity: 1, x: 0 }}
          transition={{ duration: 0.5, ease: EASE_OUT }}
          className="hidden md:block"
        >
          <div className="mb-7 flex items-center gap-3">
            <div className="relative flex h-11 w-11 items-center justify-center rounded-xl border border-primary/30 bg-primaryDim shadow-glow">
              <span className="font-mono text-lg font-bold text-primary">L</span>
              <span className="absolute -right-0.5 -top-0.5 h-2 w-2 animate-pulse-ring rounded-full bg-primary" />
            </div>
            <div>
              <h1 className="font-mono text-base font-semibold tracking-tight text-text">
                LetoRollout
              </h1>
              <p className="font-mono text-[10px] uppercase tracking-[0.2em] text-muted">
                rollout console
              </p>
            </div>
          </div>
          <p className="mb-6 max-w-sm text-sm leading-relaxed text-subtext">
            Update container images and watch rollouts unfold - pod by pod -
            across your scoped namespaces.
          </p>
          <div className="card max-w-sm p-5">
            <div className="mb-3 flex items-center justify-between">
              <span className="font-mono text-[10px] uppercase tracking-wider text-muted">
                replica readiness
              </span>
              <span className="font-mono text-[10px] text-subtext">3 / 5</span>
            </div>
            <ReplicaGrid replicas={5} ready={3} size="sm" />
          </div>
        </motion.div>

        {/* Right - auth card */}
        <motion.div
          variants={cardVariants}
          initial={reduce ? false : "hidden"}
          animate="show"
        >
          <form onSubmit={submit} className="card flex flex-col gap-4 p-6">
            <div className="mb-1 flex items-center gap-3 md:hidden">
              <div className="relative flex h-10 w-10 items-center justify-center rounded-lg border border-primary/30 bg-primaryDim shadow-glow">
                <span className="font-mono text-base font-bold text-primary">
                  L
                </span>
                <span className="absolute -right-0.5 -top-0.5 h-2 w-2 animate-pulse-ring rounded-full bg-primary" />
              </div>
              <div>
                <h1 className="font-mono text-sm font-semibold tracking-tight text-text">
                  LetoRollout
                </h1>
                <p className="font-mono text-[10px] uppercase tracking-[0.2em] text-muted">
                  rollout console
                </p>
              </div>
            </div>
            <div>
              <span className="mb-1.5 block font-mono text-[10px] uppercase tracking-wider text-muted">
                Access token
              </span>
              <p className="font-mono text-[11px] text-subtext">
                Enter the bearer token issued to you.
              </p>
            </div>
            <input
              type="password"
              value={token}
              onChange={(e) => setToken(e.target.value)}
              placeholder="letorollout-…"
              autoFocus
              className="w-full rounded-lg border border-border bg-surface px-3 py-2.5 font-mono text-sm text-text
                placeholder:text-muted/70 transition-all duration-150
                hover:border-borderHi focus:border-primary focus:outline-none focus:ring-2 focus:ring-primary/20"
            />
            <Button type="submit" variant="primary" loading={loading} className="w-full">
              Authenticate -&gt;
            </Button>
          </form>
          <p className="mt-4 text-center font-mono text-[10px] text-muted">
            scoped tokens · namespace-bound · expiring
          </p>
        </motion.div>
      </div>
    </div>
  );
}
