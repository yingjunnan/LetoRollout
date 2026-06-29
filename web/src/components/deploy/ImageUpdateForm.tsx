import { useEffect, useState } from "react";
import { api, ApiError } from "../../api/client";
import type { DeploymentSummary, RolloutResult } from "../../api/types";
import { useSession, useToasts } from "../../state/store";
import { Button } from "../ui/Button";
import { Input } from "../ui/Input";

export function ImageUpdateForm({ deployment }: { deployment: DeploymentSummary }) {
  const token = useSession((s) => s.token)!;
  const logout = useSession((s) => s.logout);
  const push = useToasts((s) => s.push);

  const [container, setContainer] = useState("");
  const [image, setImage] = useState("");
  const [dryRun, setDryRun] = useState(false);
  const [wait, setWait] = useState(false);
  const [timeout, setTimeout_] = useState("300");
  const [loading, setLoading] = useState(false);
  const [result, setResult] = useState<RolloutResult | null>(null);

  // When the selected deployment changes, default to its first container and
  // seed the image field with the current image.
  useEffect(() => {
    const c = deployment.containers[0];
    setContainer(c?.name ?? "");
    setImage(c?.image ?? "");
    setResult(null);
  }, [deployment]);

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    setResult(null);
    try {
      const res = await api.updateImage(token, deployment.namespace, deployment.name, {
        container,
        image,
        dryRun,
        wait,
        timeoutSeconds: wait ? Number(timeout) : undefined,
      });
      setResult(res);
      push("success", dryRun ? "Dry-run complete" : "Image updated");
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) {
        logout();
      } else if (err instanceof ApiError && err.status === 403) {
        push("error", "No access to this resource");
      } else {
        push("error", `Update failed: ${(err as Error).message}`);
      }
    } finally {
      setLoading(false);
    }
  };

  return (
    <form onSubmit={submit} className="max-w-md flex flex-col gap-4">
      <div>
        <span className="block text-xs text-muted mb-1">Container</span>
        <select
          value={container}
          onChange={(e) => {
            setContainer(e.target.value);
            const c = deployment.containers.find((c) => c.name === e.target.value);
            if (c) setImage(c.image);
          }}
          className="w-full bg-bg border border-border rounded-md px-3 py-1.5 text-sm text-text focus:outline-none focus:border-primary"
        >
          {deployment.containers.map((c) => (
            <option key={c.name} value={c.name}>
              {c.name} ({c.image})
            </option>
          ))}
        </select>
      </div>

      <Input
        label="Image"
        value={image}
        onChange={(e) => setImage(e.target.value)}
        placeholder="registry/image:tag"
      />

      <div className="flex flex-col gap-2">
        <label className="flex items-center gap-2 text-sm text-text">
          <input
            type="checkbox"
            checked={dryRun}
            onChange={(e) => setDryRun(e.target.checked)}
          />
          Dry run (preview without patching)
        </label>
        <label className="flex items-center gap-2 text-sm text-text">
          <input
            type="checkbox"
            checked={wait}
            onChange={(e) => setWait(e.target.checked)}
          />
          Wait for rollout
        </label>
        {wait && (
          <Input
            label="Timeout (seconds)"
            type="number"
            value={timeout}
            onChange={(e) => setTimeout_(e.target.value)}
          />
        )}
      </div>

      <div>
        <Button type="submit" variant="primary" loading={loading}>
          {dryRun ? "Preview" : "Update image"}
        </Button>
      </div>

      {result && (
        <div className="bg-panel border border-border rounded-md p-3 text-xs font-mono whitespace-pre-wrap text-text">
          {JSON.stringify(result, null, 2)}
        </div>
      )}
    </form>
  );
}
