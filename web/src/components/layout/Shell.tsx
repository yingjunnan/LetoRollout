import { useState } from "react";
import { useSession } from "../../state/store";
import { Button } from "../ui/Button";
import { useDeployments } from "../../hooks/useDeployments";
import type { DeploymentSummary } from "../../api/types";
import { ImageUpdateForm } from "../deploy/ImageUpdateForm";
import { LogViewer } from "../logs/LogViewer";
import { AdminPanel } from "../admin/AdminPanel";

type Tab = "image" | "logs";

function readyBadge(ready: number, total: number) {
  const ok = total > 0 && ready >= total;
  return (
    <span
      className={`inline-flex items-center gap-1 rounded border px-1.5 py-0.5 font-mono text-[10px] ${
        ok
          ? "border-success/40 bg-success/10 text-success"
          : "border-warning/40 bg-warning/10 text-warning"
      }`}
    >
      <span
        className={`h-1.5 w-1.5 rounded-full ${ok ? "bg-success" : "bg-warning"}`}
      />
      {ready}/{total}
    </span>
  );
}

export function Shell() {
  const scopes = useSession((s) => s.scopes);
  const isAdmin = useSession((s) => s.isAdmin);
  const logout = useSession((s) => s.logout);
  const { deployments, loading, refresh } = useDeployments();

  const [selected, setSelected] = useState<DeploymentSummary | null>(null);
  const [tab, setTab] = useState<Tab>("image");
  const [showAdmin, setShowAdmin] = useState(isAdmin);

  return (
    <div className="relative z-10 flex min-h-screen flex-col">
      {/* Topbar */}
      <header className="flex items-center justify-between border-b border-border bg-surface/80 px-4 py-2.5 backdrop-blur">
        <div className="flex items-center gap-2.5">
          <div className="flex h-6 w-6 items-center justify-center rounded border border-primary/30 bg-primaryDim">
            <span className="font-mono text-xs font-bold text-primary">L</span>
          </div>
          <span className="font-mono text-sm font-semibold tracking-tight text-text">
            LetoRollout
          </span>
          <span className="hidden font-mono text-[10px] uppercase tracking-wider text-muted sm:inline">
            · rollout console
          </span>
        </div>
        <div className="flex items-center gap-2">
          {isAdmin && (
            <button
              onClick={() => setShowAdmin((v) => !v)}
              className={`rounded-md border px-2.5 py-1 font-mono text-[11px] transition-all duration-150
                ${
                  showAdmin
                    ? "border-primary/40 bg-primaryDim text-primaryHi"
                    : "border-border bg-panel text-subtext hover:border-borderHi hover:text-text"
                }`}
            >
              {showAdmin ? "← console" : "tokens"}
            </button>
          )}
          {scopes.length > 0 && (
            <div className="hidden items-center gap-1 md:flex">
              {scopes.map((s, i) => (
                <span
                  key={i}
                  className="rounded border border-border bg-panel px-1.5 py-0.5 font-mono text-[10px] text-subtext"
                >
                  {s.deployment ? `${s.namespace}/${s.deployment}` : s.namespace}
                </span>
              ))}
            </div>
          )}
          <Button variant="ghost" size="sm" onClick={logout}>
            Logout
          </Button>
        </div>
      </header>

      {showAdmin ? (
        <AdminPanel onExit={() => setShowAdmin(false)} />
      ) : (
        <div className="flex flex-1 overflow-hidden">
          {/* Sidebar */}
          <aside className="w-64 shrink-0 overflow-y-auto border-r border-border bg-surface/60">
            <div className="flex items-center justify-between px-3 py-2.5">
              <span className="font-mono text-[10px] uppercase tracking-wider text-muted">
                deployments
              </span>
              <button
                onClick={refresh}
                disabled={loading}
                className="font-mono text-[10px] text-muted transition-colors hover:text-primary disabled:opacity-40"
              >
                {loading ? "…" : "↻"}
              </button>
            </div>
            <ul>
              {deployments.length === 0 && !loading && (
                <li className="px-3 py-8 text-center font-mono text-[11px] text-muted">
                  no deployments
                  <br />
                  in scope
                </li>
              )}
              {deployments.map((d) => {
                const active =
                  selected?.name === d.name && selected?.namespace === d.namespace;
                return (
                  <li key={`${d.namespace}/${d.name}`}>
                    <button
                      onClick={() => setSelected(d)}
                      className={`group w-full border-l-2 px-3 py-2.5 text-left transition-all duration-150 ${
                        active
                          ? "border-primary bg-primaryDim/40 text-text"
                          : "border-transparent text-subtext hover:border-borderHi hover:bg-panel/60 hover:text-text"
                      }`}
                    >
                      <div className="flex items-center justify-between gap-2">
                        <span className="truncate font-medium">{d.name}</span>
                        {readyBadge(d.readyReplicas, d.replicas)}
                      </div>
                      <div className="mt-0.5 truncate font-mono text-[10px] text-muted">
                        {d.namespace}
                      </div>
                    </button>
                  </li>
                );
              })}
            </ul>
          </aside>

          {/* Main */}
          <main className="flex-1 overflow-y-auto p-6">
            {!selected ? (
              <div className="flex h-full items-center justify-center">
                <div className="text-center">
                  <div className="mx-auto mb-3 flex h-12 w-12 items-center justify-center rounded-lg border border-dashed border-border">
                    <span className="font-mono text-lg text-muted">⟶</span>
                  </div>
                  <p className="font-mono text-xs text-muted">
                    select a deployment on the left
                  </p>
                </div>
              </div>
            ) : (
              <div className="animate-fade-up">
                <div className="mb-5 flex items-center gap-2.5">
                  <h2 className="font-mono text-sm font-semibold text-text">
                    {selected.namespace}
                    <span className="text-muted">/</span>
                    {selected.name}
                  </h2>
                  {readyBadge(selected.readyReplicas, selected.replicas)}
                </div>
                <div className="mb-5 flex gap-1 border-b border-border">
                  {(["image", "logs"] as Tab[]).map((t) => (
                    <button
                      key={t}
                      onClick={() => setTab(t)}
                      className={`-mb-px border-b-2 px-3 py-2 font-mono text-xs transition-colors ${
                        tab === t
                          ? "border-primary text-text"
                          : "border-transparent text-muted hover:text-subtext"
                      }`}
                    >
                      {t === "image" ? "image update" : "logs"}
                    </button>
                  ))}
                </div>
                {tab === "image" ? (
                  <ImageUpdateForm deployment={selected} />
                ) : (
                  <LogViewer deployment={selected} />
                )}
              </div>
            )}
          </main>
        </div>
      )}
    </div>
  );
}
