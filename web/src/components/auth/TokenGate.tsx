import { useState } from "react";
import { api, ApiError } from "../../api/client";
import { useSession, useToasts } from "../../state/store";
import { Button } from "../ui/Button";

export function TokenGate() {
  const [token, setToken] = useState("");
  const [loading, setLoading] = useState(false);
  const setSession = useSession((s) => s.setSession);
  const push = useToasts((s) => s.push);

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!token.trim()) return;
    setLoading(true);
    try {
      const v = await api.verify(token.trim());
      setSession(token.trim(), v);
      push(
        "success",
        v.isAdmin ? "Logged in as admin" : "Logged in"
      );
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
    <div className="min-h-screen flex items-center justify-center bg-bg">
      <div className="w-full max-w-sm bg-panel border border-border rounded-lg p-8">
        <h1 className="text-lg font-semibold text-text mb-1">
          LetoRollout Console
        </h1>
        <p className="text-xs text-muted mb-6">
          Enter your access token to continue.
        </p>
        <form onSubmit={submit} className="flex flex-col gap-3">
          <input
            type="password"
            value={token}
            onChange={(e) => setToken(e.target.value)}
            placeholder="token"
            autoFocus
            className="w-full bg-bg border border-border rounded-md px-3 py-2 text-sm text-text placeholder-muted focus:outline-none focus:border-primary"
          />
          <Button type="submit" variant="primary" loading={loading} className="w-full">
            Enter
          </Button>
        </form>
      </div>
    </div>
  );
}
