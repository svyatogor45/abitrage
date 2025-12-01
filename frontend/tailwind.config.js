/** @type {import('tailwindcss').Config} */
export default {
  content: [
    "./index.html",
    "./src/**/*.{js,ts,jsx,tsx}",
  ],
  theme: {
    extend: {
      colors: {
        // Цвета статусов карточек пар
        'status-active': '#22c55e',      // Зеленый - активный арбитраж
        'status-paused': '#f97316',      // Оранжевый - пауза
        'status-waiting': '#ffffff',     // Белый - ожидание

        // Цвета уведомлений
        'notify-success': '#22c55e',     // Зеленый
        'notify-warning': '#eab308',     // Желтый
        'notify-error': '#ef4444',       // Красный
        'notify-info': '#3b82f6',        // Синий

        // Основные цвета интерфейса
        'primary': '#3b82f6',
        'secondary': '#6b7280',
        'background': '#111827',
        'surface': '#1f2937',
        'border': '#374151',
      },
    },
  },
  plugins: [],
};
