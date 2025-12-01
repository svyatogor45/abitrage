import React, { useState, useEffect } from 'react';
import clsx from 'clsx';
import type { PairWithRuntime, PairUpdateRequest } from '@/types';

interface EditPairModalProps {
  isOpen: boolean;
  pair: PairWithRuntime | null;
  onClose: () => void;
  onSubmit: (id: number, data: PairUpdateRequest) => Promise<void>;
  isLoading?: boolean;
}

/**
 * Модальное окно редактирования пары
 */
export const EditPairModal: React.FC<EditPairModalProps> = ({
  isOpen,
  pair,
  onClose,
  onSubmit,
  isLoading,
}) => {
  const [formData, setFormData] = useState<PairUpdateRequest>({
    entrySpread: 1.0,
    exitSpread: 0.2,
    volume: 0.1,
    nOrders: 1,
    stopLoss: null,
  });

  const [errors, setErrors] = useState<Partial<Record<keyof PairUpdateRequest, string>>>({});

  // Инициализация формы при открытии
  useEffect(() => {
    if (pair && isOpen) {
      setFormData({
        entrySpread: pair.entrySpread,
        exitSpread: pair.exitSpread,
        volume: pair.volume,
        nOrders: pair.nOrders,
        stopLoss: pair.stopLoss,
      });
      setErrors({});
    }
  }, [pair, isOpen]);

  const hasOpenPosition = pair?.runtime?.state === 'HOLDING' ||
    pair?.runtime?.state === 'ENTERING' ||
    pair?.runtime?.state === 'EXITING';

  const validate = (): boolean => {
    const newErrors: Partial<Record<keyof PairUpdateRequest, string>> = {};

    if (formData.entrySpread !== undefined && formData.entrySpread <= 0) {
      newErrors.entrySpread = 'Спред входа должен быть > 0';
    }

    if (formData.exitSpread !== undefined && formData.exitSpread <= 0) {
      newErrors.exitSpread = 'Спред выхода должен быть > 0';
    }

    if (formData.exitSpread !== undefined && formData.entrySpread !== undefined &&
        formData.exitSpread >= formData.entrySpread) {
      newErrors.exitSpread = 'Спред выхода должен быть меньше спреда входа';
    }

    if (formData.volume !== undefined && formData.volume <= 0) {
      newErrors.volume = 'Объем должен быть > 0';
    }

    if (formData.nOrders !== undefined && formData.nOrders < 1) {
      newErrors.nOrders = 'Количество ордеров должно быть >= 1';
    }

    if (formData.stopLoss !== undefined && formData.stopLoss !== null && formData.stopLoss <= 0) {
      newErrors.stopLoss = 'Stop Loss должен быть > 0';
    }

    setErrors(newErrors);
    return Object.keys(newErrors).length === 0;
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();

    if (!pair || !validate()) {
      return;
    }

    try {
      await onSubmit(pair.id, formData);
      onClose();
    } catch (error) {
      console.error('Ошибка обновления пары:', error);
    }
  };

  if (!isOpen || !pair) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center">
      {/* Overlay */}
      <div
        className="absolute inset-0 bg-black/60"
        onClick={onClose}
      />

      {/* Modal */}
      <div className="relative bg-surface border border-border rounded-lg w-full max-w-md mx-4 p-6">
        <h2 className="text-xl font-bold text-white mb-2">
          Редактировать {pair.symbol}
        </h2>

        {hasOpenPosition && (
          <div className="bg-yellow-500/10 border border-yellow-500/30 rounded-lg p-3 mb-4">
            <p className="text-yellow-500 text-sm">
              По паре открыта позиция. Изменения вступят в силу после закрытия текущего арбитража.
            </p>
          </div>
        )}

        <form onSubmit={handleSubmit} className="space-y-4">
          {/* Рабочий объем */}
          <div>
            <label className="block text-sm font-medium text-gray-400 mb-1">
              Рабочий объем ({pair.base})
            </label>
            <input
              type="number"
              value={formData.volume}
              onChange={(e) => setFormData({ ...formData, volume: parseFloat(e.target.value) || 0 })}
              step="0.0001"
              min="0"
              className={clsx(
                'w-full px-3 py-2 bg-gray-800 border rounded-lg text-white',
                errors.volume ? 'border-red-500' : 'border-gray-700'
              )}
            />
            {errors.volume && <p className="text-red-500 text-xs mt-1">{errors.volume}</p>}
          </div>

          {/* Спреды */}
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-sm font-medium text-gray-400 mb-1">
                Спред входа (%)
              </label>
              <input
                type="number"
                value={formData.entrySpread}
                onChange={(e) => setFormData({ ...formData, entrySpread: parseFloat(e.target.value) || 0 })}
                step="0.01"
                min="0"
                className={clsx(
                  'w-full px-3 py-2 bg-gray-800 border rounded-lg text-white',
                  errors.entrySpread ? 'border-red-500' : 'border-gray-700'
                )}
              />
              {errors.entrySpread && <p className="text-red-500 text-xs mt-1">{errors.entrySpread}</p>}
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-400 mb-1">
                Спред выхода (%)
              </label>
              <input
                type="number"
                value={formData.exitSpread}
                onChange={(e) => setFormData({ ...formData, exitSpread: parseFloat(e.target.value) || 0 })}
                step="0.01"
                min="0"
                className={clsx(
                  'w-full px-3 py-2 bg-gray-800 border rounded-lg text-white',
                  errors.exitSpread ? 'border-red-500' : 'border-gray-700'
                )}
              />
              {errors.exitSpread && <p className="text-red-500 text-xs mt-1">{errors.exitSpread}</p>}
            </div>
          </div>

          {/* Количество ордеров */}
          <div>
            <label className="block text-sm font-medium text-gray-400 mb-1">
              Количество ордеров (разбиение)
            </label>
            <input
              type="number"
              value={formData.nOrders}
              onChange={(e) => setFormData({ ...formData, nOrders: parseInt(e.target.value) || 1 })}
              min="1"
              max="10"
              className={clsx(
                'w-full px-3 py-2 bg-gray-800 border rounded-lg text-white',
                errors.nOrders ? 'border-red-500' : 'border-gray-700'
              )}
            />
            {errors.nOrders && <p className="text-red-500 text-xs mt-1">{errors.nOrders}</p>}
          </div>

          {/* Stop Loss */}
          <div>
            <label className="block text-sm font-medium text-gray-400 mb-1">
              Stop Loss (USDT)
            </label>
            <input
              type="number"
              value={formData.stopLoss ?? ''}
              onChange={(e) => setFormData({
                ...formData,
                stopLoss: e.target.value ? parseFloat(e.target.value) : null
              })}
              step="1"
              min="0"
              placeholder="Не установлен"
              className={clsx(
                'w-full px-3 py-2 bg-gray-800 border rounded-lg text-white placeholder-gray-500',
                errors.stopLoss ? 'border-red-500' : 'border-gray-700'
              )}
            />
            {errors.stopLoss && <p className="text-red-500 text-xs mt-1">{errors.stopLoss}</p>}
          </div>

          {/* Кнопки */}
          <div className="flex gap-3 pt-4">
            <button
              type="button"
              onClick={onClose}
              className="flex-1 py-2 px-4 rounded-lg text-sm font-medium bg-gray-700 text-gray-300 hover:bg-gray-600 transition-colors"
            >
              Отмена
            </button>
            <button
              type="submit"
              disabled={isLoading}
              className={clsx(
                'flex-1 py-2 px-4 rounded-lg text-sm font-medium bg-primary text-white hover:bg-primary/80 transition-colors',
                isLoading && 'opacity-50 cursor-not-allowed'
              )}
            >
              {isLoading ? 'Сохранение...' : 'Сохранить'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
};

export default EditPairModal;
