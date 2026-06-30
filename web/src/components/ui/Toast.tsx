import { useToasts } from "../../state/store";

export function Toasts() {
  const toasts = useToasts((s) => s.toasts);
  const dismiss = useToasts((s) => s.dismiss);

  const color = (kind: "info" | "error" | "success") =>
    kind === "error"
      ? "border-danger"
      : kind === "success"
      ? "border-success"
      : "border-primary";

  return (
    <div className="fixed bottom-4 right-4 z-50 flex flex-col gap-2">
      {toasts.map((t) => (
        <div
          key={t.id}
          className={`bg-panel border-l-4 ${color(t.kind)} px-4 py-2 rounded-md shadow-lg text-sm text-text max-w-sm cursor-pointer`}
          onClick={() => dismiss(t.id)}
        >
          {t.message}
        </div>
      ))}
    </div>
  );
}
