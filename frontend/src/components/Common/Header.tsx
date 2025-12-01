import React from 'react';
import { useWebSocket } from '@/hooks/useWebSocket';

interface HeaderProps {
  title?: string;
}

/**
 * Шапка приложения
 */
export const Header: React.FC<HeaderProps> = ({
  title = 'Arbitrage Terminal'
}) => {
  const { isConnected, status } = useWebSocket();

  const getStatusColor = () => {
    switch (status) {
      case 'connected':
        return 'bg-green-500';
      case 'connecting':
        return 'bg-yellow-500 animate-pulse';
      case 'error':
        return 'bg-red-500';
      default:
        return 'bg-gray-500';
    }
  };

  const getStatusText = () => {
    switch (status) {
      case 'connected':
        return 'Подключено';
      case 'connecting':
        return 'Подключение...';
      case 'error':
        return 'Ошибка';
      default:
        return 'Отключено';
    }
  };

  return (
    <header className="bg-surface border-b border-border px-6 py-4">
      <div className="flex items-center justify-between">
        {/* Логотип и название */}
        <div className="flex items-center gap-3">
          <div className="w-10 h-10 bg-primary rounded-lg flex items-center justify-center">
            <svg
              className="w-6 h-6 text-white"
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M13 7h8m0 0v8m0-8l-8 8-4-4-6 6"
              />
            </svg>
          </div>
          <h1 className="text-xl font-bold text-white">{title}</h1>
        </div>

        {/* Статус подключения */}
        <div className="flex items-center gap-4">
          {/* Индикатор WebSocket */}
          <div className="flex items-center gap-2">
            <div className={`w-2 h-2 rounded-full ${getStatusColor()}`} />
            <span className="text-sm text-gray-400">{getStatusText()}</span>
          </div>

          {/* Время */}
          <div className="text-sm text-gray-400">
            <CurrentTime />
          </div>
        </div>
      </div>
    </header>
  );
};

/**
 * Компонент текущего времени
 */
const CurrentTime: React.FC = () => {
  const [time, setTime] = React.useState(new Date());

  React.useEffect(() => {
    const timer = setInterval(() => {
      setTime(new Date());
    }, 1000);

    return () => clearInterval(timer);
  }, []);

  return (
    <span>
      {time.toLocaleTimeString('ru-RU', {
        hour: '2-digit',
        minute: '2-digit',
        second: '2-digit',
      })}
    </span>
  );
};

export default Header;
