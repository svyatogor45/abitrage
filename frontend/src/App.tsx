import React from 'react';
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { Header } from '@/components/Common/Header';
import { Navigation } from '@/components/Common/Navigation';
import { BotPage } from '@/pages/BotPage';
import { APIStatsPage } from '@/pages/APIStatsPage';
import { NotificationsPage } from '@/pages/NotificationsPage';
import { useWebSocket } from '@/hooks/useWebSocket';

// Create a client
const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      refetchOnWindowFocus: false,
      retry: 1,
      staleTime: 30000,
    },
  },
});

/**
 * WebSocket Provider Component
 * Инициализирует WebSocket соединение
 */
const WebSocketProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  // Инициализируем WebSocket соединение
  useWebSocket();
  return <>{children}</>;
};

/**
 * Основной Layout приложения
 */
const AppLayout: React.FC = () => {
  return (
    <div className="min-h-screen bg-background text-white flex flex-col">
      {/* Шапка */}
      <Header />

      {/* Основной контент */}
      <main className="flex-1 overflow-auto">
        <Routes>
          <Route path="/" element={<Navigate to="/bot" replace />} />
          <Route path="/bot" element={<BotPage />} />
          <Route path="/api-stats" element={<APIStatsPage />} />
          <Route path="/notifications" element={<NotificationsPage />} />
          <Route path="*" element={<Navigate to="/bot" replace />} />
        </Routes>
      </main>

      {/* Навигация (внизу) */}
      <Navigation />
    </div>
  );
};

/**
 * Корневой компонент приложения
 */
const App: React.FC = () => {
  return (
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <WebSocketProvider>
          <AppLayout />
        </WebSocketProvider>
      </BrowserRouter>
    </QueryClientProvider>
  );
};

export default App;
