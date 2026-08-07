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
        // Base - deep space-slate, layered surfaces
        bg: "#080B11",
        surface: "#0F141C",
        panel: "#151C26",
        panelHi: "#1C2530",
        // Lines & text
        border: "#222C3A",
        borderHi: "#2F3B4C",
        muted: "#566273",
        subtext: "#8A95A5",
        text: "#DCE3EC",
        // Signal accents - each carries a semantic job from the rollout domain.
        primary: "#2DD4BF", // teal - ready / go / brand
        primaryHi: "#5EEAD4",
        primaryDim: "#0E3A36",
        aurora: "#38BDF8", // sky - data / streams / links
        auroraHi: "#7DD3FC",
        auroraDim: "#0B3145",
        amber: "#F5B14E", // amber - rolling / in-progress
        amberHi: "#FBCB6B",
        amberDim: "#3A2A12",
        success: "#34D399",
        danger: "#F87171",
        dangerHi: "#FCA5A5",
        warning: "#FBBF24",
        info: "#60A5FA",
      },
      boxShadow: {
        glow:
          "0 0 0 1px rgba(45,212,191,0.25), 0 0 24px -6px rgba(45,212,191,0.45)",
        glowAurora:
          "0 0 0 1px rgba(56,189,248,0.22), 0 0 22px -6px rgba(56,189,248,0.40)",
        panel:
          "0 1px 0 0 rgba(255,255,255,0.025), 0 10px 32px -14px rgba(0,0,0,0.7)",
        inset: "inset 0 1px 0 0 rgba(255,255,255,0.035)",
        pod: "0 0 8px 0 rgba(45,212,191,0.55)",
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
          "50%": { opacity: "0.35" },
        },
        shimmer: {
          "0%": { backgroundPosition: "-200% 0" },
          "100%": { backgroundPosition: "200% 0" },
        },
      },
      animation: {
        "fade-up": "fade-up 0.3s cubic-bezier(0.22,1,0.36,1) both",
        "slide-in": "slide-in 0.25s cubic-bezier(0.22,1,0.36,1) both",
        "pulse-ring": "pulse-ring 1.6s ease-in-out infinite",
        shimmer: "shimmer 2.2s linear infinite",
      },
    },
  },
  plugins: [],
};
