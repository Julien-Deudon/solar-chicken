'use client';

import React from 'react';
import { Notice } from '@/components/ui/States';
import { useI18n } from '@/lib/i18n';

/** Rappel discret tant que la v2 tourne en mode observation (mode === 'shadow'). */
export const ShadowBanner: React.FC = () => {
  const { t } = useI18n();
  return (
    <Notice tone="info">
      <strong>{t('shadow.title')}</strong> {t('shadow.text')}
    </Notice>
  );
};
