import { useState } from "react";
import { useSession } from "../../state/store";
import { Button } from "../ui/Button";
import { useDeployments } from "../../hooks/useDeployments";
import type { DeploymentSummary } from "../../api/types";
import { ImageUpdateForm } from "../deploy/ImageUpdateForm";
import { LogViewer } from "../logs/LogViewer";
import { AdminPanel } from "../admin/AdminPanel";

type Tab = "image" | "logs";

export function Shell() {
  const scopes = useSession((s) => s.scopes);
  const isAdmin = useSession((s) => s.isAdmin);
  const logout = useSession((s) => s.logout);
  const { deployments, loading, refresh } = useDeployments();

  const [selected, setSelected] = useState<DeploymentSummary | null>(null);
  const [tab, setTab] = useState<Tab>("image");
  // An admin lands on the token-management panel; they can switch back to the
  // (empty) console via the topbar toggle.
  const [showAdmin, setShowAdmin] = useState(isAdmin);

  return (
    <div className="min-h-screen flex flex-col bg-bg">
      {/* Topbar */}
      <header className="flex items-center justify-between border-b border-border bg-panel px-4 py-2">
        <span className="font-semibold text-text">LetoRollout · Console</span>
        <div className="flex items-center gap-3">
          {isAdmin && (
            <button
              onClick={() => setShowAdmin((v) => !v)}
              className="text-xs text-muted hover:text-text"
            >
              {showAdmin ? "Back to console" : "Token management"}
            </button>
          )}
          <span className="text-xs text-muted">
            {scopes
              .map((s) =>
                s.deployment ? `${s.namespace}/${s.deployment}` : s.namespace
              )
              .join(", ")}
          </span>
          <Button variant="secondary" onClick={logout} className="!py-0.5 !text-xs">
            Logout
          </Button>
        </div>
      </header>

      {showAdmin ? (
        <AdminPanel onExit={() => setShowAdmin(false)} />
      ) : (
        <div className="flex flex-1 overflow-hidden">
          {/* Sidebar */}
          <aside className="w-64 shrink-0 border-r border-border bg-panel overflow-y-auto">
            <div className="flex items-center justify-between px-3 py-2">
              <span className="text-xs uppercase text-muted">Deployments</span>
              <button
                onClick={refresh}
                disabled={loading}
                className="text-xs text-muted hover:text-text disabled:opacity-50"
              >
                ↻
              </button>
            </div>
            <ul>
              {deployments.length === 0 && !loading && (
                <li className="px-3 py-2 text-xs text-muted">
                  No deployments in scope.
                </li>
              )}
              {deployments.map((d) => (
                <li key={`${d.namespace}/${d.name}`}>
                  <button
                    onClick={() => setSelected(d)}
                    className={`w-full text-left px-3 py-2 text-sm hover:bg-bg border-l-2 ${
                      selected?.name === d.name && selected?.namespace === d.namespace
                        ? "border-primary text-text bg-bg"
                        : "border-transparent text-muted"
                    }`}
                  >
                    <div className="truncate">{d.name}</div>
                    <div className="text-xs text-muted truncate">{d.namespace}</div>
                  </button>
                </li>
              ))}
            </ul>
          </aside>

          {/* Main */}
          <main className="flex-1 overflow-y-auto p-6">
            {!selected ? (
              <p className="text-sm text-muted">
                Select a deployment on the left.
              </p>
            ) : (
              <div>
                <div className="flex items-center gap-2 mb-4">
                  <h2 className="text-base font-semibold text-text">
                    {selected.namespace}/{selected.name}
                  </h2>
                  <span className="text-xs text-muted">
                    {selected.readyReplicas}/{selected.replicas} ready
                  </span>
                </div>
                <div className="flex gap-1 mb-4 border-b border-border">
                  {(["image", "logs"] as Tab[]).map((t) => (
                    <button
                      key={t}
                      onClick={() => setTab(t)}
                      className={`px-3 py-1.5 text-sm rounded-t-md ${
                        tab === t
                          ? "text-text border-b-2 border-primary -mb-px"
                          : "text-muted hover:text-text"
                      }`}
                    >
                      {t === "image" ? "Image update" : "Logs"}
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
