/** @type {import('tailwindcss').Config} */
export default {
  content: ["./index.html", "./src/**/*.{ts,tsx}"],
  theme: {
    extend: {
      colors: {
        // GitHub dark theme tokens
        bg: "#0d1117",
        panel: "#161b22",
        border: "#30363d",
        muted: "#7d8590",
        text: "#c9d1d9",
        primary: "#1f6feb",
        success: "#238636",
        danger: "#da3633",
      },
    },
  },
  plugins: [],
};
