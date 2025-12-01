import React from 'react';
import clsx from 'clsx';
import { useNotifications } from '@/hooks/useNotifications';
import { NotificationFeed } from '@/components/Notifications/NotificationFeed';
import { NotificationFilters } from '@/components/Notifications/NotificationFilters';

/**
 * Страница Уведомлений
 */
export const NotificationsPage: React.FC = () => {
  const {
    notifications,
    isLoading,
    filters,
    totalCount,
    toggleFilter,
    enableAllFilters,
    disableAllFilters,
    clearNotifications,
    isClearing,
  } = useNotifications();

  return (
    <div className="p-6 space-y-6">
      {/* Заголовок */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-bold text-white">Уведомления</h1>
          <p className="text-sm text-gray-400 mt-1">
            Показано {notifications.length} из {totalCount} уведомлений
          </p>
        </div>
        <button
          onClick={() => {
            if (confirm('Очистить журнал уведомлений?')) {
              clearNotifications();
            }
          }}
          disabled={isClearing || totalCount === 0}
          className={clsx(
            'px-4 py-2 rounded-lg text-sm font-medium transition-colors',
            'bg-red-500/20 text-red-500 hover:bg-red-500/30',
            (isClearing || totalCount === 0) && 'opacity-50 cursor-not-allowed'
          )}
        >
          {isClearing ? 'Очистка...' : 'Очистить журнал'}
        </button>
      </div>

      {/* Фильтры */}
      <NotificationFilters
        filters={filters}
        onToggle={toggleFilter}
        onEnableAll={enableAllFilters}
        onDisableAll={disableAllFilters}
      />

      {/* Лента уведомлений */}
      <div className="bg-surface border border-border rounded-lg p-4">
        <NotificationFeed
          notifications={notifications}
          isLoading={isLoading}
        />
      </div>
    </div>
  );
};

export default NotificationsPage;
