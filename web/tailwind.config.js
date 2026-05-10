/** @type {import('tailwindcss').Config} */
export default {
  content: [
    "./index.html",
    "./src/**/*.{js,ts,jsx,tsx}",
  ],
  theme: {
    extend: {
      colors: {
        primary: {
          DEFAULT: '#1E40AF', // blue-800
        },
        accent: {
          DEFAULT: '#0EA5E9', // sky-500
        },
      },
      animation: {
        slideInRight: 'slideInRight 0.3s ease-out',
      },
      keyframes: {
        slideInRight: {
          from: { transform: 'translateX(100%)', opacity: '0' },
          to: { transform: 'translateX(0)', opacity: '1' },
        },
      },
    },
  },
  plugins: [],
}
