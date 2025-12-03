import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { useCallback, useEffect, useState } from 'react';
import { exchangesApi } from '@/services/api';
import { websocketService } from '@/services/websocket';
import type {
  ExchangeAccount,
  ExchangeConnectRequest,
  ExchangeName,
  BalanceUpdateMessage,
} from '@/types';

const EXCHANGES_QUERY_KEY = ['exchanges'];

/**
 * Хук для работы с биржами
 */
export function useExchanges() {
  const queryClient = useQueryClient();
  const [balances, setBalances] = useState<Map<ExchangeName, number>>(
    new Map()
  );

  // Запрос списка бирж
  const {
    data: exchanges = [],
    isLoading,
    error,
    refetch,
  } = useQuery({
    queryKey: EXCHANGES_QUERY_KEY,
    queryFn: exchangesApi.getAll,
    staleTime: 60000, // 1 минута
  });

  // Подписка на обновления балансов через WebSocket
  useEffect(() => {
    const unsubscribe = websocketService.onBalanceUpdate(
      (update: BalanceUpdateMessage) => {
        setBalances((prev) => {
          const newMap = new Map(prev);
          newMap.set(update.exchange, update.balance);
          return newMap;
        });
      }
    );

    return unsubscribe;
  }, []);

  // Инициализация балансов из начальных данных
  useEffect(() => {
    // Только обновляем если есть данные и они отличаются
    if (exchanges.length === 0) return;

    setBalances((prev) => {
      const newBalances = new Map<ExchangeName, number>();
      exchanges.forEach((exchange) => {
        if (exchange.connected) {
          newBalances.set(exchange.name, exchange.balance);
        }
      });

      // Проверяем, изменились ли данные
      if (prev.size === newBalances.size) {
        let equal = true;
        prev.forEach((value, key) => {
          if (newBalances.get(key) !== value) equal = false;
        });
        if (equal) return prev; // Возвращаем старый Map если данные не изменились
      }

      return newBalances;
    });
  }, [exchanges]);

  // Комбинирование данных бирж с актуальными балансами
  const exchangesWithBalances: ExchangeAccount[] = exchanges.map((exchange) => ({
    ...exchange,
    balance: balances.get(exchange.name) ?? exchange.balance,
  }));

  // Мутация подключения биржи
  const connectMutation = useMutation({
    mutationFn: ({
      name,
      credentials,
    }: {
      name: ExchangeName;
      credentials: ExchangeConnectRequest;
    }) => exchangesApi.connect(name, credentials),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: EXCHANGES_QUERY_KEY });
    },
  });

  // Мутация отключения биржи
  const disconnectMutation = useMutation({
    mutationFn: (name: ExchangeName) => exchangesApi.disconnect(name),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: EXCHANGES_QUERY_KEY });
    },
  });

  // Мутация обновления баланса
  const refreshBalanceMutation = useMutation({
    mutationFn: (name: ExchangeName) => exchangesApi.refreshBalance(name),
    onSuccess: (data) => {
      setBalances((prev) => {
        const newMap = new Map(prev);
        newMap.set(data.name, data.balance);
        return newMap;
      });
    },
  });

  // Методы-обертки
  const connectExchange = useCallback(
    async (name: ExchangeName, credentials: ExchangeConnectRequest) => {
      return connectMutation.mutateAsync({ name, credentials });
    },
    [connectMutation]
  );

  const disconnectExchange = useCallback(
    async (name: ExchangeName) => {
      return disconnectMutation.mutateAsync(name);
    },
    [disconnectMutation]
  );

  const refreshBalance = useCallback(
    async (name: ExchangeName) => {
      return refreshBalanceMutation.mutateAsync(name);
    },
    [refreshBalanceMutation]
  );

  const refreshAllBalances = useCallback(async () => {
    const connectedExchanges = exchanges.filter((e) => e.connected);
    await Promise.all(
      connectedExchanges.map((e) => refreshBalanceMutation.mutateAsync(e.name))
    );
  }, [exchanges, refreshBalanceMutation]);

  // Получение биржи по имени
  const getExchangeByName = useCallback(
    (name: ExchangeName): ExchangeAccount | undefined => {
      return exchangesWithBalances.find((e) => e.name === name);
    },
    [exchangesWithBalances]
  );

  // Подключенные биржи
  const connectedExchanges = exchangesWithBalances.filter((e) => e.connected);

  // Общий баланс
  const totalBalance = connectedExchanges.reduce(
    (sum, e) => sum + e.balance,
    0
  );

  // Проверка минимального количества подключенных бирж для арбитража
  const canTrade = connectedExchanges.length >= 2;

  return {
    // Данные
    exchanges: exchangesWithBalances,
    connectedExchanges,
    isLoading,
    error,
    totalBalance,
    canTrade,

    // Методы
    refetch,
    connectExchange,
    disconnectExchange,
    refreshBalance,
    refreshAllBalances,
    getExchangeByName,

    // Состояния мутаций
    isConnecting: connectMutation.isPending,
    isDisconnecting: disconnectMutation.isPending,
    isRefreshing: refreshBalanceMutation.isPending,

    // Ошибки мутаций
    connectError: connectMutation.error,
    disconnectError: disconnectMutation.error,
  };
}

/**
 * Хук для получения одной биржи по имени
 */
export function useExchange(name: ExchangeName) {
  const { getExchangeByName, ...rest } = useExchanges();

  return {
    exchange: getExchangeByName(name),
    ...rest,
  };
}
