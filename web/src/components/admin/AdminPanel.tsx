import { useCallback, useEffect, useState } from "react";
import { motion, useReducedMotion } from "motion/react";
import { api, ApiError } from "../../api/client";
import type { TokenRecord, TokenScope } from "../../api/types";
import { useSession, useToasts } from "../../state/store";
import { Button } from "../ui/Button";
import { Input } from "../ui/Input";
import { Select } from "../ui/Select";
import { ExpiryPicker, type ExpiryValue } from "./ExpiryPicker";
import { cardVariants, staggerContainer } from "../../lib/motion";

export function AdminPanel({ onExit }: { onExit: () => void }) {
  const token = useSession((s) => s.token)!;
  const push = useToasts((s) => s.push);
  const reduce = useReducedMotion();
  const [tokens, setTokens] = useState<TokenRecord[]>([]);
  const [loading, setLoading] = useState(false);

  // namespace picker state
  const [namespaces, setNamespaces] = useState<string[]>([]);
  const [nsLoading, setNsLoading] = useState(true);

  // create-form state
  const [label, setLabel] = useState("");
  const [ns, setNs] = useState("");
  const [dep, setDep] = useState("");
  const [expires, setExpires] = useState<ExpiryValue>(null);
  const [creating, setCreating] = useState(false);
  const [createdToken, setCreatedToken] = useState<string | null>(null);

  const refresh = useCallback(async () => {
    setLoading(true);
    try {
      setTokens(await api.adminListTokens(token));
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) {
        push("error", "Session expired");
      } else {
        push("error", `List failed: ${(err as Error).message}`);
      }
    } finally {
      setLoading(false);
    }
  }, [token, push]);

  const refreshNamespaces = useCallback(async () => {
    setNsLoading(true);
    try {
      const list = await api.adminListNamespaces(token);
      setNamespaces(list);
      if (!ns && list.length > 0) setNs(list.includes("default") ? "default" : list[0]);
    } catch (err) {
      push("error", `Failed to load namespaces: ${(err as Error).message}`);
    } finally {
      setNsLoading(false);
    }
  }, [token, push, ns]);

  useEffect(() => {
    refresh();
    refreshNamespaces();
  }, [refresh, refreshNamespaces]);

  const create = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!ns) {
      push("error", "Namespace is required");
      return;
    }
    setCreating(true);
    setCreatedToken(null);
    try {
      const scopes: TokenScope[] = [{ namespace: ns, deployment: dep || "" }];
      const rec = await api.adminCreateToken(token, {
        label,
        scopes,
        expiresAt: expires,
      });
      setCreatedToken(rec.token ?? null);
      push("success", "Token created");
      setLabel("");
      setDep("");
      setExpires(null);
      refresh();
    } catch (err) {
      push("error", `Create failed: ${(err as Error).message}`);
    } finally {
      setCreating(false);
    }
  };

  const remove = async (id: string) => {
    if (!confirm("Delete this token?")) return;
    try {
      await api.adminDeleteToken(token, id);
      push("success", "Token deleted");
      refresh();
    } catch (err) {
      push("error", `Delete failed: ${(err as Error).message}`);
    }
  };

  return (
    <div className="relative z-10 flex-1 overflow-y-auto p-4 lg:p-6">
      <div className="mx-auto mb-5 flex max-w-5xl items-center justify-between">
        <div className="flex items-center gap-3">
          <h2 className="font-mono text-sm font-semibold tracking-tight text-text">
            token_management
          </h2>
          <span className="rounded border border-border bg-surface px-1.5 py-0.5 font-mono text-[10px] text-muted">
            admin
          </span>
        </div>
        <Button variant="ghost" size="sm" onClick={onExit}>
          ← Back to console
        </Button>
      </div>

      <motion.div
        variants={staggerContainer}
        initial={reduce ? false : "hidden"}
        animate="show"
        className="mx-auto grid grid-cols-1 gap-5 lg:grid-cols-2 max-w-5xl"
      >
        {/* create */}
        <motion.form
          onSubmit={create}
          variants={cardVariants}
          className="card flex flex-col gap-4 p-5"
        >
          <span className="font-mono text-xs font-medium uppercase tracking-wider text-primary">
            + create token
          </span>

          <Input
            label="Label"
            value={label}
            onChange={(e) => setLabel(e.target.value)}
            placeholder="alice-prod"
          />

          <Select
            label="Namespace"
            value={ns}
            onChange={(e) => setNs(e.target.value)}
            disabled={nsLoading}
          >
            {nsLoading && <option value="">Loading namespaces…</option>}
            {!nsLoading && namespaces.length === 0 && (
              <option value="">No namespaces available</option>
            )}
            {namespaces.map((n) => (
              <option key={n} value={n}>
                {n}
              </option>
            ))}
          </Select>

          <Input
            label="Deployment (optional — empty = whole namespace)"
            value={dep}
            onChange={(e) => setDep(e.target.value)}
            placeholder="api"
          />

          <ExpiryPicker value={expires} onChange={setExpires} />

          <div className="pt-1">
            <Button type="submit" variant="primary" loading={creating} className="w-full">
              Generate token
            </Button>
          </div>

          {createdToken && (
            <div className="animate-fade-up rounded-md border border-success/40 bg-success/10 p-3">
              <div className="mb-1.5 flex items-center gap-1.5 font-mono text-[10px] uppercase tracking-wider text-success">
                <span className="h-1.5 w-1.5 rounded-full bg-success" />
                token (shown once)
              </div>
              <div className="flex items-center gap-2">
                <code className="block flex-1 break-all font-mono text-xs text-text">
                  {createdToken}
                </code>
                <button
                  type="button"
                  onClick={() => {
                    navigator.clipboard?.writeText(createdToken);
                    push("success", "Copied");
                  }}
                  className="shrink-0 rounded border border-border bg-surface px-2 py-1 font-mono text-[10px] text-subtext transition-colors hover:border-primary/50 hover:text-text"
                >
                  copy
                </button>
              </div>
            </div>
          )}
        </motion.form>

        {/* list */}
        <motion.div variants={cardVariants} className="card p-5">
          <div className="mb-3 flex items-center justify-between">
            <span className="font-mono text-xs font-medium uppercase tracking-wider text-subtext">
              existing tokens
            </span>
            <button
              onClick={refresh}
              disabled={loading}
              className="font-mono text-[10px] text-muted transition-colors hover:text-primary disabled:opacity-40"
            >
              {loading ? "syncing…" : "↻ refresh"}
            </button>
          </div>
          <ul className="flex flex-col gap-2">
            {tokens.length === 0 && !loading && (
              <li className="rounded-md border border-dashed border-border px-3 py-6 text-center font-mono text-xs text-muted">
                no tokens yet
              </li>
            )}
            {tokens.map((t) => (
              <li
                key={t.id}
                className="group rounded-md border border-border bg-surface px-3 py-2.5 transition-colors hover:border-borderHi"
              >
                <div className="flex items-center justify-between">
                  <span className="font-medium text-text">
                    {t.label || (
                      <span className="font-mono text-muted">(no label)</span>
                    )}
                  </span>
                  <Button
                    variant="danger"
                    size="sm"
                    onClick={() => remove(t.id)}
                  >
                    Delete
                  </Button>
                </div>
                <div className="mt-1.5 flex flex-wrap items-center gap-1.5">
                  {t.scopes.map((s, i) => (
                    <span
                      key={i}
                      className="rounded border border-border bg-panel px-1.5 py-0.5 font-mono text-[10px] text-subtext"
                    >
                      {s.deployment ? `${s.namespace}/${s.deployment}` : s.namespace}
                    </span>
                  ))}
                </div>
                <div className="mt-1.5 font-mono text-[10px] text-muted">
                  expires:{" "}
                  {t.expiresAt
                    ? new Date(t.expiresAt).toLocaleString()
                    : "never"}
                </div>
              </li>
            ))}
          </ul>
        </motion.div>
      </motion.div>
    </div>
  );
}
