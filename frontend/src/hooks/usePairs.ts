import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { useCallback, useEffect, useState } from 'react';
import { pairsApi } from '@/services/api';
import { websocketService } from '@/services/websocket';
import type {
  PairConfig,
  PairCreateRequest,
  PairUpdateRequest,
  PairRuntime,
  PairWithRuntime,
  PairUpdateMessage,
} from '@/types';

const PAIRS_QUERY_KEY = ['pairs'];

/**
 * Хук для работы с торговыми парами
 */
export function usePairs() {
  const queryClient = useQueryClient();
  const [runtimeData, setRuntimeData] = useState<Map<number, PairRuntime>>(
    new Map()
  );

  // Запрос списка пар
  const {
    data: pairs = [],
    isLoading,
    error,
    refetch,
  } = useQuery({
    queryKey: PAIRS_QUERY_KEY,
    queryFn: pairsApi.getAll,
    staleTime: 30000, // 30 секунд
  });

  // Подписка на обновления через WebSocket
  useEffect(() => {
    const unsubscribe = websocketService.onPairUpdate(
      (update: PairUpdateMessage) => {
        setRuntimeData((prev) => {
          const newMap = new Map(prev);
          const existing = newMap.get(update.id);

          newMap.set(update.id, {
            pairId: update.id,
            state: update.state,
            legs: update.legs || existing?.legs || [],
            filledParts: existing?.filledParts || 0,
            currentSpread: update.currentSpread,
            unrealizedPnl: update.unrealizedPnl,
            realizedPnl: existing?.realizedPnl || 0,
            lastUpdate: new Date().toISOString(),
          });

          return newMap;
        });
      }
    );

    return unsubscribe;
  }, []);

  // Комбинирование статических данных пар с runtime
  const pairsWithRuntime: PairWithRuntime[] = pairs.map((pair) => ({
    ...pair,
    runtime: runtimeData.get(pair.id),
  }));

  // Мутация создания пары
  const createMutation = useMutation({
    mutationFn: (data: PairCreateRequest) => pairsApi.create(data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: PAIRS_QUERY_KEY });
    },
  });

  // Мутация обновления пары
  const updateMutation = useMutation({
    mutationFn: ({ id, data }: { id: number; data: PairUpdateRequest }) =>
      pairsApi.update(id, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: PAIRS_QUERY_KEY });
    },
  });

  // Мутация удаления пары
  const deleteMutation = useMutation({
    mutationFn: (id: number) => pairsApi.delete(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: PAIRS_QUERY_KEY });
    },
  });

  // Мутация запуска пары
  const startMutation = useMutation({
    mutationFn: (id: number) => pairsApi.start(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: PAIRS_QUERY_KEY });
    },
  });

  // Мутация паузы пары
  const pauseMutation = useMutation({
    mutationFn: ({ id, forceClose }: { id: number; forceClose?: boolean }) =>
      pairsApi.pause(id, forceClose),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: PAIRS_QUERY_KEY });
    },
  });

  // Методы-обертки
  const createPair = useCallback(
    async (data: PairCreateRequest) => {
      return createMutation.mutateAsync(data);
    },
    [createMutation]
  );

  const updatePair = useCallback(
    async (id: number, data: PairUpdateRequest) => {
      return updateMutation.mutateAsync({ id, data });
    },
    [updateMutation]
  );

  const deletePair = useCallback(
    async (id: number) => {
      return deleteMutation.mutateAsync(id);
    },
    [deleteMutation]
  );

  const startPair = useCallback(
    async (id: number) => {
      return startMutation.mutateAsync(id);
    },
    [startMutation]
  );

  const pausePair = useCallback(
    async (id: number, forceClose = false) => {
      return pauseMutation.mutateAsync({ id, forceClose });
    },
    [pauseMutation]
  );

  // Получение пары по ID
  const getPairById = useCallback(
    (id: number): PairWithRuntime | undefined => {
      const pair = pairs.find((p) => p.id === id);
      if (!pair) return undefined;
      return {
        ...pair,
        runtime: runtimeData.get(pair.id),
      };
    },
    [pairs, runtimeData]
  );

  // Подсчет активных пар
  const activePairsCount = pairs.filter((p) => p.status === 'active').length;

  // Подсчет пар с открытыми позициями
  const holdingPairsCount = Array.from(runtimeData.values()).filter(
    (r) => r.state === 'HOLDING'
  ).length;

  return {
    // Данные
    pairs: pairsWithRuntime,
    isLoading,
    error,
    activePairsCount,
    holdingPairsCount,
    totalPairsCount: pairs.length,

    // Методы
    refetch,
    createPair,
    updatePair,
    deletePair,
    startPair,
    pausePair,
    getPairById,

    // Состояния мутаций
    isCreating: createMutation.isPending,
    isUpdating: updateMutation.isPending,
    isDeleting: deleteMutation.isPending,
    isStarting: startMutation.isPending,
    isPausing: pauseMutation.isPending,

    // Ошибки мутаций
    createError: createMutation.error,
    updateError: updateMutation.error,
    deleteError: deleteMutation.error,
  };
}

/**
 * Хук для получения одной пары по ID
 */
export function usePair(id: number) {
  const { getPairById, ...rest } = usePairs();

  return {
    pair: getPairById(id),
    ...rest,
  };
}
