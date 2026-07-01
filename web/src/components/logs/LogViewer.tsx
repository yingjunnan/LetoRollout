import { useEffect, useRef, useState } from "react";
import { api, ApiError } from "../../api/client";
import type { DeploymentSummary } from "../../api/types";
import { useSession, useToasts } from "../../state/store";
import { Button } from "../ui/Button";
import { Input } from "../ui/Input";
import { Select } from "../ui/Select";

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
    <div className="flex h-full flex-col gap-3">
      <div className="flex flex-wrap items-end gap-3">
        <Select
          label="Container"
          value={container}
          onChange={(e) => setContainer(e.target.value)}
          className="w-auto min-w-[140px]"
        >
          {deployment.containers.map((c) => (
            <option key={c.name} value={c.name}>
              {c.name}
            </option>
          ))}
        </Select>
        <Input
          label="Tail lines"
          type="number"
          value={tailLines}
          onChange={(e) => setTailLines(e.target.value)}
          className="w-28"
        />
        <label className="flex cursor-pointer items-center gap-2 pb-2 text-sm text-text">
          <input
            type="checkbox"
            checked={previous}
            onChange={(e) => setPrevious(e.target.checked)}
            className="h-4 w-4 accent-primary"
          />
          Previous
        </label>
        <div className="flex gap-2 pb-1">
          <Button variant="secondary" onClick={fetchOnce} loading={loading}>
            Fetch
          </Button>
          <Button variant={following ? "danger" : "primary"} onClick={toggleFollow}>
            {following ? "■ Stop" : "● Follow"}
          </Button>
        </div>
      </div>

      <div className="relative flex-1 overflow-hidden rounded-md border border-border bg-surface min-h-64">
        {following && (
          <div className="absolute right-3 top-2 z-10 flex items-center gap-1.5 rounded border border-success/40 bg-success/10 px-2 py-0.5 font-mono text-[10px] text-success">
            <span className="h-1.5 w-1.5 animate-pulse-ring rounded-full bg-success" />
            live
          </div>
        )}
        <pre className="h-full overflow-auto p-3 font-mono text-xs leading-relaxed text-text whitespace-pre-wrap">
          {lines.length === 0 ? (
            <span className="text-muted">
              (no logs loaded — click Fetch or Follow)
            </span>
          ) : (
            lines.map((l, i) => (
              <div key={i} className="hover:bg-panel/40">
                <span className="mr-3 select-none text-muted/50">
                  {String(i + 1).padStart(4, "0")}
                </span>
                {l}
              </div>
            ))
          )}
        </pre>
      </div>
    </div>
  );
}
