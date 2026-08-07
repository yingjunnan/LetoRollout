import { useState } from "react";
import { AnimatePresence, motion, useReducedMotion } from "motion/react";
import { useSession } from "../../state/store";
import { Button } from "../ui/Button";
import { useDeployments } from "../../hooks/useDeployments";
import type { DeploymentSummary } from "../../api/types";
import { ImageUpdateForm } from "../deploy/ImageUpdateForm";
import { LogViewer } from "../logs/LogViewer";
import { AdminPanel } from "../admin/AdminPanel";
import { ReplicaGrid } from "../ui/ReplicaGrid";
import { cardVariants, staggerContainer, EASE_OUT } from "../../lib/motion";

type ReadyState = "ready" | "rolling" | "pending";

function readyState(ready: number, total: number): ReadyState {
  if (total > 0 && ready >= total) return "ready";
  if (ready === 0) return "pending";
  return "rolling";
}

function StateBadge({ state }: { state: ReadyState }) {
  const map = {
    ready: {
      dot: "bg-success",
      text: "text-success",
      border: "border-success/40",
      bg: "bg-success/10",
      label: "ready",
    },
    rolling: {
      dot: "bg-amber",
      text: "text-amber",
      border: "border-amber/40",
      bg: "bg-amber/10",
      label: "rolling",
    },
    pending: {
      dot: "bg-muted",
      text: "text-muted",
      border: "border-border",
      bg: "bg-panel",
      label: "pending",
    },
  }[state];
  return (
    <span
      className={`inline-flex shrink-0 items-center gap-1.5 rounded-full border px-2 py-0.5 font-mono text-[10px] ${map.border} ${map.bg} ${map.text}`}
    >
      <span className={`h-1.5 w-1.5 rounded-full ${map.dot}`} />
      {map.label}
    </span>
  );
}

// A miniature replica strip for the sidebar rows - a compact echo of the
// signature grid, so the list reads readiness at a glance.
function MiniStrip({ ready, total }: { ready: number; total: number }) {
  const n = Math.min(Math.max(total, 0), 12);
  if (n === 0) return null;
  return (
    <div className="mt-1.5 flex gap-0.5">
      {Array.from({ length: n }).map((_, i) => (
        <span
          key={i}
          className={`h-1 flex-1 rounded-full ${
            i < ready ? "bg-primary/70" : "bg-amber/40"
          }`}
        />
      ))}
    </div>
  );
}

