// ============================================
// Биржи и аккаунты
// ============================================

export type ExchangeName = 'bybit' | 'bitget' | 'okx' | 'gate' | 'htx' | 'bingx';

export interface ExchangeAccount {
  id: number;
  name: ExchangeName;
  connected: boolean;
  balance: number;
  lastError?: string;
  updatedAt: string;
  createdAt: string;
}

export interface ExchangeConnectRequest {
  apiKey: string;
  secretKey: string;
  passphrase?: string; // Для OKX
}

// ============================================
// Торговые пары
// ============================================

export type PairStatus = 'paused' | 'active';
export type PairState = 'PAUSED' | 'READY' | 'ENTERING' | 'HOLDING' | 'EXITING' | 'ERROR';

export interface PairConfig {
  id: number;
  symbol: string;
  base: string;
  quote: string;
  entrySpread: number;
  exitSpread: number;
  volume: number;
  nOrders: number;
  stopLoss: number | null;
  status: PairStatus;
  tradesCount: number;
  totalPnl: number;
  createdAt: string;
  updatedAt: string;
}

export interface PairCreateRequest {
  base: string;
  quote: string;
  entrySpread: number;
  exitSpread: number;
  volume: number;
  nOrders: number;
  stopLoss?: number;
}

export interface PairUpdateRequest {
  entrySpread?: number;
  exitSpread?: number;
  volume?: number;
  nOrders?: number;
  stopLoss?: number | null;
}

export interface Leg {
  exchange: ExchangeName;
  side: 'long' | 'short';
  entryPrice: number;
  currentPrice: number;
  quantity: number;
  unrealizedPnl: number;
}

export interface PairRuntime {
  pairId: number;
  state: PairState;
  legs: Leg[];
  filledParts: number;
  currentSpread: number;
  unrealizedPnl: number;
  realizedPnl: number;
  lastUpdate: string;
}

// Комбинированный тип для отображения в UI
export interface PairWithRuntime extends PairConfig {
  runtime?: PairRuntime;
}

// ============================================
// Уведомления
// ============================================

export type NotificationType =
  | 'OPEN'
  | 'CLOSE'
  | 'SL'
  | 'LIQUIDATION'
  | 'ERROR'
  | 'MARGIN'
  | 'PAUSE'
  | 'SECOND_LEG_FAIL';

export type NotificationSeverity = 'info' | 'warn' | 'error';

export interface Notification {
  id: number;
  timestamp: string;
  type: NotificationType;
  severity: NotificationSeverity;
  pairId?: number;
  message: string;
  meta?: Record<string, unknown>;
}

// ============================================
// Статистика
// ============================================

export interface StopLossEvent {
  symbol: string;
  exchanges: [string, string];
  timestamp: string;
}

export interface LiquidationEvent {
  symbol: string;
  exchange: string;
  side: 'long' | 'short';
  timestamp: string;
}

export interface StopLossStats {
  today: number;
  week: number;
  month: number;
  events: StopLossEvent[];
}

export interface LiquidationStats {
  today: number;
  week: number;
  month: number;
  events: LiquidationEvent[];
}

export interface PairStat {
  symbol: string;
  value: number;
}

export interface Stats {
  totalTrades: number;
  totalPnl: number;
  todayTrades: number;
  todayPnl: number;
  weekTrades: number;
  weekPnl: number;
  monthTrades: number;
  monthPnl: number;
  stopLossStats: StopLossStats;
  liquidationStats: LiquidationStats;
  topPairsByTrades: PairStat[];
  topPairsByProfit: PairStat[];
  topPairsByLoss: PairStat[];
}

// ============================================
// Настройки
// ============================================

export interface NotificationPreferences {
  open: boolean;
  close: boolean;
  stopLoss: boolean;
  liquidation: boolean;
  apiError: boolean;
  margin: boolean;
  pause: boolean;
  secondLegFail: boolean;
}

export interface Settings {
  id: number;
  considerFunding: boolean;
  maxConcurrentTrades: number | null;
  notificationPrefs: NotificationPreferences;
  updatedAt: string;
}

export interface SettingsUpdateRequest {
  considerFunding?: boolean;
  maxConcurrentTrades?: number | null;
  notificationPrefs?: Partial<NotificationPreferences>;
}

// ============================================
// Черный список
// ============================================

export interface BlacklistEntry {
  id: number;
  symbol: string;
  reason?: string;
  createdAt: string;
}

export interface BlacklistCreateRequest {
  symbol: string;
  reason?: string;
}

// ============================================
// WebSocket сообщения
// ============================================

export type WebSocketMessageType =
  | 'pairUpdate'
  | 'notification'
  | 'balanceUpdate'
  | 'statsUpdate';

export interface WebSocketMessage<T = unknown> {
  type: WebSocketMessageType;
  data: T;
}

export interface PairUpdateMessage {
  id: number;
  state: PairState;
  currentSpread: number;
  unrealizedPnl: number;
  legs?: Leg[];
}

export interface BalanceUpdateMessage {
  exchange: ExchangeName;
  balance: number;
}

export interface StatsUpdateMessage {
  totalTrades: number;
  totalPnl: number;
  todayTrades: number;
  todayPnl: number;
}

// ============================================
// API ответы
// ============================================

export interface ApiResponse<T> {
  data: T;
  message?: string;
}

export interface ApiError {
  error: string;
  code?: string;
  details?: Record<string, string>;
}

export interface PaginatedResponse<T> {
  data: T[];
  total: number;
  page: number;
  pageSize: number;
}

// ============================================
// UI состояния
// ============================================

export interface LoadingState {
  isLoading: boolean;
  error: string | null;
}

export interface ModalState {
  isOpen: boolean;
  data?: unknown;
}

// ============================================
// Фильтры уведомлений
// ============================================

export interface NotificationFilters {
  open: boolean;
  close: boolean;
  stopLoss: boolean;
  liquidation: boolean;
  apiError: boolean;
  margin: boolean;
  pause: boolean;
  secondLegFail: boolean;
}

// Дефолтные значения фильтров
export const DEFAULT_NOTIFICATION_FILTERS: NotificationFilters = {
  open: true,
  close: true,
  stopLoss: true,
  liquidation: true,
  apiError: true,
  margin: true,
  pause: true,
  secondLegFail: true,
};

// ============================================
// Константы
// ============================================

export const EXCHANGE_NAMES: Record<ExchangeName, string> = {
  bybit: 'Bybit',
  bitget: 'Bitget',
  okx: 'OKX',
  gate: 'Gate.io',
  htx: 'HTX',
  bingx: 'BingX',
};

export const NOTIFICATION_TYPE_LABELS: Record<NotificationType, string> = {
  OPEN: 'Открытие сделки',
  CLOSE: 'Закрытие сделки',
  SL: 'Stop Loss',
  LIQUIDATION: 'Ликвидация',
  ERROR: 'Ошибка API',
  MARGIN: 'Недостаток маржи',
  PAUSE: 'Пауза/Остановка',
  SECOND_LEG_FAIL: 'Вторая нога не открыта',
};

export const PAIR_STATE_LABELS: Record<PairState, string> = {
  PAUSED: 'На паузе',
  READY: 'Ожидание условий',
  ENTERING: 'Вход в позицию',
  HOLDING: 'Позиция открыта',
  EXITING: 'Закрытие позиции',
  ERROR: 'Ошибка',
};

export const MAX_PAIRS = 30;
export const MAX_NOTIFICATIONS = 100;
export const BALANCE_UPDATE_INTERVAL = 60000; // 1 минута
export const PRICE_UPDATE_INTERVAL = 1000; // 1 секунда
