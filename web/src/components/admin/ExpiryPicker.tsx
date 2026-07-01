import { useState } from "react";

// Resolves the picker's choice into an RFC3339 UTC string, or null = never.
export type ExpiryValue = string | null;

interface Preset {
  label: string;
  ms: number | null; // null = never
}

const PRESETS: Preset[] = [
  { label: "1h", ms: 3_600_000 },
  { label: "1d", ms: 86_400_000 },
  { label: "7d", ms: 604_800_000 },
  { label: "30d", ms: 2_592_000_000 },
  { label: "Never", ms: null },
];

function toRFC3339UTC(ms: number): string {
  return new Date(ms).toISOString();
}

function toDatetimeLocal(ms: number): string {
  // datetime-local expects local time in yyyy-MM-ddTHH:mm (no timezone).
  const d = new Date(ms);
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(
    d.getHours()
  )}:${pad(d.getMinutes())}`;
}

interface ExpiryPickerProps {
  value: ExpiryValue;
  onChange: (v: ExpiryValue) => void;
}

export function ExpiryPicker({ value, onChange }: ExpiryPickerProps) {
  const [custom, setCustom] = useState(false);

  // Which preset is active? null if none (custom mode or unset).
  const activePreset = custom
    ? null
    : PRESETS.find((p) => {
        if (p.ms === null) return value === null;
        if (value === null) return false;
        // match by hour-granularity to tolerate clock skew
        return Math.abs(new Date(value).getTime() - (Date.now() + p.ms)) < 60_000;
      }) ?? null;

  const applyPreset = (p: Preset) => {
    setCustom(false);
    onChange(p.ms === null ? null : toRFC3339UTC(Date.now() + p.ms));
  };

  return (
    <div>
      <span className="mb-1.5 block font-mono text-[10px] uppercase tracking-wider text-muted">
        Expires
      </span>
      <div className="flex flex-wrap gap-1.5">
        {PRESETS.map((p) => {
          const active = activePreset?.label === p.label;
          return (
            <button
              key={p.label}
              type="button"
              onClick={() => applyPreset(p)}
              className={`rounded-md border px-2.5 py-1 font-mono text-xs transition-all duration-150 active:scale-95
                ${
                  active
                    ? "border-primary bg-primaryDim text-primaryHi"
                    : "border-border bg-surface text-subtext hover:border-borderHi hover:text-text"
                }`}
            >
              {p.label}
            </button>
          );
        })}
        <button
          type="button"
          onClick={() => {
            setCustom(true);
            // default custom to 24h ahead when first opened
            if (!custom && value === null) {
              onChange(toRFC3339UTC(Date.now() + 86_400_000));
            }
          }}
          className={`rounded-md border px-2.5 py-1 font-mono text-xs transition-all duration-150 active:scale-95
            ${
              custom
                ? "border-primary bg-primaryDim text-primaryHi"
                : "border-border bg-surface text-subtext hover:border-borderHi hover:text-text"
            }`}
        >
          Custom
        </button>
      </div>

      {custom && (
        <input
          type="datetime-local"
          value={value ? toDatetimeLocal(new Date(value).getTime()) : ""}
          onChange={(e) => {
            if (!e.target.value) {
              onChange(null);
              return;
            }
            // datetime-local is local wall-clock; convert to UTC RFC3339.
            const local = new Date(e.target.value);
            if (!isNaN(local.getTime())) {
              onChange(local.toISOString());
            }
          }}
          className="mt-2 w-full rounded-md border border-border bg-surface px-3 py-2 text-sm text-text
            transition-all duration-150 hover:border-borderHi
            focus:border-primary focus:outline-none focus:ring-2 focus:ring-primary/20"
        />
      )}

      <span className="mt-1.5 block font-mono text-[10px] text-muted">
        {value === null
          ? "Token never expires"
          : `Expires ${new Date(value).toUTCString()}`}
      </span>
    </div>
  );
}
