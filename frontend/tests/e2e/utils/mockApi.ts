import type { Page, Route } from '@playwright/test';
import type {
  ExchangeAccount,
  PairConfig,
  PairStatus,
  PairCreateRequest,
} from '@/types';

interface MockState {
  exchanges: ExchangeAccount[];
  pairs: PairConfig[];
}

const iso = () => new Date().toISOString();

const clone = <T>(data: T): T => JSON.parse(JSON.stringify(data));

const defaultState: MockState = {
  exchanges: [
    {
      id: 1,
      name: 'bybit',
      connected: true,
      balance: 1500,
      updatedAt: iso(),
      createdAt: iso(),
    },
    {
      id: 2,
      name: 'bitget',
      connected: true,
      balance: 1200,
      updatedAt: iso(),
      createdAt: iso(),
    },
  ],
  pairs: [
    {
      id: 1,
      symbol: 'BTCUSDT',
      base: 'BTC',
      quote: 'USDT',
      entrySpread: 1.0,
      exitSpread: 0.2,
      volume: 0.5,
      nOrders: 1,
      stopLoss: 100,
      status: 'paused',
      tradesCount: 3,
      totalPnl: 45,
      createdAt: iso(),
      updatedAt: iso(),
    },
  ],
};

type RouteHandler = (route: Route) => Promise<void>;

const withJson =
  <T>(body: T, status = 200): Parameters<Route['fulfill']>[0] =>
  ({
    status,
    contentType: 'application/json',
    body: JSON.stringify(body),
  });

const parseBody = <T>(route: Route): T =>
  JSON.parse(route.request().postData() || '{}');

export async function setupBotPageMocks(
  page: Page,
  overrides?: Partial<MockState>
) {
  const state: MockState = {
    exchanges: overrides?.exchanges ?? clone(defaultState.exchanges),
    pairs: overrides?.pairs ?? clone(defaultState.pairs),
  };

  await page.route('**/api/exchanges', async (route) => {
    if (route.request().method() === 'GET') {
      await route.fulfill(withJson(state.exchanges));
      return;
    }

    await route.continue();
  });

  const handlePairs: RouteHandler = async (route) => {
    const method = route.request().method();
    const url = new URL(route.request().url());

    if (method === 'GET') {
      await route.fulfill(withJson(state.pairs));
      return;
    }

    if (method === 'POST' && url.pathname.endsWith('/pairs')) {
      const payload = parseBody<PairCreateRequest>(route);
      const newPair: PairConfig = {
        id: state.pairs.length + 1,
        symbol: `${payload.base}${payload.quote}`,
        tradesCount: 0,
        totalPnl: 0,
        status: 'paused',
        createdAt: iso(),
        updatedAt: iso(),
        ...payload,
      };
      state.pairs.push(newPair);
      await route.fulfill(withJson(newPair, 201));
      return;
    }

    const match = url.pathname.match(/\/pairs\/(\d+)\/(start|pause)$/);
    if (match) {
      const [, idStr, action] = match;
      const id = Number(idStr);
      const target = state.pairs.find((pair) => pair.id === id);
      if (!target) {
        await route.fulfill({ status: 404 });
        return;
      }

      const nextStatus: PairStatus = action === 'start' ? 'active' : 'paused';
      target.status = nextStatus;
      target.updatedAt = iso();
      await route.fulfill(withJson(target));
      return;
    }

    await route.continue();
  };

  await page.route('**/api/pairs*', handlePairs);

  return state;
}

