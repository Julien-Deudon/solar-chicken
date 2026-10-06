'use client';

import React from 'react';
import Link from 'next/link';
import { usePathname } from 'next/navigation';
import { Egg, SlidersHorizontal } from 'lucide-react';
import { useI18n, type MessageKey } from '@/lib/i18n';

const TABS: { href: string; labelKey: MessageKey; icon: typeof Egg; match: string[] }[] = [
  { href: '/dashboard', labelKey: 'nav.coop', icon: Egg, match: ['/dashboard', '/coops', '/devices'] },
  { href: '/settings', labelKey: 'nav.settings', icon: SlidersHorizontal, match: ['/settings'] },
];

/**
 * Navigation principale : barre d'onglets en bas sur téléphone (sous le pouce),
 * barre fine en haut à partir de la tablette.
 */
export const Navbar: React.FC = () => {
  const { t } = useI18n();
  const pathname = usePathname() ?? '';
  const isActive = (match: string[]) => match.some((p) => pathname.startsWith(p));

  return (
    <>
      <header className="sticky top-0 z-30 hidden border-b border-line/70 bg-canvas/90 backdrop-blur sm:block">
        <nav className="mx-auto flex h-14 max-w-5xl items-center justify-between px-6" aria-label={t('nav.main')}>
          <Link href="/dashboard" className="font-display text-xl font-extrabold tracking-tight">
            Solar Chicken
          </Link>
          <div className="flex gap-1">
            {TABS.map(({ href, labelKey, icon: Icon, match }) => (
              <Link
                key={href}
                href={href}
                aria-current={isActive(match) ? 'page' : undefined}
                className={`inline-flex min-h-[40px] items-center gap-2 rounded-xl px-3.5 text-[15px] font-bold ${
                  isActive(match) ? 'bg-ink text-canvas' : 'text-muted hover:bg-ink/5 hover:text-ink'
                }`}
              >
                <Icon className="h-4 w-4" aria-hidden="true" />
                {t(labelKey)}
              </Link>
            ))}
          </div>
        </nav>
      </header>

      <nav
        className="pb-safe fixed inset-x-0 bottom-0 z-30 border-t border-line/70 bg-canvas/95 backdrop-blur sm:hidden"
        aria-label={t('nav.main')}
      >
        <div className="mx-auto grid max-w-md grid-cols-2 px-6 pt-1.5">
          {TABS.map(({ href, labelKey, icon: Icon, match }) => {
            const active = isActive(match);
            return (
              <Link
                key={href}
                href={href}
                aria-current={active ? 'page' : undefined}
                className={`flex min-h-[52px] flex-col items-center justify-center gap-0.5 rounded-xl text-[13px] font-bold ${
                  active ? 'text-ink' : 'text-muted'
                }`}
              >
                <span className={`flex h-8 w-14 items-center justify-center rounded-full ${active ? 'bg-sun/35' : ''}`}>
                  <Icon className="h-5 w-5" aria-hidden="true" />
                </span>
                {t(labelKey)}
              </Link>
            );
          })}
        </div>
      </nav>
    </>
  );
};
