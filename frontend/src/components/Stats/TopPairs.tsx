import React from 'react';
import { useQuery } from '@tanstack/react-query';
import clsx from 'clsx';
import { statsApi } from '@/services/api';
import type { PairStat } from '@/types';

/**
 * Топ-5 пар по показателям
 */
export const TopPairs: React.FC = () => {
  const { data: stats, isLoading } = useQuery({
    queryKey: ['stats'],
    queryFn: statsApi.get,
  });

  if (isLoading) {
    return (
      <div className="bg-surface border border-border rounded-lg p-6">
        <div className="animate-pulse space-y-4">
          <div className="h-6 bg-gray-700 rounded w-1/4"></div>
          <div className="space-y-2">
            {[1, 2, 3, 4, 5].map((i) => (
              <div key={i} className="h-8 bg-gray-700 rounded"></div>
            ))}
          </div>
        </div>
      </div>
    );
  }

  if (!stats) {
    return null;
  }

  return (
    <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
      {/* Топ по сделкам */}
      <TopList
        title="Топ-5 по сделкам"
        items={stats.topPairsByTrades}
        formatValue={(value) => `${value} сделок`}
        icon={
          <svg className="w-5 h-5 text-blue-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z" />
          </svg>
        }
      />

      {/* Топ по прибыли */}
      <TopList
        title="Топ-5 прибыльных"
        items={stats.topPairsByProfit}
        formatValue={(value) => `+${value.toFixed(2)} USDT`}
        valueColor="green"
        icon={
          <svg className="w-5 h-5 text-green-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 7h8m0 0v8m0-8l-8 8-4-4-6 6" />
          </svg>
        }
      />

      {/* Топ по убыткам */}
      <TopList
        title="Топ-5 убыточных"
        items={stats.topPairsByLoss}
        formatValue={(value) => `${value.toFixed(2)} USDT`}
        valueColor="red"
        icon={
          <svg className="w-5 h-5 text-red-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 17h8m0 0V9m0 8l-8-8-4 4-6-6" />
          </svg>
        }
      />
    </div>
  );
};

interface TopListProps {
  title: string;
  items: PairStat[];
  formatValue: (value: number) => string;
  valueColor?: 'green' | 'red';
  icon: React.ReactNode;
}

const TopList: React.FC<TopListProps> = ({
  title,
  items,
  formatValue,
  valueColor,
  icon,
}) => {
  return (
    <div className="bg-surface border border-border rounded-lg p-4">
      <div className="flex items-center gap-2 mb-4">
        {icon}
        <h3 className="text-sm font-medium text-gray-400">{title}</h3>
      </div>

      {items.length === 0 ? (
        <div className="text-center text-gray-500 py-4 text-sm">
          Нет данных
        </div>
      ) : (
        <div className="space-y-2">
          {items.map((item, index) => (
            <div
              key={item.symbol}
              className="flex items-center justify-between py-2 px-3 bg-gray-800/50 rounded-lg"
            >
              <div className="flex items-center gap-3">
                <span className="text-gray-500 text-sm w-4">{index + 1}.</span>
                <span className="text-white font-medium text-sm">{item.symbol}</span>
              </div>
              <span
                className={clsx(
                  'text-sm font-medium',
                  valueColor === 'green' ? 'text-green-500' :
                  valueColor === 'red' ? 'text-red-500' :
                  'text-white'
                )}
              >
                {formatValue(item.value)}
              </span>
            </div>
          ))}
        </div>
      )}
    </div>
  );
};

export default TopPairs;
