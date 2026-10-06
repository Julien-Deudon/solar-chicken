import React from 'react';
import Link from 'next/link';
import { ChevronLeft } from 'lucide-react';

/** Lien de retour, avec une zone tactile confortable. */
export const BackLink: React.FC<{ href: string; children: React.ReactNode }> = ({ href, children }) => (
  <Link
    href={href}
    className="-ml-2 mb-1 inline-flex min-h-[44px] items-center gap-1.5 rounded-xl px-2 text-[15px] font-bold text-muted hover:bg-ink/5 hover:text-ink"
  >
    <ChevronLeft className="h-5 w-5" aria-hidden="true" />
    {children}
  </Link>
);
