import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { useCallback, useEffect, useRef, useMemo } from 'react';
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

  // Ref для хранения WebSocket балансов (не вызывает ре-рендер)
  const wsBalancesRef = useRef<Map<ExchangeName, number>>(new Map());

  // Запрос списка бирж с отключённым retry при ошибках
  const {
    data: exchanges = [],
    isLoading,
    error,
    refetch,
  } = useQuery({
    queryKey: EXCHANGES_QUERY_KEY,
    queryFn: exchangesApi.getAll,
    staleTime: 60000, // 1 минута
    retry: false, // Отключаем retry чтобы избежать бесконечного цикла
    refetchOnWindowFocus: false,
  });

  // Подписка на обновления балансов через WebSocket
  useEffect(() => {
    const unsubscribe = websocketService.onBalanceUpdate(
      (update: BalanceUpdateMessage) => {
        wsBalancesRef.current.set(update.exchange, update.balance);
        // Инвалидируем кэш чтобы обновить UI
        queryClient.invalidateQueries({ queryKey: EXCHANGES_QUERY_KEY });
      }
    );

    return unsubscribe;
  }, [queryClient]);

  // Комбинирование данных бирж с актуальными балансами (без useEffect!)
  const exchangesWithBalances: ExchangeAccount[] = useMemo(() => {
    return exchanges.map((exchange) => ({
      ...exchange,
      balance: wsBalancesRef.current.get(exchange.name) ?? exchange.balance,
    }));
  }, [exchanges]);

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
      wsBalancesRef.current.set(data.name, data.balance);
      queryClient.invalidateQueries({ queryKey: EXCHANGES_QUERY_KEY });
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
