/** @type {import('tailwindcss').Config} */
module.exports = {
  presets: [require("tailwindcss-preset-email")],
  content: [
    "./components/**/*.html",
    "./emails/**/*.html",
    "./layouts/**/*.html",
  ],
  theme: {
    colors: {
      "base-100": "#FCFAF7",
      "base-200": "#F5F3EC",
      "base-300": "#EFE9DE",
      "base-content": "#312824",
      primary: "#7B9985",
      "primary-content": "#FFFFFF",
      secondary: "#E5B79B",
      "secondary-content": "#312824",
      accent: "#EDA0B5",
      "accent-content": "#312824",
    },
  },
};
