import type { SelectHTMLAttributes, ReactNode } from "react";

interface SelectProps extends SelectHTMLAttributes<HTMLSelectElement> {
  label?: string;
  children: ReactNode;
}

export function Select({ label, id, className = "", children, ...rest }: SelectProps) {
  return (
    <label className="block">
      {label && (
        <span className="mb-1.5 block font-mono text-[10px] uppercase tracking-wider text-muted">
          {label}
        </span>
      )}
      <div className="relative">
        <select
          id={id}
          {...rest}
          className={`w-full appearance-none rounded-md border border-border bg-surface px-3 py-2 pr-8 text-sm text-text
            transition-all duration-150
            hover:border-borderHi
            focus:border-primary focus:outline-none focus:ring-2 focus:ring-primary/20 ${className}`}
        >
          {children}
        </select>
        <svg
          className="pointer-events-none absolute right-2.5 top-1/2 h-4 w-4 -translate-y-1/2 text-muted"
          viewBox="0 0 20 20"
          fill="currentColor"
          aria-hidden
        >
          <path
            fillRule="evenodd"
            d="M5.23 7.21a.75.75 0 0 1 1.06.02L10 11.06l3.71-3.83a.75.75 0 1 1 1.08 1.04l-4.25 4.39a.75.75 0 0 1-1.08 0L5.21 8.27a.75.75 0 0 1 .02-1.06z"
            clipRule="evenodd"
          />
        </svg>
      </div>
    </label>
  );
}
