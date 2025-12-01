import type {
  WebSocketMessage,
  PairUpdateMessage,
  BalanceUpdateMessage,
  StatsUpdateMessage,
  Notification,
  WebSocketMessageType,
} from '@/types';

type MessageHandler<T = unknown> = (data: T) => void;

interface WebSocketHandlers {
  pairUpdate: Set<MessageHandler<PairUpdateMessage>>;
  notification: Set<MessageHandler<Notification>>;
  balanceUpdate: Set<MessageHandler<BalanceUpdateMessage>>;
  statsUpdate: Set<MessageHandler<StatsUpdateMessage>>;
}

interface WebSocketServiceConfig {
  url?: string;
  reconnectInterval?: number;
  maxReconnectAttempts?: number;
  pingInterval?: number;
}

const DEFAULT_CONFIG: Required<WebSocketServiceConfig> = {
  url: `${window.location.protocol === 'https:' ? 'wss:' : 'ws:'}//${window.location.host}/ws/stream`,
  reconnectInterval: 3000,
  maxReconnectAttempts: 10,
  pingInterval: 30000,
};

class WebSocketService {
  private socket: WebSocket | null = null;
  private config: Required<WebSocketServiceConfig>;
  private reconnectAttempts = 0;
  private reconnectTimeout: ReturnType<typeof setTimeout> | null = null;
  private pingTimeout: ReturnType<typeof setInterval> | null = null;
  private isManualClose = false;

  private handlers: WebSocketHandlers = {
    pairUpdate: new Set(),
    notification: new Set(),
    balanceUpdate: new Set(),
    statsUpdate: new Set(),
  };

  private connectionHandlers = {
    onOpen: new Set<() => void>(),
    onClose: new Set<() => void>(),
    onError: new Set<(error: Event) => void>(),
  };

  constructor(config: WebSocketServiceConfig = {}) {
    this.config = { ...DEFAULT_CONFIG, ...config };
  }

  /**
   * Подключение к WebSocket серверу
   */
  connect(): void {
    if (this.socket?.readyState === WebSocket.OPEN) {
      console.warn('WebSocket уже подключен');
      return;
    }

    this.isManualClose = false;
    this.createConnection();
  }

  /**
   * Отключение от WebSocket сервера
   */
  disconnect(): void {
    this.isManualClose = true;
    this.cleanup();

    if (this.socket) {
      this.socket.close(1000, 'Manual disconnect');
      this.socket = null;
    }
  }

  /**
   * Проверка состояния подключения
   */
  isConnected(): boolean {
    return this.socket?.readyState === WebSocket.OPEN;
  }

  /**
   * Подписка на обновления пар
   */
  onPairUpdate(handler: MessageHandler<PairUpdateMessage>): () => void {
    this.handlers.pairUpdate.add(handler);
    return () => this.handlers.pairUpdate.delete(handler);
  }

  /**
   * Подписка на уведомления
   */
  onNotification(handler: MessageHandler<Notification>): () => void {
    this.handlers.notification.add(handler);
    return () => this.handlers.notification.delete(handler);
  }

  /**
   * Подписка на обновления балансов
   */
  onBalanceUpdate(handler: MessageHandler<BalanceUpdateMessage>): () => void {
    this.handlers.balanceUpdate.add(handler);
    return () => this.handlers.balanceUpdate.delete(handler);
  }

  /**
   * Подписка на обновления статистики
   */
  onStatsUpdate(handler: MessageHandler<StatsUpdateMessage>): () => void {
    this.handlers.statsUpdate.add(handler);
    return () => this.handlers.statsUpdate.delete(handler);
  }

  /**
   * Подписка на событие открытия соединения
   */
  onOpen(handler: () => void): () => void {
    this.connectionHandlers.onOpen.add(handler);
    return () => this.connectionHandlers.onOpen.delete(handler);
  }

  /**
   * Подписка на событие закрытия соединения
   */
  onClose(handler: () => void): () => void {
    this.connectionHandlers.onClose.add(handler);
    return () => this.connectionHandlers.onClose.delete(handler);
  }

