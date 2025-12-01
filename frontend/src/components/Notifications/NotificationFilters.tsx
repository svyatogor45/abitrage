import React from 'react';
import clsx from 'clsx';
import type { NotificationFilters as FiltersType } from '@/types';
import { NOTIFICATION_TYPE_LABELS } from '@/types';

interface NotificationFiltersProps {
  filters: FiltersType;
  onToggle: (key: keyof FiltersType) => void;
  onEnableAll: () => void;
  onDisableAll: () => void;
}

const FILTER_ITEMS: Array<{
  key: keyof FiltersType;
  label: string;
  color: string;
}> = [
  { key: 'open', label: 'Открытие сделки', color: 'bg-green-500' },
  { key: 'close', label: 'Закрытие сделки', color: 'bg-green-500' },
  { key: 'stopLoss', label: 'Stop Loss', color: 'bg-yellow-500' },
  { key: 'liquidation', label: 'Ликвидация', color: 'bg-red-500' },
  { key: 'apiError', label: 'Ошибка API', color: 'bg-red-500' },
  { key: 'margin', label: 'Недостаток маржи', color: 'bg-yellow-500' },
  { key: 'pause', label: 'Пауза/Остановка', color: 'bg-orange-500' },
  { key: 'secondLegFail', label: 'Вторая нога не открыта', color: 'bg-red-500' },
];

/**
 * Фильтры уведомлений
 */
export const NotificationFilters: React.FC<NotificationFiltersProps> = ({
  filters,
  onToggle,
  onEnableAll,
  onDisableAll,
}) => {
  const enabledCount = Object.values(filters).filter(Boolean).length;
  const allEnabled = enabledCount === FILTER_ITEMS.length;
  const noneEnabled = enabledCount === 0;

  return (
    <div className="bg-surface border border-border rounded-lg p-4">
      <div className="flex items-center justify-between mb-4">
        <h3 className="text-sm font-medium text-white">Фильтры</h3>
        <div className="flex gap-2">
          <button
            onClick={onEnableAll}
            disabled={allEnabled}
            className={clsx(
              'text-xs text-primary hover:text-primary/80 transition-colors',
              allEnabled && 'opacity-50 cursor-not-allowed'
            )}
          >
            Все
          </button>
          <span className="text-gray-600">|</span>
          <button
            onClick={onDisableAll}
            disabled={noneEnabled}
            className={clsx(
              'text-xs text-gray-400 hover:text-white transition-colors',
              noneEnabled && 'opacity-50 cursor-not-allowed'
            )}
          >
            Ничего
          </button>
        </div>
      </div>

      <div className="flex flex-wrap gap-2">
        {FILTER_ITEMS.map((item) => (
          <FilterChip
            key={item.key}
            label={item.label}
            color={item.color}
            isActive={filters[item.key]}
            onClick={() => onToggle(item.key)}
          />
        ))}
      </div>
    </div>
  );
};

interface FilterChipProps {
  label: string;
  color: string;
  isActive: boolean;
  onClick: () => void;
}

const FilterChip: React.FC<FilterChipProps> = ({
  label,
  color,
  isActive,
  onClick,
}) => {
  return (
    <button
      onClick={onClick}
      className={clsx(
        'flex items-center gap-2 px-3 py-1.5 rounded-full text-sm transition-colors',
        isActive
          ? 'bg-gray-700 text-white'
          : 'bg-gray-800/50 text-gray-500 hover:bg-gray-800'
      )}
    >
      <div
        className={clsx(
          'w-2 h-2 rounded-full',
          isActive ? color : 'bg-gray-600'
        )}
      />
      {label}
    </button>
  );
};

export default NotificationFilters;
