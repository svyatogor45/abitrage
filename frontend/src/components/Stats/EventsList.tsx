import React, { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import clsx from 'clsx';
import { format } from 'date-fns';
import { ru } from 'date-fns/locale';
import { statsApi } from '@/services/api';
import type { StopLossEvent, LiquidationEvent } from '@/types';

/**
 * Списки событий SL и ликвидаций
 */
export const EventsList: React.FC = () => {
  const { data: stats } = useQuery({
    queryKey: ['stats'],
    queryFn: statsApi.get,
  });

  if (!stats) {
    return null;
  }

  return (
    <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
      <StopLossList events={stats.stopLossStats.events} />
      <LiquidationList events={stats.liquidationStats.events} />
    </div>
  );
};

interface StopLossListProps {
  events: StopLossEvent[];
}

const StopLossList: React.FC<StopLossListProps> = ({ events }) => {
  const [isExpanded, setIsExpanded] = useState(false);

  return (
    <div className="bg-surface border border-border rounded-lg p-4">
      <div
        className="flex items-center justify-between cursor-pointer"
        onClick={() => setIsExpanded(!isExpanded)}
      >
        <div className="flex items-center gap-2">
          <svg className="w-5 h-5 text-yellow-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
          </svg>
          <h3 className="text-sm font-medium text-white">
            Stop Loss ({events.length})
          </h3>
        </div>
        <svg
          className={clsx(
            'w-4 h-4 text-gray-400 transition-transform',
            isExpanded && 'rotate-180'
          )}
          fill="none"
          stroke="currentColor"
          viewBox="0 0 24 24"
        >
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
        </svg>
      </div>

      {isExpanded && (
        <div className="mt-4 space-y-2 max-h-60 overflow-y-auto">
          {events.length === 0 ? (
            <div className="text-center text-gray-500 py-2 text-sm">
              Нет событий
            </div>
          ) : (
            events.map((event, index) => (
              <div
                key={index}
                className="flex items-center justify-between py-2 px-3 bg-yellow-500/10 rounded-lg text-sm"
              >
                <div>
                  <span className="text-white font-medium">{event.symbol}</span>
                  <span className="text-gray-400 mx-2">
                    ({event.exchanges.join('/')})
                  </span>
                </div>
                <span className="text-gray-400 text-xs">
                  {format(new Date(event.timestamp), 'dd.MM.yyyy HH:mm', { locale: ru })}
                </span>
              </div>
            ))
          )}
        </div>
      )}
    </div>
  );
};

interface LiquidationListProps {
  events: LiquidationEvent[];
}

const LiquidationList: React.FC<LiquidationListProps> = ({ events }) => {
  const [isExpanded, setIsExpanded] = useState(false);

  return (
    <div className="bg-surface border border-border rounded-lg p-4">
      <div
        className="flex items-center justify-between cursor-pointer"
        onClick={() => setIsExpanded(!isExpanded)}
      >
        <div className="flex items-center gap-2">
          <svg className="w-5 h-5 text-red-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
          <h3 className="text-sm font-medium text-white">
            Ликвидации ({events.length})
          </h3>
        </div>
        <svg
          className={clsx(
            'w-4 h-4 text-gray-400 transition-transform',
            isExpanded && 'rotate-180'
          )}
          fill="none"
          stroke="currentColor"
          viewBox="0 0 24 24"
        >
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
        </svg>
      </div>

      {isExpanded && (
        <div className="mt-4 space-y-2 max-h-60 overflow-y-auto">
          {events.length === 0 ? (
            <div className="text-center text-gray-500 py-2 text-sm">
              Нет событий
            </div>
          ) : (
            events.map((event, index) => (
              <div
                key={index}
                className="flex items-center justify-between py-2 px-3 bg-red-500/10 rounded-lg text-sm"
              >
                <div>
                  <span className="text-white font-medium">{event.symbol}</span>
                  <span className="text-gray-400 mx-2">({event.exchange})</span>
                  <span
                    className={clsx(
                      'text-xs px-1.5 py-0.5 rounded',
                      event.side === 'long' ? 'bg-green-500/20 text-green-500' : 'bg-red-500/20 text-red-500'
                    )}
                  >
                    {event.side.toUpperCase()}
                  </span>
                </div>
                <span className="text-gray-400 text-xs">
                  {format(new Date(event.timestamp), 'dd.MM.yyyy HH:mm', { locale: ru })}
                </span>
              </div>
            ))
          )}
        </div>
      )}
    </div>
  );
};

export default EventsList;
