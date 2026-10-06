'use client';

import React from 'react';
import { AlertTriangle, Info, RefreshCw } from 'lucide-react';
import { Button } from '@/components/ui/Button';
import { useI18n } from '@/lib/i18n';

export const Spinner: React.FC<{ className?: string; label?: string }> = ({ className = 'h-8 w-8', label }) => {
  const { t } = useI18n();
  return (
    <span role="status" className="inline-flex items-center">
      <span className={`animate-spin rounded-full border-[3px] border-sun border-t-transparent ${className}`} aria-hidden="true" />
      <span className="sr-only">{label ?? t('common.loading')}</span>
    </span>
  );
};

export const PageLoader: React.FC<{ label?: string }> = ({ label }) => {
  const { t } = useI18n();
  const text = label ?? t('common.loading');
  return (
    <div className="flex flex-col items-center justify-center gap-3 py-24 text-muted">
      <Spinner className="h-9 w-9" label={text} />
      <p className="text-[15px]">{text}</p>
    </div>
  );
};

/**
 * Avant que la langue soit connue (premier affichage, voir I18nProvider) : la roue seule,
 * sans texte, exactement à la place de celle de PageLoader.
 */
export const BootLoader: React.FC = () => (
  <div className="flex flex-col items-center justify-center gap-3 py-24" aria-busy="true">
    <span className="inline-flex items-center">
      <span className="h-9 w-9 animate-spin rounded-full border-[3px] border-sun border-t-transparent" aria-hidden="true" />
    </span>
  </div>
);

interface ErrorStateProps {
  title?: string;
  message: string;
  onRetry?: () => void;
  children?: React.ReactNode;
}

export const ErrorState: React.FC<ErrorStateProps> = ({ title, message, onRetry, children }) => {
  const { t } = useI18n();
  return (
    <div role="alert" className="rounded-sheet bg-surface p-5">
      <div className="flex items-start gap-3">
        <AlertTriangle className="mt-1 h-5 w-5 shrink-0 text-alert" aria-hidden="true" />
        <div className="min-w-0 flex-1">
          <p className="font-display text-xl font-bold">{title ?? t('common.loadFailed')}</p>
          <p className="mt-1 break-words text-[15px] text-muted">{message}</p>
          {(onRetry || children) && (
            <div className="mt-4 flex flex-wrap gap-2">
              {onRetry && (
                <Button variant="outline" size="sm" onClick={onRetry}>
                  <RefreshCw className="h-4 w-4" aria-hidden="true" />
                  {t('common.retry')}
                </Button>
              )}
              {children}
            </div>
          )}
        </div>
      </div>
    </div>
  );
};

/** Note compacte : information (bleu nuit pâle), avertissement (jaune) ou erreur (rouge). */
export const Notice: React.FC<{
  tone?: 'warning' | 'error' | 'info';
  title?: React.ReactNode;
  children: React.ReactNode;
  className?: string;
}> = ({ tone = 'warning', title, children, className = '' }) => {
  const tones = {
    warning: 'bg-sun/20 text-ink',
    error: 'bg-alert/12 text-ink',
    info: 'bg-info text-ink',
  };
  const Icon = tone === 'info' ? Info : AlertTriangle;
  const iconTone = { warning: 'text-ink', error: 'text-alert', info: 'text-muted' }[tone];
  return (
    <div role={tone === 'info' ? 'note' : 'alert'} className={`flex gap-2.5 rounded-2xl px-3.5 py-3 text-[15px] leading-snug ${tones[tone]} ${className}`}>
      <Icon className={`mt-0.5 h-[18px] w-[18px] shrink-0 ${iconTone}`} aria-hidden="true" />
      <div className="min-w-0 break-words">
        {title && <p className="font-bold">{title}</p>}
        {children}
      </div>
    </div>
  );
};
