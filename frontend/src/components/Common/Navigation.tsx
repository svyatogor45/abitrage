import React from 'react';
import { NavLink } from 'react-router-dom';
import clsx from 'clsx';
import { useUnreadNotificationsCount } from '@/hooks/useNotifications';

interface NavItem {
  path: string;
  label: string;
  icon: React.ReactNode;
}

const navItems: NavItem[] = [
  {
    path: '/',
    label: 'БОТ',
    icon: (
      <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path
          strokeLinecap="round"
          strokeLinejoin="round"
          strokeWidth={2}
          d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z"
        />
      </svg>
    ),
  },
  {
    path: '/api-stats',
    label: 'API / STATS',
    icon: (
      <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path
          strokeLinecap="round"
          strokeLinejoin="round"
          strokeWidth={2}
          d="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z"
        />
      </svg>
    ),
  },
  {
    path: '/notifications',
    label: 'Уведомления',
    icon: (
      <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path
          strokeLinecap="round"
          strokeLinejoin="round"
          strokeWidth={2}
          d="M15 17h5l-1.405-1.405A2.032 2.032 0 0118 14.158V11a6.002 6.002 0 00-4-5.659V5a2 2 0 10-4 0v.341C7.67 6.165 6 8.388 6 11v3.159c0 .538-.214 1.055-.595 1.436L4 17h5m6 0v1a3 3 0 11-6 0v-1m6 0H9"
        />
      </svg>
    ),
  },
];

/**
 * Навигация по вкладкам
 */
export const Navigation: React.FC = () => {
  const { count: unreadCount, resetCount } = useUnreadNotificationsCount();

  return (
    <nav className="bg-surface border-b border-border">
      <div className="flex">
        {navItems.map((item) => (
          <NavLink
            key={item.path}
            to={item.path}
            onClick={() => {
              if (item.path === '/notifications') {
                resetCount();
              }
            }}
            className={({ isActive }) =>
              clsx(
                'flex items-center gap-2 px-6 py-4 text-sm font-medium transition-colors relative',
                'hover:bg-gray-800/50',
                isActive
                  ? 'text-primary border-b-2 border-primary'
                  : 'text-gray-400 hover:text-white'
              )
            }
          >
            {item.icon}
            <span>{item.label}</span>

            {/* Бейдж для непрочитанных уведомлений */}
            {item.path === '/notifications' && unreadCount > 0 && (
              <span className="absolute top-2 right-2 min-w-[20px] h-5 px-1.5 flex items-center justify-center bg-red-500 text-white text-xs font-bold rounded-full">
                {unreadCount > 99 ? '99+' : unreadCount}
              </span>
            )}
          </NavLink>
        ))}
      </div>
    </nav>
  );
};

export default Navigation;
