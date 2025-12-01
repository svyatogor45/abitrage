import React, { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import clsx from 'clsx';
import { blacklistApi } from '@/services/api';
import type { BlacklistEntry } from '@/types';

/**
 * Компонент черного списка торговых пар
 */
export const Blacklist: React.FC = () => {
  const queryClient = useQueryClient();
  const [newSymbol, setNewSymbol] = useState('');
  const [newReason, setNewReason] = useState('');

  // Запрос списка
  const { data: entries = [], isLoading } = useQuery({
    queryKey: ['blacklist'],
    queryFn: blacklistApi.getAll,
  });

  // Мутация добавления
  const addMutation = useMutation({
    mutationFn: blacklistApi.add,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['blacklist'] });
      setNewSymbol('');
      setNewReason('');
    },
  });

  // Мутация удаления
  const removeMutation = useMutation({
    mutationFn: blacklistApi.remove,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['blacklist'] });
    },
  });

  const handleAdd = (e: React.FormEvent) => {
    e.preventDefault();
    if (!newSymbol.trim()) return;

    addMutation.mutate({
      symbol: newSymbol.toUpperCase().trim(),
      reason: newReason.trim() || undefined,
    });
  };

  const handleRemove = (symbol: string) => {
    if (confirm(`Удалить ${symbol} из черного списка?`)) {
      removeMutation.mutate(symbol);
    }
  };

  return (
    <div className="bg-surface border border-border rounded-lg p-4">
      <h3 className="text-lg font-bold text-white mb-4">Черный список</h3>

      <p className="text-sm text-gray-400 mb-4">
        Справочный список инструментов, с которыми вы не желаете работать.
        Носит информативный характер.
      </p>

      {/* Форма добавления */}
      <form onSubmit={handleAdd} className="flex gap-2 mb-4">
        <input
          type="text"
          value={newSymbol}
          onChange={(e) => setNewSymbol(e.target.value)}
          placeholder="Символ (напр. BTCUSDT)"
          className="flex-1 px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-white placeholder-gray-500 text-sm"
        />
        <input
          type="text"
          value={newReason}
          onChange={(e) => setNewReason(e.target.value)}
          placeholder="Причина (опционально)"
          className="flex-1 px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-white placeholder-gray-500 text-sm"
        />
        <button
          type="submit"
          disabled={!newSymbol.trim() || addMutation.isPending}
          className={clsx(
            'px-4 py-2 rounded-lg text-sm font-medium bg-red-500/20 text-red-500 hover:bg-red-500/30 transition-colors',
            (!newSymbol.trim() || addMutation.isPending) && 'opacity-50 cursor-not-allowed'
          )}
        >
          {addMutation.isPending ? 'Добавление...' : 'Добавить'}
        </button>
      </form>

      {/* Список */}
      {isLoading ? (
        <div className="text-center text-gray-400 py-4">Загрузка...</div>
      ) : entries.length === 0 ? (
        <div className="text-center text-gray-500 py-4">
          Черный список пуст
        </div>
      ) : (
        <div className="space-y-2">
          {entries.map((entry) => (
            <BlacklistItem
              key={entry.id}
              entry={entry}
              onRemove={handleRemove}
              isRemoving={removeMutation.isPending}
            />
          ))}
        </div>
      )}
    </div>
  );
};

interface BlacklistItemProps {
  entry: BlacklistEntry;
  onRemove: (symbol: string) => void;
  isRemoving: boolean;
}

const BlacklistItem: React.FC<BlacklistItemProps> = ({
  entry,
  onRemove,
  isRemoving,
}) => {
  return (
    <div className="flex items-center justify-between px-3 py-2 bg-gray-800/50 rounded-lg">
      <div className="flex items-center gap-3">
        <span className="text-white font-medium">{entry.symbol}</span>
        {entry.reason && (
          <span className="text-sm text-gray-400">— {entry.reason}</span>
        )}
      </div>
      <button
        onClick={() => onRemove(entry.symbol)}
        disabled={isRemoving}
        className={clsx(
          'p-1 text-gray-400 hover:text-red-500 transition-colors',
          isRemoving && 'opacity-50 cursor-not-allowed'
        )}
        title="Удалить из черного списка"
      >
        <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth={2}
            d="M6 18L18 6M6 6l12 12"
          />
        </svg>
      </button>
    </div>
  );
};

export default Blacklist;
