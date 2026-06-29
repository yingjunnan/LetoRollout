import { useCallback, useEffect, useState } from "react";
import { api, ApiError } from "../api/client";
import type { DeploymentSummary } from "../api/types";
import { useSession, useToasts } from "../state/store";

// Lists deployments across every namespace in the token's scope.
export function useDeployments() {
  const token = useSession((s) => s.token)!;
  const scopes = useSession((s) => s.scopes);
  const logout = useSession((s) => s.logout);
  const push = useToasts((s) => s.push);
  const [deployments, setDeployments] = useState<DeploymentSummary[]>([]);
  const [loading, setLoading] = useState(false);

  const refresh = useCallback(async () => {
    const namespaces = Array.from(
      new Set(scopes.map((s) => s.namespace).filter(Boolean))
    );
    if (namespaces.length === 0) return;
    setLoading(true);
    try {
      const results = await Promise.all(
        namespaces.map((ns) => api.listDeployments(token, ns))
      );
      setDeployments(results.flat());
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) {
        logout();
      } else {
        push("error", `Failed to list deployments: ${(err as Error).message}`);
      }
    } finally {
      setLoading(false);
    }
  }, [token, scopes, logout, push]);

  useEffect(() => {
    refresh();
  }, [refresh]);

  return { deployments, loading, refresh };
}