export function Shell() {
  const scopes = useSession((s) => s.scopes);
  const isAdmin = useSession((s) => s.isAdmin);
  const logout = useSession((s) => s.logout);
  const { deployments, loading, refresh } = useDeployments();

  const [selected, setSelected] = useState<DeploymentSummary | null>(null);
  const [showAdmin, setShowAdmin] = useState(isAdmin);

  return (
    <div className="relative z-10 flex h-screen flex-col overflow-hidden">
      {/* Topbar */}
      <header className="sticky top-0 z-20 flex items-center justify-between border-b border-border bg-surface/80 px-4 py-2.5 backdrop-blur-md">
        <div className="flex items-center gap-2.5">
          <div className="relative flex h-7 w-7 items-center justify-center rounded-lg border border-primary/30 bg-primaryDim">
            <span className="font-mono text-sm font-bold text-primary">L</span>
          </div>
          <div className="leading-tight">
            <span className="font-mono text-sm font-semibold tracking-tight text-text">
              LetoRollout
            </span>
            <span className="hidden font-mono text-[10px] uppercase tracking-wider text-muted sm:inline">
              {" "}
              · rollout console
            </span>
          </div>
        </div>
        <div className="flex items-center gap-2">
          <div className="hidden items-center gap-1.5 rounded-full border border-border bg-panel px-2.5 py-1 md:flex">
            <span
              className={`h-1.5 w-1.5 rounded-full ${
                loading ? "bg-amber" : "bg-success"
              }`}
            />
            <span className="font-mono text-[10px] text-subtext">
              {isAdmin
                ? "admin mode"
                : `${scopes.length} scope${scopes.length === 1 ? "" : "s"}`}
            </span>
          </div>
          {isAdmin && (
            <button
              onClick={() => setShowAdmin((v) => !v)}
              className={`rounded-lg border px-2.5 py-1 font-mono text-[11px] transition-all duration-150
                ${
                  showAdmin
                    ? "border-primary/40 bg-primaryDim text-primaryHi"
                    : "border-border bg-panel text-subtext hover:border-borderHi hover:text-text"
                }`}
            >
              {showAdmin ? "← console" : "tokens"}
            </button>
          )}
          <Button variant="ghost" size="sm" onClick={logout}>
            Logout
          </Button>
        </div>
      </header>

      <AnimatePresence mode="wait">
        {showAdmin ? (
          <motion.div
            key="admin"
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            transition={{ duration: 0.2 }}
            className="flex flex-1 overflow-hidden"
          >
            <AdminPanel onExit={() => setShowAdmin(false)} />
          </motion.div>
        ) : (
          <motion.div
            key="console"
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            transition={{ duration: 0.2 }}
            className="flex flex-1 overflow-hidden"
          >
            {/* Sidebar */}
            <aside className="flex w-64 shrink-0 flex-col border-r border-border bg-surface/60 lg:w-72">
              <div className="flex items-center justify-between px-3 py-2.5">
                <span className="font-mono text-[10px] uppercase tracking-wider text-muted">
                  deployments
                </span>
                <button
                  onClick={refresh}
                  disabled={loading}
                  className="font-mono text-[10px] text-muted transition-colors hover:text-primary disabled:opacity-40"
                  title="Refresh"
                >
                  {loading ? "syncing…" : "↻"}
                </button>
              </div>
              <div className="flex-1 overflow-y-auto pb-3">
                {deployments.length === 0 && !loading && (
                  <div className="px-3 py-10 text-center font-mono text-[11px] leading-relaxed text-muted">
                    no deployments
                    <br />
                    in scope
                  </div>
                )}
                <ul>
                  {deployments.map((d) => {
                    const active =
                      selected?.name === d.name &&
                      selected?.namespace === d.namespace;
                    return (
                      <li key={`${d.namespace}/${d.name}`}>
                        <button
                          onClick={() => setSelected(d)}
                          className={`group w-full border-l-2 px-3 py-2.5 text-left transition-all duration-150 ${
                            active
                              ? "border-primary bg-primaryDim/30 text-text"
                              : "border-transparent text-subtext hover:border-borderHi hover:bg-panel/50 hover:text-text"
                          }`}
                        >
                          <div className="flex items-center justify-between gap-2">
                            <span className="truncate font-medium">{d.name}</span>
                            <span className="shrink-0 font-mono text-[10px] text-muted">
                              {d.readyReplicas}/{d.replicas}
                            </span>
                          </div>
                          <div className="mt-0.5 truncate font-mono text-[10px] text-muted">
                            {d.namespace}
                          </div>
                          <MiniStrip ready={d.readyReplicas} total={d.replicas} />
                        </button>
                      </li>
                    );
                  })}
                </ul>
              </div>
            </aside>

            {/* Main bento canvas. On large screens it is an app shell: the
                bento fills the viewport and panels scroll internally, so the
                page itself never scrolls. Mobile falls back to page scroll. */}
            <main className="flex-1 overflow-y-auto p-4 lg:overflow-hidden lg:p-6">
              <AnimatePresence mode="wait">
                {!selected ? (
                  <motion.div
                    key="empty"
                    initial={{ opacity: 0, y: 8 }}
                    animate={{ opacity: 1, y: 0 }}
                    exit={{ opacity: 0 }}
                    transition={{ duration: 0.3, ease: EASE_OUT }}
                    className="flex h-full min-h-[60vh] items-center justify-center"
                  >
                    <div className="text-center">
                      <div className="mx-auto mb-4 flex h-16 w-16 items-center justify-center rounded-2xl border border-dashed border-border">
                        <span className="font-mono text-2xl text-muted">⟶</span>
                      </div>
                      <p className="font-mono text-sm text-subtext">
                        select a deployment on the left
                      </p>
                      <p className="mt-1 font-mono text-[11px] text-muted">
                        to inspect replicas, update images, and tail logs
                      </p>
                    </div>
                  </motion.div>
                ) : (
                  <Bento
                    key={`${selected.namespace}/${selected.name}`}
                    deployment={selected}
                  />
                )}
              </AnimatePresence>
            </main>
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  );
}

function Bento({ deployment }: { deployment: DeploymentSummary }) {
  const reduce = useReducedMotion();
  const state = readyState(deployment.readyReplicas, deployment.replicas);

  return (
    <motion.div
      variants={staggerContainer}
      initial={reduce ? false : "hidden"}
      animate="show"
      exit={{ opacity: 0, transition: { duration: 0.15 } }}
      className="grid grid-cols-1 gap-4 lg:h-full lg:grid-cols-3 lg:grid-rows-[auto_minmax(0,1fr)]"
    >
      {/* Hero: identity + replica readiness grid (the signature) */}
      <motion.section
        variants={cardVariants}
        className="card flex flex-col gap-4 p-5 lg:col-span-1 lg:row-start-1"
      >
        <div className="flex items-start justify-between gap-2">
          <div className="min-w-0">
            <div className="truncate font-mono text-base font-semibold text-text">
              {deployment.name}
            </div>
            <div className="truncate font-mono text-[11px] text-muted">
              {deployment.namespace}
            </div>
          </div>
          <StateBadge state={state} />
        </div>

        <div className="border-t border-border pt-4">
          <span className="mb-3 block font-mono text-[10px] uppercase tracking-wider text-muted">
            replica readiness
          </span>
          <ReplicaGrid
            replicas={deployment.replicas}
            ready={deployment.readyReplicas}
          />
        </div>

        <div className="border-t border-border pt-4">
          <span className="mb-2 block font-mono text-[10px] uppercase tracking-wider text-muted">
            containers
          </span>
          <ul className="flex flex-col gap-2">
            {deployment.containers.map((c) => (
              <li key={c.name} className="flex flex-col gap-0.5">
                <span className="font-mono text-xs text-text">{c.name}</span>
                <span
                  className="truncate font-mono text-[10px] text-aurora"
                  title={c.image}
                >
                  {c.image}
                </span>
              </li>
            ))}
          </ul>
        </div>
      </motion.section>

      {/* Image update */}
      <motion.section
        variants={cardVariants}
        className="card p-5 lg:col-span-2 lg:row-start-1"
      >
        <div className="mb-4">
          <span className="font-mono text-[10px] uppercase tracking-wider text-primary">
            image update
          </span>
        </div>
        <ImageUpdateForm deployment={deployment} />
      </motion.section>

      {/* Logs terminal */}
      <motion.section
        variants={cardVariants}
        className="card flex flex-col p-5 lg:col-span-3 lg:row-start-2 lg:min-h-0"
      >
        <div className="mb-4">
          <span className="font-mono text-[10px] uppercase tracking-wider text-aurora">
            logs
          </span>
        </div>
        <LogViewer deployment={deployment} />
      </motion.section>
    </motion.div>
  );
}
