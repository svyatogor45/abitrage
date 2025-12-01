import { useEffect, useState, useCallback, useRef } from 'react';
import { websocketService } from '@/services/websocket';
import type {
  PairUpdateMessage,
  BalanceUpdateMessage,
  StatsUpdateMessage,
  Notification,
} from '@/types';

type ConnectionStatus = 'connecting' | 'connected' | 'disconnected' | 'error';

interface UseWebSocketOptions {
  autoConnect?: boolean;
  onPairUpdate?: (data: PairUpdateMessage) => void;
  onBalanceUpdate?: (data: BalanceUpdateMessage) => void;
  onStatsUpdate?: (data: StatsUpdateMessage) => void;
  onNotification?: (data: Notification) => void;
}

/**
 * Хук для работы с WebSocket соединением
 */
export function useWebSocket(options: UseWebSocketOptions = {}) {
  const {
    autoConnect = true,
    onPairUpdate,
    onBalanceUpdate,
    onStatsUpdate,
    onNotification,
  } = options;

  const [status, setStatus] = useState<ConnectionStatus>('disconnected');
  const [lastError, setLastError] = useState<Event | null>(null);
  const mountedRef = useRef(true);

  // Подключение к WebSocket
  const connect = useCallback(() => {
    if (!mountedRef.current) return;
    setStatus('connecting');
    websocketService.connect();
  }, []);

  // Отключение от WebSocket
  const disconnect = useCallback(() => {
    websocketService.disconnect();
    if (mountedRef.current) {
      setStatus('disconnected');
    }
  }, []);

  // Обработчики событий соединения
  useEffect(() => {
    const unsubscribeOpen = websocketService.onOpen(() => {
      if (mountedRef.current) {
        setStatus('connected');
        setLastError(null);
      }
    });

    const unsubscribeClose = websocketService.onClose(() => {
      if (mountedRef.current) {
        setStatus('disconnected');
      }
    });

    const unsubscribeError = websocketService.onError((error) => {
      if (mountedRef.current) {
        setStatus('error');
        setLastError(error);
      }
    });

    return () => {
      unsubscribeOpen();
      unsubscribeClose();
      unsubscribeError();
    };
  }, []);

  // Подписки на сообщения
  useEffect(() => {
    const unsubscribers: (() => void)[] = [];

    if (onPairUpdate) {
      unsubscribers.push(websocketService.onPairUpdate(onPairUpdate));
    }

    if (onBalanceUpdate) {
      unsubscribers.push(websocketService.onBalanceUpdate(onBalanceUpdate));
    }

    if (onStatsUpdate) {
      unsubscribers.push(websocketService.onStatsUpdate(onStatsUpdate));
    }

    if (onNotification) {
      unsubscribers.push(websocketService.onNotification(onNotification));
    }

    return () => {
      unsubscribers.forEach((unsub) => unsub());
    };
  }, [onPairUpdate, onBalanceUpdate, onStatsUpdate, onNotification]);

  // Автоматическое подключение
  useEffect(() => {
    mountedRef.current = true;

    if (autoConnect) {
      connect();
    }

    return () => {
      mountedRef.current = false;
    };
  }, [autoConnect, connect]);

  return {
    status,
    isConnected: status === 'connected',
    isConnecting: status === 'connecting',
    lastError,
    connect,
    disconnect,
  };
}

/**
 * Хук для подписки только на обновления пар
 */
export function usePairUpdates(
  onUpdate: (data: PairUpdateMessage) => void
) {
  useEffect(() => {
    const unsubscribe = websocketService.onPairUpdate(onUpdate);
    return unsubscribe;
  }, [onUpdate]);
}

/**
 * Хук для подписки только на обновления балансов
 */
export function useBalanceUpdates(
  onUpdate: (data: BalanceUpdateMessage) => void
) {
  useEffect(() => {
    const unsubscribe = websocketService.onBalanceUpdate(onUpdate);
    return unsubscribe;
  }, [onUpdate]);
}

/**
 * Хук для подписки только на уведомления
 */
export function useNotificationUpdates(
  onUpdate: (data: Notification) => void
) {
  useEffect(() => {
    const unsubscribe = websocketService.onNotification(onUpdate);
    return unsubscribe;
  }, [onUpdate]);
}

/**
 * Хук для подписки только на обновления статистики
 */
export function useStatsUpdates(
  onUpdate: (data: StatsUpdateMessage) => void
) {
  useEffect(() => {
    const unsubscribe = websocketService.onStatsUpdate(onUpdate);
    return unsubscribe;
  }, [onUpdate]);
}
