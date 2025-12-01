import axios, { AxiosInstance, AxiosError } from 'axios';
import type {
  ExchangeAccount,
  ExchangeConnectRequest,
  ExchangeName,
  PairConfig,
  PairCreateRequest,
  PairUpdateRequest,
  Notification,
  NotificationType,
  Stats,
  Settings,
  SettingsUpdateRequest,
  BlacklistEntry,
  BlacklistCreateRequest,
  ApiError,
} from '@/types';

// Базовый URL API
const API_BASE_URL = import.meta.env.VITE_API_URL || '/api';

// Создание axios инстанса
const apiClient: AxiosInstance = axios.create({
  baseURL: API_BASE_URL,
  timeout: 30000,
  headers: {
    'Content-Type': 'application/json',
  },
});

// Интерцептор для добавления токена авторизации
apiClient.interceptors.request.use(
  (config) => {
    const token = localStorage.getItem('auth_token');
    if (token) {
      config.headers.Authorization = `Bearer ${token}`;
    }
    return config;
  },
  (error) => Promise.reject(error)
);

// Интерцептор для обработки ошибок
apiClient.interceptors.response.use(
  (response) => response,
  (error: AxiosError<ApiError>) => {
    if (error.response?.status === 401) {
      // Очистка токена и редирект на логин
      localStorage.removeItem('auth_token');
      window.location.href = '/login';
    }
    return Promise.reject(error);
  }
);

// ============================================
// Exchanges API
// ============================================

export const exchangesApi = {
  /**
   * Получение списка всех бирж и их статусов
   */
  getAll: async (): Promise<ExchangeAccount[]> => {
    const response = await apiClient.get<ExchangeAccount[]>('/exchanges');
    return response.data;
  },

  /**
   * Подключение биржи
   */
  connect: async (
    name: ExchangeName,
    credentials: ExchangeConnectRequest
  ): Promise<ExchangeAccount> => {
    const response = await apiClient.post<ExchangeAccount>(
      `/exchanges/${name}/connect`,
      credentials
    );
    return response.data;
  },

  /**
   * Отключение биржи
   */
  disconnect: async (name: ExchangeName): Promise<void> => {
    await apiClient.delete(`/exchanges/${name}/connect`);
  },

  /**
   * Обновление баланса биржи
   */
  refreshBalance: async (name: ExchangeName): Promise<ExchangeAccount> => {
    const response = await apiClient.get<ExchangeAccount>(
      `/exchanges/${name}/balance`
    );
    return response.data;
  },
};

// ============================================
// Pairs API
// ============================================

export const pairsApi = {
  /**
   * Получение списка всех пар
   */
  getAll: async (): Promise<PairConfig[]> => {
    const response = await apiClient.get<PairConfig[]>('/pairs');
    return response.data;
  },

  /**
   * Получение конкретной пары по ID
   */
  getById: async (id: number): Promise<PairConfig> => {
    const response = await apiClient.get<PairConfig>(`/pairs/${id}`);
    return response.data;
  },

  /**
   * Создание новой пары
   */
  create: async (data: PairCreateRequest): Promise<PairConfig> => {
    const response = await apiClient.post<PairConfig>('/pairs', data);
    return response.data;
  },

  /**
   * Обновление параметров пары
   */
  update: async (id: number, data: PairUpdateRequest): Promise<PairConfig> => {
    const response = await apiClient.patch<PairConfig>(`/pairs/${id}`, data);
    return response.data;
  },

  /**
   * Удаление пары
   */
  delete: async (id: number): Promise<void> => {
    await apiClient.delete(`/pairs/${id}`);
  },

  /**
   * Запуск мониторинга пары
   */
  start: async (id: number): Promise<PairConfig> => {
    const response = await apiClient.post<PairConfig>(`/pairs/${id}/start`);
    return response.data;
  },

  /**
   * Приостановка пары
   */
  pause: async (id: number, forceClose = false): Promise<PairConfig> => {
    const response = await apiClient.post<PairConfig>(`/pairs/${id}/pause`, {
      forceClose,
    });
    return response.data;
  },
};

// ============================================
// Notifications API
// ============================================

export const notificationsApi = {
  /**
   * Получение списка уведомлений
   */
  getAll: async (
    types?: NotificationType[],
    limit = 100
  ): Promise<Notification[]> => {
    const params: Record<string, string> = { limit: String(limit) };
    if (types && types.length > 0) {
      params.types = types.join(',');
    }
    const response = await apiClient.get<Notification[]>('/notifications', {
      params,
    });
    return response.data;
  },

  /**
   * Очистка журнала уведомлений
   */
  clear: async (): Promise<void> => {
    await apiClient.delete('/notifications');
  },
};

// ============================================
// Stats API
// ============================================

export const statsApi = {
  /**
   * Получение агрегированной статистики
   */
  get: async (): Promise<Stats> => {
    const response = await apiClient.get<Stats>('/stats');
    return response.data;
  },

  /**
   * Получение топ-5 пар
   */
  getTopPairs: async (): Promise<{
    byTrades: Stats['topPairsByTrades'];
    byProfit: Stats['topPairsByProfit'];
    byLoss: Stats['topPairsByLoss'];
  }> => {
    const response = await apiClient.get<{
      byTrades: Stats['topPairsByTrades'];
      byProfit: Stats['topPairsByProfit'];
      byLoss: Stats['topPairsByLoss'];
    }>('/stats/top-pairs');
    return response.data;
  },

  /**
   * Сброс счетчиков статистики
   */
  reset: async (): Promise<void> => {
    await apiClient.post('/stats/reset');
  },
};

// ============================================
// Blacklist API
// ============================================

export const blacklistApi = {
  /**
   * Получение черного списка
   */
  getAll: async (): Promise<BlacklistEntry[]> => {
    const response = await apiClient.get<BlacklistEntry[]>('/blacklist');
    return response.data;
  },

  /**
   * Добавление в черный список
   */
  add: async (data: BlacklistCreateRequest): Promise<BlacklistEntry> => {
    const response = await apiClient.post<BlacklistEntry>('/blacklist', data);
    return response.data;
  },

  /**
   * Удаление из черного списка
   */
  remove: async (symbol: string): Promise<void> => {
    await apiClient.delete(`/blacklist/${symbol}`);
  },
};

// ============================================
// Settings API
// ============================================

export const settingsApi = {
  /**
   * Получение глобальных настроек
   */
  get: async (): Promise<Settings> => {
    const response = await apiClient.get<Settings>('/settings');
    return response.data;
  },

  /**
   * Обновление настроек
   */
  update: async (data: SettingsUpdateRequest): Promise<Settings> => {
    const response = await apiClient.patch<Settings>('/settings', data);
    return response.data;
  },
};

// ============================================
// Auth API (опционально)
// ============================================

export const authApi = {
  /**
   * Вход в систему
   */
  login: async (
    username: string,
    password: string
  ): Promise<{ token: string }> => {
    const response = await apiClient.post<{ token: string }>('/auth/login', {
      username,
      password,
    });
    localStorage.setItem('auth_token', response.data.token);
    return response.data;
  },

  /**
   * Выход из системы
   */
  logout: async (): Promise<void> => {
    await apiClient.post('/auth/logout');
    localStorage.removeItem('auth_token');
  },

  /**
   * Проверка авторизации
   */
  check: async (): Promise<boolean> => {
    try {
      await apiClient.get('/auth/check');
      return true;
    } catch {
      return false;
    }
  },
};

// Экспорт клиента для кастомных запросов
export { apiClient };
