import React from 'react';
import clsx from 'clsx';
import type { PairWithRuntime } from '@/types';
import { EXCHANGE_NAMES, PAIR_STATE_LABELS } from '@/types';

interface PairCardProps {
  pair: PairWithRuntime;
  onStart: (id: number) => void;
  onPause: (id: number) => void;
  onEdit: (pair: PairWithRuntime) => void;
  onDelete: (id: number) => void;
  isStarting?: boolean;
  isPausing?: boolean;
}

/**
 * Карточка торговой пары
 */
export const PairCard: React.FC<PairCardProps> = ({
  pair,
  onStart,
  onPause,
  onEdit,
  onDelete,
  isStarting,
  isPausing,
}) => {
  const runtime = pair.runtime;
  const state = runtime?.state || (pair.status === 'paused' ? 'PAUSED' : 'READY');

  // Определение цвета рамки
  const getBorderColor = () => {
    if (state === 'HOLDING' || state === 'ENTERING' || state === 'EXITING') {
      return 'border-status-active'; // Зеленый - активный арбитраж
    }
    if (state === 'PAUSED') {
      return 'border-status-paused'; // Оранжевый - пауза
    }
    return 'border-gray-600'; // Белый/нейтральный - ожидание
  };

  // Форматирование чисел
  const formatNumber = (value: number, decimals = 2) => {
    return value.toLocaleString('ru-RU', {
      minimumFractionDigits: decimals,
      maximumFractionDigits: decimals,
    });
  };

  const formatPnl = (pnl: number) => {
    const formatted = formatNumber(Math.abs(pnl));
    const sign = pnl >= 0 ? '+' : '-';
    return `${sign}${formatted}`;
  };

  const hasOpenPosition = state === 'HOLDING' || state === 'ENTERING' || state === 'EXITING';

  return (
    <div
      className={clsx(
        'bg-surface rounded-lg border-2 p-4 transition-all',
        getBorderColor()
      )}
    >
      {/* Заголовок */}
      <div className="flex justify-between items-start mb-3">
        <div>
          <h3 className="text-lg font-bold text-white">{pair.symbol}</h3>
          <p className="text-xs text-gray-500">{PAIR_STATE_LABELS[state]}</p>
        </div>
        {hasOpenPosition && runtime && (
          <div
            className={clsx(
              'text-lg font-bold',
              runtime.unrealizedPnl >= 0 ? 'text-green-500' : 'text-red-500'
            )}
          >
            {formatPnl(runtime.unrealizedPnl)} USDT
          </div>
        )}
      </div>

      {/* Параметры пары */}
      <div className="text-xs text-gray-400 mb-3 flex flex-wrap gap-x-3 gap-y-1">
        <span>Р.О: {formatNumber(pair.volume, 4)} {pair.base}</span>
        <span>С.В: {formatNumber(pair.entrySpread, 2)}%</span>
        <span>С.ВЫ: {formatNumber(pair.exitSpread, 2)}%</span>
        <span>N.ОР: {pair.nOrders}</span>
        <span>SL: {pair.stopLoss ? `${formatNumber(pair.stopLoss)} USDT` : '—'}</span>
      </div>

      {/* Состояние позиции */}
      <div className="min-h-[80px] mb-3">
        {hasOpenPosition && runtime?.legs && runtime.legs.length === 2 ? (
          <PositionInfo legs={runtime.legs} currentSpread={runtime.currentSpread} />
        ) : state === 'PAUSED' ? (
          <div className="text-center text-gray-500 py-4">
            Работа торговой пары приостановлена
          </div>
        ) : (
          <div className="text-center text-gray-500 py-4">
            Торговая пара запущена (ожидание условий)
          </div>
        )}
      </div>

      {/* Статистика пары */}
      <div className="text-xs text-gray-400 border-t border-border pt-3 mb-3">
        <span>N сделок: {pair.tradesCount}</span>
        <span className="mx-2">|</span>
        <span
          className={clsx(
            pair.totalPnl >= 0 ? 'text-green-500' : 'text-red-500'
          )}
        >
          PNL: {formatPnl(pair.totalPnl)} USDT
        </span>
      </div>

      {/* Кнопки управления */}
      <div className="flex gap-2">
        {/* Старт/Пауза */}
        {state === 'PAUSED' ? (
          <button
            onClick={() => onStart(pair.id)}
            disabled={isStarting}
            className={clsx(
              'flex-1 py-2 px-3 rounded-lg text-sm font-medium transition-colors',
              'bg-green-500/20 text-green-500 hover:bg-green-500/30',
              isStarting && 'opacity-50 cursor-not-allowed'
            )}
          >
            {isStarting ? 'Запуск...' : 'Старт'}
          </button>
        ) : (
          <button
            onClick={() => onPause(pair.id)}
            disabled={isPausing}
            className={clsx(
              'flex-1 py-2 px-3 rounded-lg text-sm font-medium transition-colors',
              'bg-orange-500/20 text-orange-500 hover:bg-orange-500/30',
              isPausing && 'opacity-50 cursor-not-allowed'
            )}
          >
            {isPausing ? 'Остановка...' : 'Пауза'}
          </button>
        )}

        {/* Редактировать */}
        <button
          onClick={() => onEdit(pair)}
          className="py-2 px-3 rounded-lg text-sm font-medium transition-colors bg-gray-700 text-gray-300 hover:bg-gray-600"
        >
          <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={2}
              d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z"
            />
          </svg>
        </button>

        {/* Удалить */}
        <button
          onClick={() => onDelete(pair.id)}
          disabled={hasOpenPosition}
          className={clsx(
            'py-2 px-3 rounded-lg text-sm font-medium transition-colors',
            'bg-red-500/20 text-red-500 hover:bg-red-500/30',
            hasOpenPosition && 'opacity-50 cursor-not-allowed'
          )}
          title={hasOpenPosition ? 'Нельзя удалить пару с открытой позицией' : 'Удалить'}
        >
          <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={2}
              d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"
            />
          </svg>
        </button>
      </div>
    </div>
  );
};

