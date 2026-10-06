'use client';

import React from 'react';
import { Navbar } from '@/components/layout/Navbar';
import { PageLoader } from '@/components/ui/States';
import { useRequireAuth } from '@/lib/hooks';

/**
 * Cadre des pages connectées : vérifie le jeton (sinon /login), navigation,
 * colonne lisible sur téléphone, deux colonnes possibles sur grand écran.
 */
export const AppShell: React.FC<{ children: React.ReactNode; wide?: boolean }> = ({ children, wide = false }) => {
  const ready = useRequireAuth();

  if (!ready) return <PageLoader />;

  return (
    <div className="min-h-screen">
      <Navbar />
      <main className={`mx-auto px-4 pb-28 pt-5 sm:px-6 sm:pb-12 sm:pt-8 ${wide ? 'max-w-5xl' : 'max-w-2xl'}`}>{children}</main>
    </div>
  );
};
