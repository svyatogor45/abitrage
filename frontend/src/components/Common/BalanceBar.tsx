import React from 'react';
import clsx from 'clsx';
import { useExchanges } from '@/hooks/useExchanges';
import { EXCHANGE_NAMES, type ExchangeName } from '@/types';

interface BalanceBarProps {
  className?: string;
}

/**
 * Панель с балансами бирж
 */
export const BalanceBar: React.FC<BalanceBarProps> = ({ className }) => {
  const {
    connectedExchanges,
    totalBalance,
    isLoading,
    refreshAllBalances,
    isRefreshing
  } = useExchanges();

  const formatBalance = (balance: number): string => {
    return balance.toLocaleString('ru-RU', {
      minimumFractionDigits: 2,
      maximumFractionDigits: 2,
    });
  };

  if (connectedExchanges.length === 0) {
    return (
      <div className={clsx('bg-surface border border-border rounded-lg p-4', className)}>
        <div className="text-center text-gray-400">
          <p>Нет подключенных бирж</p>
          <p className="text-sm mt-1">
            Перейдите во вкладку API/STATS для подключения
          </p>
        </div>
      </div>
    );
  }

  return (
    <div className={clsx('bg-surface border border-border rounded-lg p-4', className)}>
      <div className="flex items-center justify-between mb-3">
        <h3 className="text-sm font-medium text-gray-400">Балансы бирж</h3>
        <button
          onClick={() => refreshAllBalances()}
          disabled={isRefreshing}
          className={clsx(
            'text-sm text-primary hover:text-primary/80 transition-colors',
            isRefreshing && 'opacity-50 cursor-not-allowed'
          )}
        >
          {isRefreshing ? (
            <span className="flex items-center gap-1">
              <svg className="animate-spin h-4 w-4" viewBox="0 0 24 24">
                <circle
                  className="opacity-25"
                  cx="12"
                  cy="12"
                  r="10"
                  stroke="currentColor"
                  strokeWidth="4"
                  fill="none"
                />
                <path
                  className="opacity-75"
                  fill="currentColor"
                  d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
                />
              </svg>
              Обновление...
            </span>
          ) : (
            'Обновить'
          )}
        </button>
      </div>

      {/* Список балансов */}
      <div className="flex flex-wrap gap-4">
        {connectedExchanges.map((exchange) => (
          <ExchangeBalance
            key={exchange.name}
            name={exchange.name}
            balance={exchange.balance}
            hasError={!!exchange.lastError}
          />
        ))}
      </div>

      {/* Общий баланс */}
      <div className="mt-4 pt-3 border-t border-border flex justify-between items-center">
        <span className="text-sm text-gray-400">Общий баланс:</span>
        <span className="text-lg font-bold text-white">
          {formatBalance(totalBalance)} USDT
        </span>
      </div>
    </div>
  );
};

interface ExchangeBalanceProps {
  name: ExchangeName;
  balance: number;
  hasError?: boolean;
}

const ExchangeBalance: React.FC<ExchangeBalanceProps> = ({
  name,
  balance,
  hasError,
}) => {
  const formatBalance = (balance: number): string => {
    return balance.toLocaleString('ru-RU', {
      minimumFractionDigits: 2,
      maximumFractionDigits: 2,
    });
  };

  return (
    <div
      className={clsx(
        'flex items-center gap-2 px-3 py-2 rounded-lg',
        hasError ? 'bg-red-500/10' : 'bg-gray-800/50'
      )}
    >
      {/* Иконка статуса */}
      <div
        className={clsx(
          'w-2 h-2 rounded-full',
          hasError ? 'bg-red-500' : 'bg-green-500'
        )}
      />

      {/* Название биржи */}
      <span className="text-sm font-medium text-gray-300">
        {EXCHANGE_NAMES[name]}:
      </span>

      {/* Баланс */}
      <span
        className={clsx(
          'text-sm font-bold',
          hasError ? 'text-red-400' : 'text-white'
        )}
      >
        {hasError ? 'Ошибка' : `${formatBalance(balance)} USDT`}
      </span>
    </div>
  );
};

export default BalanceBar;
