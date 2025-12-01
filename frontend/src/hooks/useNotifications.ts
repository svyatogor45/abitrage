import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { useCallback, useEffect, useState } from 'react';
import { notificationsApi } from '@/services/api';
import { websocketService } from '@/services/websocket';
import type {
  Notification,
  NotificationType,
  NotificationFilters,
  DEFAULT_NOTIFICATION_FILTERS,
} from '@/types';

const NOTIFICATIONS_QUERY_KEY = ['notifications'];
const MAX_NOTIFICATIONS = 100;

/**
 * Хук для работы с уведомлениями
 */
export function useNotifications(initialFilters?: Partial<NotificationFilters>) {
  const queryClient = useQueryClient();
  const [filters, setFilters] = useState<NotificationFilters>({
    open: true,
    close: true,
    stopLoss: true,
    liquidation: true,
    apiError: true,
    margin: true,
    pause: true,
    secondLegFail: true,
    ...initialFilters,
  });

  // Маппинг фильтров в типы уведомлений
  const getActiveTypes = useCallback((): NotificationType[] => {
    const typeMap: Record<keyof NotificationFilters, NotificationType> = {
      open: 'OPEN',
      close: 'CLOSE',
      stopLoss: 'SL',
      liquidation: 'LIQUIDATION',
      apiError: 'ERROR',
      margin: 'MARGIN',
      pause: 'PAUSE',
      secondLegFail: 'SECOND_LEG_FAIL',
    };

    return Object.entries(filters)
      .filter(([, enabled]) => enabled)
      .map(([key]) => typeMap[key as keyof NotificationFilters]);
  }, [filters]);

  // Запрос списка уведомлений
  const {
    data: notifications = [],
    isLoading,
    error,
    refetch,
  } = useQuery({
    queryKey: [...NOTIFICATIONS_QUERY_KEY, getActiveTypes()],
    queryFn: () => notificationsApi.getAll(getActiveTypes(), MAX_NOTIFICATIONS),
    staleTime: 10000, // 10 секунд
  });

  // Подписка на новые уведомления через WebSocket
  useEffect(() => {
    const unsubscribe = websocketService.onNotification(
      (notification: Notification) => {
        // Проверяем, проходит ли уведомление через фильтр
        const activeTypes = getActiveTypes();
        if (!activeTypes.includes(notification.type)) {
          return;
        }

        // Добавляем новое уведомление в начало списка
        queryClient.setQueryData<Notification[]>(
          [...NOTIFICATIONS_QUERY_KEY, activeTypes],
          (old = []) => {
            const updated = [notification, ...old];
            // Ограничиваем количество уведомлений
            return updated.slice(0, MAX_NOTIFICATIONS);
          }
        );
      }
    );

    return unsubscribe;
  }, [queryClient, getActiveTypes]);

  // Мутация очистки уведомлений
  const clearMutation = useMutation({
    mutationFn: () => notificationsApi.clear(),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: NOTIFICATIONS_QUERY_KEY });
    },
  });

  // Методы управления фильтрами
  const toggleFilter = useCallback((key: keyof NotificationFilters) => {
    setFilters((prev) => ({
      ...prev,
      [key]: !prev[key],
    }));
  }, []);

  const setFilter = useCallback(
    (key: keyof NotificationFilters, value: boolean) => {
      setFilters((prev) => ({
        ...prev,
        [key]: value,
      }));
    },
    []
  );

  const resetFilters = useCallback(() => {
    setFilters({
      open: true,
      close: true,
      stopLoss: true,
      liquidation: true,
      apiError: true,
      margin: true,
      pause: true,
      secondLegFail: true,
    });
  }, []);

  const enableAllFilters = useCallback(() => {
    setFilters({
      open: true,
      close: true,
      stopLoss: true,
      liquidation: true,
      apiError: true,
      margin: true,
      pause: true,
      secondLegFail: true,
    });
  }, []);

  const disableAllFilters = useCallback(() => {
    setFilters({
      open: false,
      close: false,
      stopLoss: false,
      liquidation: false,
      apiError: false,
      margin: false,
      pause: false,
      secondLegFail: false,
    });
  }, []);

  // Очистка уведомлений
  const clearNotifications = useCallback(async () => {
    return clearMutation.mutateAsync();
  }, [clearMutation]);

  // Фильтрация уведомлений на клиенте (дополнительная)
  const filteredNotifications = notifications.filter((n) =>
    getActiveTypes().includes(n.type)
  );

  // Группировка по типам для подсчета
  const countByType = notifications.reduce(
    (acc, n) => {
      acc[n.type] = (acc[n.type] || 0) + 1;
      return acc;
    },
    {} as Record<NotificationType, number>
  );

  // Подсчет непрочитанных (последние за 5 минут)
  const fiveMinutesAgo = Date.now() - 5 * 60 * 1000;
  const recentCount = notifications.filter(
    (n) => new Date(n.timestamp).getTime() > fiveMinutesAgo
  ).length;

  return {
    // Данные
    notifications: filteredNotifications,
    allNotifications: notifications,
    isLoading,
    error,
    filters,
    countByType,
    recentCount,
    totalCount: notifications.length,

    // Методы
    refetch,
    clearNotifications,
    toggleFilter,
    setFilter,
    resetFilters,
    enableAllFilters,
    disableAllFilters,

    // Состояния мутаций
    isClearing: clearMutation.isPending,

    // Ошибки
    clearError: clearMutation.error,
  };
}

/**
 * Хук для получения только количества непрочитанных уведомлений
 */
export function useUnreadNotificationsCount() {
  const [count, setCount] = useState(0);

  useEffect(() => {
    const unsubscribe = websocketService.onNotification(() => {
      setCount((prev) => prev + 1);
    });

    return unsubscribe;
  }, []);

  const resetCount = useCallback(() => {
    setCount(0);
  }, []);

  return { count, resetCount };
}
