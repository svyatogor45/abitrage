import React from 'react';
import { useExchanges } from '@/hooks/useExchanges';
import { ExchangeCard } from './ExchangeCard';
import type { ExchangeName } from '@/types';

// Порядок отображения бирж
const EXCHANGE_ORDER: ExchangeName[] = [
  'bybit',
  'bitget',
  'okx',
  'gate',
  'htx',
  'bingx',
];

/**
 * Список всех поддерживаемых бирж
 */
export const ExchangeList: React.FC = () => {
  const {
    exchanges,
    connectExchange,
    disconnectExchange,
    refreshBalance,
    isConnecting,
    isDisconnecting,
    canTrade,
  } = useExchanges();

  // Создаем полный список с учетом порядка
  const orderedExchanges = EXCHANGE_ORDER.map((name) => {
    const existing = exchanges.find((e) => e.name === name);
    return existing || {
      id: 0,
      name,
      connected: false,
      balance: 0,
      updatedAt: '',
      createdAt: '',
    };
  });

  return (
    <div>
      <div className="flex items-center justify-between mb-4">
        <h2 className="text-xl font-bold text-white">API Подключения</h2>
        {!canTrade && (
          <div className="text-sm text-yellow-500 flex items-center gap-2">
            <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
              />
            </svg>
            Подключите минимум 2 биржи для арбитража
          </div>
        )}
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
        {orderedExchanges.map((exchange) => (
          <ExchangeCard
            key={exchange.name}
            exchange={exchange}
            onConnect={connectExchange}
            onDisconnect={disconnectExchange}
            onRefreshBalance={refreshBalance}
            isConnecting={isConnecting}
            isDisconnecting={isDisconnecting}
          />
        ))}
      </div>
    </div>
  );
};

export default ExchangeList;
