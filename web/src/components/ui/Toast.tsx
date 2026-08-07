import { AnimatePresence, motion } from "motion/react";
import { useToasts } from "../../state/store";
import { EASE_OUT } from "../../lib/motion";

const STYLES: Record<
  "info" | "error" | "success",
  { border: string; iconBg: string; label: string }
> = {
  error: {
    border: "border-danger/60",
    iconBg: "bg-danger/20 text-dangerHi",
    label: "error",
  },
  success: {
    border: "border-success/60",
    iconBg: "bg-success/20 text-success",
    label: "ok",
  },
  info: {
    border: "border-aurora/60",
    iconBg: "bg-aurora/20 text-aurora",
    label: "info",
  },
};

function icon(kind: "info" | "error" | "success") {
  if (kind === "error")
    return (
      <path
        fillRule="evenodd"
        d="M10 18a8 8 0 1 0 0-16 8 8 0 0 0 0 16zm-1-9.75a.75.75 0 0 1 1.5 0v4a.75.75 0 0 1-1.5 0v-4zM10 6.5a1 1 0 1 1 0 2 1 1 0 0 1 0-2z"
        clipRule="evenodd"
      />
    );
  if (kind === "success")
    return (
      <path
        fillRule="evenodd"
        d="M10 18a8 8 0 1 0 0-16 8 8 0 0 0 0 16zm3.78-9.78a.75.75 0 0 0-1.06-1.06L9 10.94 7.28 9.22a.75.75 0 1 0-1.06 1.06l2.25 2.25a.75.75 0 0 0 1.06 0l4.25-4.31z"
        clipRule="evenodd"
      />
    );
  return (
    <path
      fillRule="evenodd"
      d="M10 18a8 8 0 1 0 0-16 8 8 0 0 0 0 16zm-.75-11.25a.75.75 0 0 1 1.5 0v4.5a.75.75 0 0 1-1.5 0v-4.5zM10 14a1 1 0 1 1 0-2 1 1 0 0 1 0 2z"
      clipRule="evenodd"
    />
  );
}

export function Toasts() {
  const toasts = useToasts((s) => s.toasts);
  const dismiss = useToasts((s) => s.dismiss);

  return (
    <div className="fixed bottom-4 right-4 z-50 flex flex-col gap-2">
      <AnimatePresence initial={false}>
        {toasts.map((t) => {
          const s = STYLES[t.kind];
          return (
            <motion.div
              key={t.id}
              layout
              initial={{ opacity: 0, x: 24, scale: 0.96 }}
              animate={{ opacity: 1, x: 0, scale: 1 }}
              exit={{ opacity: 0, x: 24, scale: 0.96 }}
              transition={{ duration: 0.22, ease: EASE_OUT }}
              onClick={() => dismiss(t.id)}
              className={`flex max-w-sm cursor-pointer items-start gap-2.5 rounded-lg border ${s.border} bg-panel px-3.5 py-2.5 shadow-panel backdrop-blur`}
            >
              <span
                className={`mt-0.5 flex h-5 w-5 shrink-0 items-center justify-center rounded-full ${s.iconBg}`}
              >
                <svg className="h-3.5 w-3.5" viewBox="0 0 20 20" fill="currentColor">
                  {icon(t.kind)}
                </svg>
              </span>
              <div className="min-w-0">
                <div className="font-mono text-[9px] uppercase tracking-wider text-muted">
                  {s.label}
                </div>
                <div className="text-sm text-text">{t.message}</div>
              </div>
            </motion.div>
          );
        })}
      </AnimatePresence>
    </div>
  );
}
