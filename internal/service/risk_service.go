package service

// RiskService - бизнес-логика управления рисками
//
// ВАЖНО: Функционал управления рисками реализован в пакете bot, а не в service.
// См. internal/bot/risk.go для полной реализации:
//
// - RiskManager: централизованный менеджер рисков
//   - CheckStopLoss: проверка достижения Stop Loss (PNL ≤ -SL)
//   - HandleStopLoss: обработка срабатывания SL (закрытие позиций, пауза пары, уведомление)
//   - HandleLiquidation: обработка ликвидации (закрытие второй ноги, уведомление)
//   - CheckMarginRequirement: проверка маржинальных требований перед входом
//   - ValidateOrderLimits: проверка лимитов биржи (min/max volume, notional)
//
// - RiskMonitor: воркер для периодической проверки рисков
//   - Start: запуск мониторинга (каждые 500ms по умолчанию)
//   - checkAllRisks: проверка всех активных пар в состоянии HOLDING
//
// Архитектурное решение:
// RiskManager работает как часть торгового движка (bot package), а не как отдельный
// сервис, потому что:
// 1. Требует прямого доступа к PairState и runtime данным
// 2. Должен мгновенно реагировать на изменения (без сетевых запросов к БД)
// 3. Интегрирован с OrderExecutor для экстренного закрытия позиций
// 4. Использует in-memory кэш цен из PriceTracker
//
// Использование:
//
//	// В Engine при инициализации:
//	riskManager := bot.NewRiskManager(notifChan, closePosFn, pauseFn, bot.DefaultRiskConfig())
//	riskMonitor := bot.NewRiskMonitor(riskManager, engine.GetActivePairs)
//	go riskMonitor.Start(ctx)
//
// См. также:
// - internal/bot/position.go: PositionManager для мониторинга PNL и условий выхода
// - internal/bot/order.go: OrderExecutor для выполнения ордеров
