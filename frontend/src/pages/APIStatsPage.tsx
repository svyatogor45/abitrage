import React from 'react';
import { ExchangeList } from '@/components/API/ExchangeList';
import { StatsOverview } from '@/components/Stats/StatsOverview';
import { TopPairs } from '@/components/Stats/TopPairs';
import { EventsList } from '@/components/Stats/EventsList';

/**
 * Страница API / STATS
 */
export const APIStatsPage: React.FC = () => {
  return (
    <div className="p-6 space-y-8">
      {/* Секция API */}
      <section>
        <ExchangeList />
      </section>

      {/* Разделитель */}
      <hr className="border-border" />

      {/* Секция STATS */}
      <section className="space-y-6">
        <h2 className="text-xl font-bold text-white">Статистика</h2>

        {/* Обзор статистики */}
        <StatsOverview />

        {/* Топ-5 пар */}
        <TopPairs />

        {/* События SL и ликвидации */}
        <EventsList />
      </section>
    </div>
  );
};

export default APIStatsPage;
