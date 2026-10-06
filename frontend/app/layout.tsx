import type { Metadata, Viewport } from 'next';
import { Bricolage_Grotesque, Lexend } from 'next/font/google';
import { Toaster } from 'react-hot-toast';
import { BootLoader } from '@/components/ui/States';
import { I18nProvider } from '@/lib/i18n';
import { LOCALE_SCRIPT } from '@/lib/locale';
import './globals.css';

// Bricolage Grotesque : états et heures (le poulailler est un projet bricolé maison).
// Lexend : texte courant, conçue pour la lisibilité (dehors, sur un téléphone).
const display = Bricolage_Grotesque({ subsets: ['latin'], variable: '--font-display', display: 'swap' });
const body = Lexend({ subsets: ['latin'], variable: '--font-body', display: 'swap' });

export const metadata: Metadata = {
  title: 'Solar Chicken',
  description: 'Omlet coop doors that follow the sun',
};

export const viewport: Viewport = {
  width: 'device-width',
  initialScale: 1,
  viewportFit: 'cover',
  themeColor: [
    { media: '(prefers-color-scheme: light)', color: '#E7EFF6' },
    { media: '(prefers-color-scheme: dark)', color: '#111829' },
  ],
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    // lang est corrigé par LOCALE_SCRIPT avant le premier affichage (d'où suppressHydrationWarning),
    // puis tenu à jour par I18nProvider.
    <html lang="en" className={`${display.variable} ${body.variable}`} suppressHydrationWarning>
      <head>
        <script dangerouslySetInnerHTML={{ __html: LOCALE_SCRIPT }} />
      </head>
      <body>
        <I18nProvider fallback={<BootLoader />}>{children}</I18nProvider>
        <Toaster
          position="top-center"
          toastOptions={{
            duration: 4000,
            className: '!bg-surface !text-ink !rounded-2xl !shadow-lg !ring-1 !ring-line',
            style: { maxWidth: '92vw' },
          }}
        />
      </body>
    </html>
  );
}
