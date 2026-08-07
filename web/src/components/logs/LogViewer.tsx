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
  const preRef = useRef<HTMLPreElement | null>(null);
  // Whether the view is pinned to the bottom. Cleared when the user scrolls up
  // to inspect history so new lines don't yank them back down; re-armed when
  // they scroll back to the bottom.
  const stickRef = useRef(true);

  const stopFollow = () => {
    if (esRef.current) {
      esRef.current.close();
      esRef.current = null;
    }
    setFollowing(false);
  };

  const startFollow = (c: string) => {
    if (!c) return;
    if (esRef.current) {
      esRef.current.close();
      esRef.current = null;
    }
    setLines([]);
    setFollowing(true);
    stickRef.current = true;
    const es = api.streamLogs(
      token,
      deployment.namespace,
      deployment.name,
      { container: c, tailLines: Number(tailLines) || undefined, previous },
      (line) => setLines((prev) => [...prev, line]),
      (err) => {
        push("error", `Log stream: ${err}`);
        stopFollow();
      }
    );
    esRef.current = es;
  };

  const fetchOnce = async () => {
    stopFollow();
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
    startFollow(container);
  };

  const onScroll = () => {
    const el = preRef.current;
    if (!el) return;
    stickRef.current = el.scrollHeight - el.scrollTop - el.clientHeight < 40;
  };

  // On mount / deployment change: reset to the first container and auto-start
  // following so logs stream as soon as a deployment is opened.
  useEffect(() => {
    const initialContainer = deployment.containers[0]?.name ?? "";
    setContainer(initialContainer);
    setLines([]);
    stopFollow();
    startFollow(initialContainer);
    return () => stopFollow();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [deployment]);

  // Keep the view pinned to the latest line while following, unless the user
  // has scrolled up to read history.
  useEffect(() => {
    const el = preRef.current;
    if (!el || !following || !stickRef.current) return;
    el.scrollTop = el.scrollHeight;
  }, [lines, following]);

  return (
    <div className="flex min-h-0 flex-col gap-3 lg:flex-1">
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

      {/* The terminal scrolls internally - on large screens it fills the
          remaining viewport height, so accumulating lines never stretch the
          page. overscroll-contain stops scroll chaining to the page. */}
      <div className="relative flex min-h-0 flex-col rounded-lg border border-border bg-surface lg:flex-1">
        {following && (
          <div className="absolute right-3 top-2 z-10 flex items-center gap-1.5 rounded border border-success/40 bg-success/10 px-2 py-0.5 font-mono text-[10px] text-success">
            <span className="h-1.5 w-1.5 animate-pulse-ring rounded-full bg-success" />
            live
          </div>
        )}
        <pre
          ref={preRef}
          onScroll={onScroll}
          className="flex-1 min-h-[340px] max-h-[60vh] overflow-auto overscroll-contain p-3 font-mono text-xs leading-relaxed text-text whitespace-pre-wrap lg:min-h-0 lg:max-h-none"
        >
          {lines.length === 0 ? (
            <span className="text-muted">(streaming…)</span>
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
