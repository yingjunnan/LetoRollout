import type { ButtonHTMLAttributes, ReactNode } from "react";

type Variant = "primary" | "secondary" | "danger" | "ghost";
type Size = "sm" | "md";

const base =
  "relative inline-flex items-center justify-center gap-1.5 rounded-md font-medium " +
  "transition-all duration-150 active:scale-[0.97] disabled:opacity-40 disabled:cursor-not-allowed " +
  "disabled:active:scale-100 select-none focus-visible:ring-2 focus-visible:ring-primary/40";

const sizes: Record<Size, string> = {
  sm: "px-2.5 py-1 text-xs",
  md: "px-3.5 py-1.5 text-sm",
};

const variants: Record<Variant, string> = {
  // Prominent solid teal — the primary action always reads first.
  primary:
    "bg-primary text-[#06120f] hover:bg-primaryHi shadow-[0_0_0_1px_rgba(45,212,191,0.4),0_4px_14px_-4px_rgba(45,212,191,0.5)]",
  secondary:
    "bg-panelHi text-text border border-borderHi hover:border-primary/50 hover:text-white",
  danger:
    "bg-danger/15 text-dangerHi border border-danger/40 hover:bg-danger/25 hover:border-danger/60",
  ghost: "bg-transparent text-subtext hover:text-text hover:bg-panelHi",
};

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: Variant;
  size?: Size;
  loading?: boolean;
  children: ReactNode;
}

export function Button({
  variant = "secondary",
  size = "md",
  loading = false,
  disabled,
  className = "",
  children,
  ...rest
}: ButtonProps) {
  return (
    <button
      {...rest}
      disabled={disabled || loading}
      className={`${base} ${sizes[size]} ${variants[variant]} ${className}`}
    >
      {loading && (
        <svg
          className="h-3.5 w-3.5 animate-spin"
          viewBox="0 0 24 24"
          fill="none"
          aria-hidden
        >
          <circle
            className="opacity-25"
            cx="12"
            cy="12"
            r="10"
            stroke="currentColor"
            strokeWidth="3"
          />
          <path
            className="opacity-90"
            fill="currentColor"
            d="M12 2a10 10 0 0 1 10 10h-3a7 7 0 0 0-7-7V2z"
          />
        </svg>
      )}
      {children}
    </button>
  );
}
