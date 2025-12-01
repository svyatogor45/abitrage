import React from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import clsx from 'clsx';
import { statsApi } from '@/services/api';
import type { Stats } from '@/types';

/**
 * Обзор статистики
 */
export const StatsOverview: React.FC = () => {
  const queryClient = useQueryClient();

  const { data: stats, isLoading } = useQuery({
    queryKey: ['stats'],
    queryFn: statsApi.get,
    refetchInterval: 60000, // Обновление каждую минуту
  });

  const resetMutation = useMutation({
    mutationFn: statsApi.reset,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['stats'] });
    },
  });

  const formatPnl = (pnl: number) => {
    const formatted = Math.abs(pnl).toLocaleString('ru-RU', {
      minimumFractionDigits: 2,
      maximumFractionDigits: 2,
    });
    return pnl >= 0 ? `+${formatted}` : `-${formatted}`;
  };

  if (isLoading) {
    return (
      <div className="bg-surface border border-border rounded-lg p-6">
        <div className="animate-pulse space-y-4">
          <div className="h-6 bg-gray-700 rounded w-1/4"></div>
          <div className="grid grid-cols-3 gap-4">
            {[1, 2, 3].map((i) => (
              <div key={i} className="h-20 bg-gray-700 rounded"></div>
            ))}
          </div>
        </div>
      </div>
    );
  }

  if (!stats) {
    return (
      <div className="bg-surface border border-border rounded-lg p-6 text-center text-gray-400">
        Статистика недоступна
      </div>
    );
  }

  return (
    <div className="bg-surface border border-border rounded-lg p-6">
      <div className="flex items-center justify-between mb-6">
        <h2 className="text-xl font-bold text-white">Статистика</h2>
        <button
          onClick={() => {
            if (confirm('Сбросить всю статистику?')) {
              resetMutation.mutate();
            }
          }}
          disabled={resetMutation.isPending}
          className={clsx(
            'text-sm text-gray-400 hover:text-white transition-colors',
            resetMutation.isPending && 'opacity-50 cursor-not-allowed'
          )}
        >
          Сбросить
        </button>
      </div>

      {/* Основные метрики */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mb-6">
        {/* Количество сделок */}
        <StatCard
          title="Сделки"
          items={[
            { label: 'Сегодня', value: stats.todayTrades },
            { label: 'Неделя', value: stats.weekTrades },
            { label: 'Месяц', value: stats.monthTrades },
          ]}
        />

        {/* PNL */}
        <StatCard
          title="PNL (USDT)"
          items={[
            { label: 'Сегодня', value: formatPnl(stats.todayPnl), color: stats.todayPnl >= 0 ? 'green' : 'red' },
            { label: 'Неделя', value: formatPnl(stats.weekPnl), color: stats.weekPnl >= 0 ? 'green' : 'red' },
            { label: 'Месяц', value: formatPnl(stats.monthPnl), color: stats.monthPnl >= 0 ? 'green' : 'red' },
          ]}
        />

        {/* Stop Loss */}
        <StatCard
          title="Stop Loss"
          items={[
            { label: 'Сегодня', value: stats.stopLossStats.today },
            { label: 'Неделя', value: stats.stopLossStats.week },
            { label: 'Месяц', value: stats.stopLossStats.month },
          ]}
          warning={stats.stopLossStats.month > 0}
        />

        {/* Ликвидации */}
        <StatCard
          title="Ликвидации"
          items={[
            { label: 'Сегодня', value: stats.liquidationStats.today },
            { label: 'Неделя', value: stats.liquidationStats.week },
            { label: 'Месяц', value: stats.liquidationStats.month },
          ]}
          danger={stats.liquidationStats.month > 0}
        />
      </div>

      {/* Общие итоги */}
      <div className="grid grid-cols-2 gap-4 p-4 bg-gray-800/50 rounded-lg">
        <div>
          <div className="text-sm text-gray-400">Всего сделок</div>
          <div className="text-2xl font-bold text-white">{stats.totalTrades}</div>
        </div>
        <div>
          <div className="text-sm text-gray-400">Общий PNL</div>
          <div
            className={clsx(
              'text-2xl font-bold',
              stats.totalPnl >= 0 ? 'text-green-500' : 'text-red-500'
            )}
          >
            {formatPnl(stats.totalPnl)} USDT
          </div>
        </div>
      </div>
    </div>
  );
};

interface StatCardProps {
  title: string;
  items: Array<{
    label: string;
    value: number | string;
    color?: 'green' | 'red';
  }>;
  warning?: boolean;
  danger?: boolean;
}

const StatCard: React.FC<StatCardProps> = ({ title, items, warning, danger }) => {
  return (
    <div
      className={clsx(
        'p-4 rounded-lg',
        danger ? 'bg-red-500/10 border border-red-500/30' :
        warning ? 'bg-yellow-500/10 border border-yellow-500/30' :
        'bg-gray-800/50'
      )}
    >
      <h3 className="text-sm font-medium text-gray-400 mb-2">{title}</h3>
      <div className="space-y-1">
        {items.map((item, index) => (
          <div key={index} className="flex justify-between text-sm">
            <span className="text-gray-500">{item.label}:</span>
            <span
              className={clsx(
                'font-medium',
                item.color === 'green' ? 'text-green-500' :
                item.color === 'red' ? 'text-red-500' :
                'text-white'
              )}
            >
              {item.value}
            </span>
          </div>
        ))}
      </div>
    </div>
  );
};

export default StatsOverview;
