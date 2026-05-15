/** @type {import('tailwindcss').Config} */
module.exports = {
  plugins: [
    require("@tailwindcss/typography"),
    require("tailwindcss-safe-area"),
  ],
  presets: [require("tailwindcss-preset-email")],
  content: [
    "./components/**/*.html",
    "./emails/**/*.html",
    "./layouts/**/*.html",
  ],
  theme: {
    fontFamily: {
      sans: ["Inter", "sans-serif"],
      serif: ["TexGyrePagella", "serif"],
      display: ["Nunito", "sans-serif"],
    },
    colors: {
      "base-100": "#FCFAF7",
      "base-200": "#EEEAE1",
      "base-300": "#D6CFC1",
      "base-content": "#312824",
      primary: "#4B6553",
      "primary-content": "#FCFAF7",
      secondary: "#643F29",
      "secondary-content": "#FCFAF7",
      accent: "#73384A",
      "accent-content": "#FCFAF7",
      neutral: "#4B433B",
      "neutral-content": "#FCFAF7",
      info: "#1A5DAA",
      "info-content": "#FCFAF7",
      success: "#00711C",
      "success-content": "#FCFAF7",
      warning: "#FABE29",
      "warning-content": "#312824",
      error: "#AD3037",
      "error-content": "#FCFAF7",
    },
  },
};
