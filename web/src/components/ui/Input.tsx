import type { InputHTMLAttributes } from "react";

interface InputProps extends InputHTMLAttributes<HTMLInputElement> {
  label?: string;
  hint?: string;
}

export function Input({ label, hint, id, className = "", ...rest }: InputProps) {
  return (
    <label className="block">
      {label && (
        <span className="block text-xs text-muted mb-1">{label}</span>
      )}
      <input
        id={id}
        {...rest}
        className={`w-full bg-bg border border-border rounded-md px-3 py-1.5 text-sm text-text placeholder-muted focus:outline-none focus:border-primary ${className}`}
      />
      {hint && <span className="block text-xs text-muted mt-1">{hint}</span>}
    </label>
  );
}
