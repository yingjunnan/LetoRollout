import { useEffect, useRef, useState } from "react";
import { api, ApiError } from "../../api/client";
import type { DeploymentSummary } from "../../api/types";
import { useSession, useToasts } from "../../state/store";
import { Button } from "../ui/Button";
import { Input } from "../ui/Input";

export function LogViewer({ deployment }: { deployment: DeploymentSummary }) {
  const token = useSession((s) => s.token)!;
  const logout = useSession((s) => s.logout);
  const push = useToasts((s) => s.push);

  const [container, setContainer] = useState(deployment.containers[0]?.name ?? "");
  const [tailLines, setTailLines] = useState("500");
  const [previous, setPrevious] = useState(false);
  const [lines, setLines] = useState<string[]>([]);
  const [loading, setLoading] = useState(false);
  const [following, setFollowing] = useState(false);
  const esRef = useRef<EventSource | null>(null);

  useEffect(() => {
    setContainer(deployment.containers[0]?.name ?? "");
    setLines([]);
    stopFollow();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [deployment]);

  const stopFollow = () => {
    if (esRef.current) {
      esRef.current.close();
      esRef.current = null;
    }
    setFollowing(false);
  };

  const fetchOnce = async () => {
    setLoading(true);
    setLines([]);
    try {
      const text = await api.fetchLogs(token, deployment.namespace, deployment.name, {
        container,
        tailLines: Number(tailLines) || undefined,
        previous,
      });
      setLines(text ? text.split("\n") : ["(no logs)"]);
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) {
        logout();
      } else if (err instanceof ApiError && err.status === 403) {
        push("error", "No access to this resource");
      } else {
        push("error", `Logs failed: ${(err as Error).message}`);
      }
    } finally {
      setLoading(false);
    }
  };

  const toggleFollow = () => {
    if (following) {
      stopFollow();
      return;
    }
    setLines([]);
    setFollowing(true);
    const es = api.streamLogs(
      token,
      deployment.namespace,
      deployment.name,
      { container, tailLines: Number(tailLines) || undefined, previous },
      (line) => setLines((prev) => [...prev, line]),
      (err) => {
        push("error", `Log stream: ${err}`);
        stopFollow();
      }
    );
    esRef.current = es;
  };

  useEffect(() => () => stopFollow(), []);

  return (
    <div className="flex flex-col gap-3 h-full">
      <div className="flex flex-wrap items-end gap-3">
        <label className="block">
          <span className="block text-xs text-muted mb-1">Container</span>
          <select
            value={container}
            onChange={(e) => setContainer(e.target.value)}
            className="bg-bg border border-border rounded-md px-3 py-1.5 text-sm text-text focus:outline-none focus:border-primary"
          >
            {deployment.containers.map((c) => (
              <option key={c.name} value={c.name}>
                {c.name}
              </option>
            ))}
          </select>
        </label>
        <Input
          label="Tail lines"
          type="number"
          value={tailLines}
          onChange={(e) => setTailLines(e.target.value)}
          className="w-28"
        />
        <label className="flex items-center gap-2 text-sm text-text pb-1.5">
          <input
            type="checkbox"
            checked={previous}
            onChange={(e) => setPrevious(e.target.checked)}
          />
          Previous
        </label>
        <div className="flex gap-2 pb-1">
          <Button variant="secondary" onClick={fetchOnce} loading={loading}>
            Fetch
          </Button>
          <Button
            variant={following ? "danger" : "primary"}
            onClick={toggleFollow}
          >
            {following ? "Stop" : "Follow"}
          </Button>
        </div>
      </div>

      <pre className="flex-1 overflow-auto bg-bg border border-border rounded-md p-3 text-xs font-mono text-text whitespace-pre-wrap min-h-64">
        {lines.length === 0
          ? "(no logs loaded — click Fetch or Follow)"
          : lines.join("\n")}
      </pre>
    </div>
  );
}
