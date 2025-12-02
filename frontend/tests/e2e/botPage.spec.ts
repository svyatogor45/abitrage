import { test, expect } from '@playwright/test';
import { setupBotPageMocks } from './utils/mockApi';

test.describe('BotPage scenarios', () => {
  test('user can add a new pair', async ({ page }) => {
    await setupBotPageMocks(page, { pairs: [] });

    await page.goto('/bot');

    await page.getByRole('button', { name: 'Добавить пару' }).click();

    await page.getByLabel('Базовая валюта').fill('ETH');
    await page.getByLabel(/Рабочий объем/).fill('0.3');
    await page.getByLabel('Спред входа (%)').fill('1.5');
    await page.getByLabel('Спред выхода (%)').fill('0.3');
    await page.getByLabel('Количество ордеров (разбиение)').fill('2');
    await page.getByLabel(/Stop Loss/).fill('80');

    await page.getByRole('button', { name: 'Добавить', exact: true }).click();

    await expect(page.getByText('ETHUSDT')).toBeVisible();
    await expect(
      page.getByText('Р.О: 0.3 ETH', { exact: false })
    ).toBeVisible();
  });

  test('user can start and pause a pair', async ({ page }) => {
    await setupBotPageMocks(page);

    await page.goto('/bot');

    const startButton = page.getByRole('button', { name: 'Старт' });
    await expect(startButton).toBeVisible();
    await startButton.click();

    const pauseButton = page.getByRole('button', { name: 'Пауза' });
    await expect(pauseButton).toBeVisible();
    await pauseButton.click();

    await expect(page.getByRole('button', { name: 'Старт' })).toBeVisible();
  });
});

