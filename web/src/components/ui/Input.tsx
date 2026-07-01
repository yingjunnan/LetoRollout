import type { InputHTMLAttributes } from "react";

interface InputProps extends InputHTMLAttributes<HTMLInputElement> {
  label?: string;
  hint?: string;
}

export function Input({ label, hint, id, className = "", ...rest }: InputProps) {
  return (
    <label className="block">
      {label && (
        <span className="mb-1.5 block font-mono text-[10px] uppercase tracking-wider text-muted">
          {label}
        </span>
      )}
      <input
        id={id}
        {...rest}
        className={`w-full rounded-md border border-border bg-surface px-3 py-2 text-sm text-text placeholder:text-muted/70
          transition-all duration-150
          hover:border-borderHi
          focus:border-primary focus:outline-none focus:ring-2 focus:ring-primary/20 ${className}`}
      />
      {hint && (
        <span className="mt-1 block font-mono text-[10px] text-muted">{hint}</span>
      )}
    </label>
  );
}
