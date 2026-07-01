/** @type {import('tailwindcss').Config} */
export default {
  content: ["./index.html", "./src/**/*.{ts,tsx}"],
  theme: {
    extend: {
      fontFamily: {
        sans: ['"Inter Tight"', "ui-sans-serif", "system-ui", "sans-serif"],
        mono: ['"JetBrains Mono"', "ui-monospace", "SFMono-Regular", "monospace"],
      },
      colors: {
        // Base — deep slate-black, layered surfaces
        bg: "#0a0e14",
        surface: "#0f141b",
        panel: "#141b24",
        panelHi: "#1a222d",
        // Lines & text
        border: "#222b36",
        borderHi: "#2f3a47",
        muted: "#5b6776",
        subtext: "#8b96a5",
        text: "#d6dde6",
        // Signal accents — one confident primary + supporting roles
        primary: "#2dd4bf", // teal — "go / active"
        primaryHi: "#5eead4",
        primaryDim: "#134e4a",
        success: "#34d399",
        danger: "#f87171",
        dangerHi: "#fca5a5",
        warning: "#fbbf24",
        info: "#60a5fa",
      },
      boxShadow: {
        glow: "0 0 0 1px rgba(45,212,191,0.25), 0 0 24px -6px rgba(45,212,191,0.45)",
        panel: "0 1px 0 0 rgba(255,255,255,0.02), 0 8px 30px -12px rgba(0,0,0,0.6)",
        inset: "inset 0 1px 0 0 rgba(255,255,255,0.03)",
      },
      keyframes: {
        "fade-up": {
          "0%": { opacity: "0", transform: "translateY(6px)" },
          "100%": { opacity: "1", transform: "translateY(0)" },
        },
        "slide-in": {
          "0%": { opacity: "0", transform: "translateX(16px)" },
          "100%": { opacity: "1", transform: "translateX(0)" },
        },
        "pulse-ring": {
          "0%, 100%": { opacity: "1" },
          "50%": { opacity: "0.4" },
        },
      },
      animation: {
        "fade-up": "fade-up 0.3s cubic-bezier(0.22,1,0.36,1) both",
        "slide-in": "slide-in 0.25s cubic-bezier(0.22,1,0.36,1) both",
        "pulse-ring": "pulse-ring 1.6s ease-in-out infinite",
      },
    },
  },
  plugins: [],
};
