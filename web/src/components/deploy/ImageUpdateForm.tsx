import { useEffect, useState } from "react";
import { api, ApiError } from "../../api/client";
import type { DeploymentSummary, RolloutResult } from "../../api/types";
import { useSession, useToasts } from "../../state/store";
import { Button } from "../ui/Button";
import { Input } from "../ui/Input";
import { Select } from "../ui/Select";

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
    <form onSubmit={submit} className="flex max-w-lg flex-col gap-4">
      <Select
        label="Container"
        value={container}
        onChange={(e) => {
          setContainer(e.target.value);
          const c = deployment.containers.find((c) => c.name === e.target.value);
          if (c) setImage(c.image);
        }}
      >
        {deployment.containers.map((c) => (
          <option key={c.name} value={c.name}>
            {c.name} ({c.image})
          </option>
        ))}
      </Select>

      <Input
        label="Image"
        value={image}
        onChange={(e) => setImage(e.target.value)}
        placeholder="registry/image:tag"
      />

      <div className="flex flex-col gap-2.5">
        <ToggleRow
          checked={dryRun}
          onChange={setDryRun}
          label="Dry run"
          hint="preview without patching"
        />
        <ToggleRow
          checked={wait}
          onChange={setWait}
          label="Wait for rollout"
          hint="block until ready"
        />
        {wait && (
          <Input
            label="Timeout (seconds)"
            type="number"
            value={timeout}
            onChange={(e) => setTimeout_(e.target.value)}
          />
        )}
      </div>

      <div className="pt-1">
        <Button type="submit" variant="primary" loading={loading}>
          {dryRun ? "Preview" : "Update image"}
        </Button>
      </div>

      {result && (
        <div className="animate-fade-up rounded-md border border-success/40 bg-success/5 p-3">
          <div className="mb-1.5 flex items-center gap-1.5 font-mono text-[10px] uppercase tracking-wider text-success">
            <span className="h-1.5 w-1.5 rounded-full bg-success" />
            result
          </div>
          <pre className="overflow-auto whitespace-pre-wrap break-all font-mono text-[11px] text-text">
            {JSON.stringify(result, null, 2)}
          </pre>
        </div>
      )}
    </form>
  );
}

function ToggleRow({
  checked,
  onChange,
  label,
  hint,
}: {
  checked: boolean;
  onChange: (v: boolean) => void;
  label: string;
  hint: string;
}) {
  return (
    <label className="flex cursor-pointer items-center justify-between gap-3 rounded-md border border-border bg-surface px-3 py-2 transition-colors hover:border-borderHi">
      <span className="flex items-baseline gap-2">
        <span className="text-sm text-text">{label}</span>
        <span className="font-mono text-[10px] text-muted">{hint}</span>
      </span>
      <button
        type="button"
        role="switch"
        aria-checked={checked}
        onClick={() => onChange(!checked)}
        className={`relative h-5 w-9 shrink-0 rounded-full transition-colors duration-200 ${
          checked ? "bg-primary" : "bg-border"
        }`}
      >
        <span
          className={`absolute top-0.5 h-4 w-4 rounded-full bg-white shadow transition-transform duration-200 ${
            checked ? "translate-x-4" : "translate-x-0.5"
          }`}
        />
      </button>
    </label>
  );
}