  /**
   * Подписка на ошибки соединения
   */
  onError(handler: (error: Event) => void): () => void {
    this.connectionHandlers.onError.add(handler);
    return () => this.connectionHandlers.onError.delete(handler);
  }

  /**
   * Отправка сообщения на сервер
   */
  send(message: unknown): void {
    if (this.socket?.readyState !== WebSocket.OPEN) {
      console.error('WebSocket не подключен');
      return;
    }

    this.socket.send(JSON.stringify(message));
  }

  private createConnection(): void {
    try {
      // Добавление токена авторизации в URL
      const token = localStorage.getItem('auth_token');
      const url = new URL(this.config.url);
      if (token) {
        url.searchParams.set('token', token);
      }

      this.socket = new WebSocket(url.toString());

      this.socket.onopen = this.handleOpen.bind(this);
      this.socket.onclose = this.handleClose.bind(this);
      this.socket.onerror = this.handleError.bind(this);
      this.socket.onmessage = this.handleMessage.bind(this);
    } catch (error) {
      console.error('Ошибка создания WebSocket:', error);
      this.scheduleReconnect();
    }
  }

  private handleOpen(): void {
    console.log('WebSocket подключен');
    this.reconnectAttempts = 0;
    this.startPing();

    this.connectionHandlers.onOpen.forEach((handler) => handler());
  }

  private handleClose(event: CloseEvent): void {
    console.log(`WebSocket закрыт: код ${event.code}, причина: ${event.reason}`);
    this.cleanup();

    this.connectionHandlers.onClose.forEach((handler) => handler());

    if (!this.isManualClose) {
      this.scheduleReconnect();
    }
  }

  private handleError(event: Event): void {
    console.error('WebSocket ошибка:', event);

    this.connectionHandlers.onError.forEach((handler) => handler(event));
  }

  private handleMessage(event: MessageEvent): void {
    try {
      const message: WebSocketMessage = JSON.parse(event.data);
      this.dispatchMessage(message);
    } catch (error) {
      console.error('Ошибка парсинга WebSocket сообщения:', error);
    }
  }

  private dispatchMessage(message: WebSocketMessage): void {
    const { type, data } = message;

    switch (type as WebSocketMessageType) {
      case 'pairUpdate':
        this.handlers.pairUpdate.forEach((handler) =>
          handler(data as PairUpdateMessage)
        );
        break;

      case 'notification':
        this.handlers.notification.forEach((handler) =>
          handler(data as Notification)
        );
        break;

      case 'balanceUpdate':
        this.handlers.balanceUpdate.forEach((handler) =>
          handler(data as BalanceUpdateMessage)
        );
        break;

      case 'statsUpdate':
        this.handlers.statsUpdate.forEach((handler) =>
          handler(data as StatsUpdateMessage)
        );
        break;

      default:
        console.warn('Неизвестный тип WebSocket сообщения:', type);
    }
  }

  private scheduleReconnect(): void {
    if (this.reconnectAttempts >= this.config.maxReconnectAttempts) {
      console.error('Достигнуто максимальное количество попыток переподключения');
      return;
    }

    this.reconnectAttempts++;
    const delay = this.config.reconnectInterval * Math.pow(2, this.reconnectAttempts - 1);

    console.log(
      `Переподключение через ${delay}мс (попытка ${this.reconnectAttempts}/${this.config.maxReconnectAttempts})`
    );

    this.reconnectTimeout = setTimeout(() => {
      this.createConnection();
    }, delay);
  }

  private startPing(): void {
    this.pingTimeout = setInterval(() => {
      if (this.socket?.readyState === WebSocket.OPEN) {
        this.send({ type: 'ping' });
      }
    }, this.config.pingInterval);
  }

  private cleanup(): void {
    if (this.reconnectTimeout) {
      clearTimeout(this.reconnectTimeout);
      this.reconnectTimeout = null;
    }

    if (this.pingTimeout) {
      clearInterval(this.pingTimeout);
      this.pingTimeout = null;
    }
  }
}

// Singleton экземпляр
export const websocketService = new WebSocketService();

// Экспорт класса для создания дополнительных инстансов
export { WebSocketService };
