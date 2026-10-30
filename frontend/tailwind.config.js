/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{ts,tsx}'],
  theme: {
    extend: {
      colors: {
        brand: {
          50: '#eef4ff',
          100: '#d9e5ff',
          500: '#3b63f6',
          600: '#2b4fe0',
          700: '#2340b8',
        },
      },
    },
  },
  plugins: [],
}