interface PositionInfoProps {
  legs: PairWithRuntime['runtime']['legs'];
  currentSpread: number;
}

const PositionInfo: React.FC<PositionInfoProps> = ({ legs, currentSpread }) => {
  const formatNumber = (value: number, decimals = 2) => {
    return value.toLocaleString('ru-RU', {
      minimumFractionDigits: decimals,
      maximumFractionDigits: decimals,
    });
  };

  const formatPnl = (pnl: number) => {
    const formatted = formatNumber(Math.abs(pnl));
    const sign = pnl >= 0 ? '+' : '-';
    return `${sign}${formatted}`;
  };

  return (
    <div className="space-y-2">
      {legs.map((leg, index) => (
        <div key={index} className="flex justify-between items-center text-sm">
          <div className="flex items-center gap-2">
            <span className="text-gray-400">{EXCHANGE_NAMES[leg.exchange]}</span>
            <span
              className={clsx(
                'text-xs px-1.5 py-0.5 rounded',
                leg.side === 'long' ? 'bg-green-500/20 text-green-500' : 'bg-red-500/20 text-red-500'
              )}
            >
              {leg.side.toUpperCase()}
            </span>
          </div>
          <div className="text-right">
            <div className="text-white">${formatNumber(leg.currentPrice)}</div>
            <div
              className={clsx(
                'text-xs',
                leg.unrealizedPnl >= 0 ? 'text-green-500' : 'text-red-500'
              )}
            >
              {formatPnl(leg.unrealizedPnl)}
            </div>
          </div>
        </div>
      ))}
      <div className="text-center text-xs text-gray-400 pt-1">
        Текущий спред: {formatNumber(currentSpread, 3)}%
      </div>
    </div>
  );
};

export default PairCard;
