import { useCallback, useEffect, useState } from "react";
import { api } from "../../api/client";
import type { TokenRecord, TokenScope } from "../../api/types";
import { useSession, useToasts } from "../../state/store";
import { Button } from "../ui/Button";
import { Input } from "../ui/Input";

export function AdminPanel({ onExit }: { onExit: () => void }) {
  const token = useSession((s) => s.token)!;
  const push = useToasts((s) => s.push);
  const [tokens, setTokens] = useState<TokenRecord[]>([]);
  const [loading, setLoading] = useState(false);

  // create-form state
  const [label, setLabel] = useState("");
  const [ns, setNs] = useState("");
  const [dep, setDep] = useState("");
  const [expires, setExpires] = useState("");
  const [creating, setCreating] = useState(false);
  const [createdToken, setCreatedToken] = useState<string | null>(null);

  const refresh = useCallback(async () => {
    setLoading(true);
    try {
      setTokens(await api.adminListTokens(token));
    } catch (err) {
      push("error", `List failed: ${(err as Error).message}`);
    } finally {
      setLoading(false);
    }
  }, [token, push]);

  useEffect(() => {
    refresh();
  }, [refresh]);

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
        expiresAt: expires || null,
      });
      setCreatedToken(rec.token ?? null);
      push("success", "Token created");
      setLabel("");
      setNs("");
      setDep("");
      setExpires("");
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
    <div className="flex-1 overflow-y-auto p-6">
      <div className="flex items-center justify-between mb-4">
        <h2 className="text-base font-semibold text-text">Token management</h2>
        <Button variant="secondary" onClick={onExit} className="!text-xs">
          Back to console
        </Button>
      </div>

      <div className="grid grid-cols-2 gap-6 max-w-4xl">
        {/* create */}
        <form
          onSubmit={create}
          className="bg-panel border border-border rounded-md p-4 flex flex-col gap-3"
        >
          <span className="text-sm font-medium text-text">Create token</span>
          <Input
            label="Label"
            value={label}
            onChange={(e) => setLabel(e.target.value)}
            placeholder="alice-prod"
          />
          <Input
            label="Namespace"
            value={ns}
            onChange={(e) => setNs(e.target.value)}
            placeholder="default"
          />
          <Input
            label="Deployment (optional — empty = whole namespace)"
            value={dep}
            onChange={(e) => setDep(e.target.value)}
            placeholder="api"
          />
          <Input
            label="Expires at (optional, RFC3339)"
            value={expires}
            onChange={(e) => setExpires(e.target.value)}
            placeholder="2026-12-31T00:00:00Z"
          />
          <Button type="submit" variant="primary" loading={creating}>
            Create
          </Button>
          {createdToken && (
            <div className="bg-bg border border-border rounded-md p-2 text-xs font-mono break-all text-success">
              {createdToken}
              <div className="text-muted mt-1">
                (shown once — copy now)
              </div>
            </div>
          )}
        </form>

        {/* list */}
        <div className="bg-panel border border-border rounded-md p-4">
          <div className="flex items-center justify-between mb-2">
            <span className="text-sm font-medium text-text">
              Existing tokens
            </span>
            <button
              onClick={refresh}
              disabled={loading}
              className="text-xs text-muted hover:text-text"
            >
              ↻
            </button>
          </div>
          <ul className="flex flex-col gap-2">
            {tokens.length === 0 && (
              <li className="text-xs text-muted">No tokens.</li>
            )}
            {tokens.map((t) => (
              <li
                key={t.id}
                className="bg-bg border border-border rounded-md p-2 text-xs"
              >
                <div className="flex items-center justify-between">
                  <span className="font-medium text-text">
                    {t.label || "(no label)"}
                  </span>
                  <Button
                    variant="danger"
                    onClick={() => remove(t.id)}
                    className="!py-0.5 !px-2 !text-xs"
                  >
                    Delete
                  </Button>
                </div>
                <div className="text-muted mt-1">
                  scopes:{" "}
                  {t.scopes
                    .map((s) =>
                      s.deployment ? `${s.namespace}/${s.deployment}` : s.namespace
                    )
                    .join(", ")}
                </div>
                <div className="text-muted">
                  expires: {t.expiresAt ?? "never"}
                </div>
              </li>
            ))}
          </ul>
        </div>
      </div>
    </div>
  );
}
