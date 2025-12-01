import React, { useState } from 'react';
import clsx from 'clsx';
import type { ExchangeAccount, ExchangeName, ExchangeConnectRequest } from '@/types';
import { EXCHANGE_NAMES } from '@/types';

interface ExchangeCardProps {
  exchange: ExchangeAccount;
  onConnect: (name: ExchangeName, credentials: ExchangeConnectRequest) => Promise<void>;
  onDisconnect: (name: ExchangeName) => Promise<void>;
  onRefreshBalance: (name: ExchangeName) => Promise<void>;
  isConnecting: boolean;
  isDisconnecting: boolean;
}

/**
 * Карточка биржи
 */
export const ExchangeCard: React.FC<ExchangeCardProps> = ({
  exchange,
  onConnect,
  onDisconnect,
  onRefreshBalance,
  isConnecting,
  isDisconnecting,
}) => {
  const [isEditing, setIsEditing] = useState(false);
  const [apiKey, setApiKey] = useState('');
  const [secretKey, setSecretKey] = useState('');
  const [passphrase, setPassphrase] = useState('');
  const [error, setError] = useState('');

  const needsPassphrase = exchange.name === 'okx';

  const handleConnect = async () => {
    if (!apiKey.trim() || !secretKey.trim()) {
      setError('Заполните все обязательные поля');
      return;
    }

    if (needsPassphrase && !passphrase.trim()) {
      setError('Для OKX требуется Passphrase');
      return;
    }

    setError('');

    try {
      await onConnect(exchange.name, {
        apiKey: apiKey.trim(),
        secretKey: secretKey.trim(),
        passphrase: needsPassphrase ? passphrase.trim() : undefined,
      });
      setIsEditing(false);
      setApiKey('');
      setSecretKey('');
      setPassphrase('');
    } catch (err) {
      setError('Ошибка подключения. Проверьте ключи.');
    }
  };

  const handleDisconnect = async () => {
    if (confirm(`Отключить ${EXCHANGE_NAMES[exchange.name]}?`)) {
      await onDisconnect(exchange.name);
    }
  };

  const formatBalance = (balance: number) => {
    return balance.toLocaleString('ru-RU', {
      minimumFractionDigits: 2,
      maximumFractionDigits: 2,
    });
  };

  return (
    <div className="bg-surface border border-border rounded-lg p-4">
      {/* Заголовок */}
      <div className="flex items-center justify-between mb-4">
        <div className="flex items-center gap-3">
          {/* Логотип биржи (placeholder) */}
          <div className="w-10 h-10 bg-gray-700 rounded-lg flex items-center justify-center text-white font-bold text-sm">
            {exchange.name.slice(0, 2).toUpperCase()}
          </div>
          <div>
            <h3 className="text-lg font-bold text-white">
              {EXCHANGE_NAMES[exchange.name]}
            </h3>
            <div className="flex items-center gap-2">
              <div
                className={clsx(
                  'w-2 h-2 rounded-full',
                  exchange.connected ? 'bg-green-500' : 'bg-red-500'
                )}
              />
              <span className="text-xs text-gray-400">
                {exchange.connected ? 'Подключено' : 'Не подключено'}
              </span>
            </div>
          </div>
        </div>
      </div>

      {/* Баланс (если подключено) */}
      {exchange.connected && !isEditing && (
        <div className="mb-4">
          <div className="text-sm text-gray-400">Баланс:</div>
          <div className="text-xl font-bold text-white">
            {formatBalance(exchange.balance)} USDT
          </div>
          {exchange.lastError && (
            <div className="text-xs text-red-500 mt-1">{exchange.lastError}</div>
          )}
        </div>
      )}

      {/* Форма ключей */}
      {(isEditing || !exchange.connected) && (
        <div className="space-y-3 mb-4">
          <div>
            <label className="block text-xs text-gray-400 mb-1">API Key</label>
            <input
              type="password"
              value={apiKey}
              onChange={(e) => setApiKey(e.target.value)}
              placeholder="Введите API Key"
              className="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-white text-sm placeholder-gray-500"
            />
          </div>
          <div>
            <label className="block text-xs text-gray-400 mb-1">Secret Key</label>
            <input
              type="password"
              value={secretKey}
              onChange={(e) => setSecretKey(e.target.value)}
              placeholder="Введите Secret Key"
              className="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-white text-sm placeholder-gray-500"
            />
          </div>
          {needsPassphrase && (
            <div>
              <label className="block text-xs text-gray-400 mb-1">Passphrase</label>
              <input
                type="password"
                value={passphrase}
                onChange={(e) => setPassphrase(e.target.value)}
                placeholder="Введите Passphrase"
                className="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-white text-sm placeholder-gray-500"
              />
            </div>
          )}
          {error && <p className="text-red-500 text-xs">{error}</p>}
        </div>
      )}

      {/* Кнопки */}
      <div className="flex gap-2">
        {!exchange.connected ? (
          <button
            onClick={handleConnect}
            disabled={isConnecting}
            className={clsx(
              'flex-1 py-2 px-4 rounded-lg text-sm font-medium transition-colors',
              'bg-green-500/20 text-green-500 hover:bg-green-500/30',
              isConnecting && 'opacity-50 cursor-not-allowed'
            )}
          >
            {isConnecting ? 'Подключение...' : 'Подключить'}
          </button>
        ) : isEditing ? (
          <>
            <button
              onClick={() => setIsEditing(false)}
              className="flex-1 py-2 px-4 rounded-lg text-sm font-medium bg-gray-700 text-gray-300 hover:bg-gray-600 transition-colors"
            >
              Отмена
            </button>
            <button
              onClick={handleConnect}
              disabled={isConnecting}
              className={clsx(
                'flex-1 py-2 px-4 rounded-lg text-sm font-medium',
                'bg-primary text-white hover:bg-primary/80 transition-colors',
                isConnecting && 'opacity-50 cursor-not-allowed'
              )}
            >
              {isConnecting ? 'Сохранение...' : 'Сохранить'}
            </button>
          </>
        ) : (
          <>
            <button
              onClick={() => onRefreshBalance(exchange.name)}
              className="py-2 px-4 rounded-lg text-sm font-medium bg-gray-700 text-gray-300 hover:bg-gray-600 transition-colors"
              title="Обновить баланс"
            >
              <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"
                />
              </svg>
            </button>
            <button
              onClick={() => setIsEditing(true)}
              className="flex-1 py-2 px-4 rounded-lg text-sm font-medium bg-gray-700 text-gray-300 hover:bg-gray-600 transition-colors"
            >
              Изменить
            </button>
            <button
              onClick={handleDisconnect}
              disabled={isDisconnecting}
              className={clsx(
                'py-2 px-4 rounded-lg text-sm font-medium',
                'bg-red-500/20 text-red-500 hover:bg-red-500/30 transition-colors',
                isDisconnecting && 'opacity-50 cursor-not-allowed'
              )}
            >
              Отключить
            </button>
          </>
        )}
      </div>
    </div>
  );
};

export default ExchangeCard;
