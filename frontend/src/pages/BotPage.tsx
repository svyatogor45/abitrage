import React, { useState } from 'react';
import clsx from 'clsx';
import { usePairs } from '@/hooks/usePairs';
import { BalanceBar } from '@/components/Common/BalanceBar';
import { PairCard } from '@/components/Bot/PairCard';
import { AddPairModal } from '@/components/Bot/AddPairModal';
import { EditPairModal } from '@/components/Bot/EditPairModal';
import { Blacklist } from '@/components/Bot/Blacklist';
import type { PairWithRuntime, PairCreateRequest, PairUpdateRequest } from '@/types';
import { MAX_PAIRS } from '@/types';

/**
 * Страница БОТ
 */
export const BotPage: React.FC = () => {
  const {
    pairs,
    isLoading,
    totalPairsCount,
    activePairsCount,
    holdingPairsCount,
    createPair,
    updatePair,
    deletePair,
    startPair,
    pausePair,
    isCreating,
    isUpdating,
    isStarting,
    isPausing,
  } = usePairs();

  const [isAddModalOpen, setIsAddModalOpen] = useState(false);
  const [editingPair, setEditingPair] = useState<PairWithRuntime | null>(null);

  const handleCreate = async (data: PairCreateRequest) => {
    await createPair(data);
  };

  const handleUpdate = async (id: number, data: PairUpdateRequest) => {
    await updatePair(id, data);
  };

  const handleDelete = async (id: number) => {
    if (confirm('Удалить эту торговую пару?')) {
      await deletePair(id);
    }
  };

  const canAddPair = totalPairsCount < MAX_PAIRS;

  return (
    <div className="p-6 space-y-6">
      {/* Балансы */}
      <BalanceBar />

      {/* Статистика пар */}
      <div className="bg-surface border border-border rounded-lg p-4">
        <div className="flex items-center justify-between">
          <div className="flex gap-6">
            <div>
              <span className="text-sm text-gray-400">Всего пар:</span>
              <span className="ml-2 text-lg font-bold text-white">{totalPairsCount}</span>
              <span className="text-gray-500">/{MAX_PAIRS}</span>
            </div>
            <div>
              <span className="text-sm text-gray-400">Активных:</span>
              <span className="ml-2 text-lg font-bold text-white">{activePairsCount}</span>
            </div>
            <div>
              <span className="text-sm text-gray-400">С позициями:</span>
              <span className="ml-2 text-lg font-bold text-green-500">{holdingPairsCount}</span>
            </div>
          </div>
        </div>
      </div>

      {/* Кнопка добавления */}
      <div className="flex justify-center">
        <button
          onClick={() => setIsAddModalOpen(true)}
          disabled={!canAddPair}
          className={clsx(
            'px-6 py-3 rounded-lg font-medium transition-colors',
            'bg-primary text-white hover:bg-primary/80',
            !canAddPair && 'opacity-50 cursor-not-allowed'
          )}
        >
          {canAddPair ? 'Добавить пару' : `Достигнут лимит (${MAX_PAIRS} пар)`}
        </button>
      </div>

      {/* Сетка карточек */}
      {isLoading ? (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {[1, 2, 3].map((i) => (
            <div key={i} className="animate-pulse">
              <div className="h-64 bg-gray-800 rounded-lg"></div>
            </div>
          ))}
        </div>
      ) : pairs.length === 0 ? (
        <div className="text-center py-12">
          <svg className="w-16 h-16 mx-auto text-gray-600 mb-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 6v6m0 0v6m0-6h6m-6 0H6" />
          </svg>
          <p className="text-gray-400 text-lg">Нет торговых пар</p>
          <p className="text-gray-500 text-sm mt-2">Нажмите "Добавить пару" для создания первой</p>
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {pairs.map((pair) => (
            <PairCard
              key={pair.id}
              pair={pair}
              onStart={startPair}
              onPause={pausePair}
              onEdit={setEditingPair}
              onDelete={handleDelete}
              isStarting={isStarting}
              isPausing={isPausing}
            />
          ))}
        </div>
      )}

      {/* Черный список */}
      <Blacklist />

      {/* Модальные окна */}
      <AddPairModal
        isOpen={isAddModalOpen}
        onClose={() => setIsAddModalOpen(false)}
        onSubmit={handleCreate}
        isLoading={isCreating}
      />

      <EditPairModal
        isOpen={!!editingPair}
        pair={editingPair}
        onClose={() => setEditingPair(null)}
        onSubmit={handleUpdate}
        isLoading={isUpdating}
      />
    </div>
  );
};

export default BotPage;
