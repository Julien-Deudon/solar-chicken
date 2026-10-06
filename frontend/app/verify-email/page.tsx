'use client';

import React, { Suspense, useEffect, useRef, useState } from 'react';
import { useRouter, useSearchParams } from 'next/navigation';
import { CheckCircle, Loader2, XCircle } from 'lucide-react';
import { Card } from '@/components/ui/Card';
import { Button } from '@/components/ui/Button';
import { authApi, getErrorMessage } from '@/lib/api';
import { useI18n } from '@/lib/i18n';

function VerifyEmailContent() {
  const { t } = useI18n();
  const router = useRouter();
  const searchParams = useSearchParams();
  const token = searchParams.get('token');

  const [status, setStatus] = useState<'loading' | 'success' | 'error'>('loading');
  // Message d'erreur du serveur ; null : lien sans jeton.
  const [error, setError] = useState<string | null>(null);
  const started = useRef(false);

  useEffect(() => {
    if (!token) {
      setStatus('error');
      setError(null);
      return;
    }
    // Un seul appel, même en mode strict (double effet en développement).
    if (started.current) return;
    started.current = true;

    let timer: number | undefined;
    authApi
      .verifyEmail(token)
      .then(() => {
        setStatus('success');
        timer = window.setTimeout(() => router.push('/login'), 3000);
      })
      .catch((e) => {
        setStatus('error');
        setError(getErrorMessage(e, 'verify.failed'));
      });
    return () => window.clearTimeout(timer);
  }, [token, router]);

  return (
    <div className="flex min-h-screen items-center justify-center bg-canvas p-4">
      <Card className="w-full max-w-md">
        <div className="py-6 text-center" aria-live="polite">
          {status === 'loading' && (
            <>
              <Loader2 className="mx-auto mb-6 h-16 w-16 animate-spin text-ink" aria-hidden="true" />
              <h1 className="mb-2 text-2xl font-bold text-ink">{t('verify.inProgress')}</h1>
              <p className="text-muted">{t('verify.wait')}</p>
            </>
          )}

          {status === 'success' && (
            <>
              <CheckCircle className="mx-auto mb-6 h-16 w-16 text-ok" aria-hidden="true" />
              <h1 className="mb-2 text-2xl font-bold text-ink">{t('verify.successTitle')}</h1>
              <p className="mb-2 text-muted">{t('verify.verified')}</p>
              <p className="mb-6 text-sm text-muted">{t('verify.redirecting')}</p>
              <Button onClick={() => router.push('/login')} className="w-full">
                {t('verify.signInNow')}
              </Button>
            </>
          )}

          {status === 'error' && (
            <>
              <XCircle className="mx-auto mb-6 h-16 w-16 text-alert" aria-hidden="true" />
              <h1 className="mb-2 text-2xl font-bold text-ink">{t('verify.errorTitle')}</h1>
              <p className="mb-6 text-muted">{error ?? t('verify.missingToken')}</p>
              <div className="space-y-3">
                <Button onClick={() => router.push('/login')} className="w-full">
                  {t('auth.backToLogin')}
                </Button>
                <Button variant="secondary" onClick={() => router.push('/resend-verification')} className="w-full">
                  {t('resend.title')}
                </Button>
              </div>
            </>
          )}
        </div>
      </Card>
    </div>
  );
}

export default function VerifyEmailPage() {
  return (
    <Suspense
      fallback={
        <div className="flex min-h-screen items-center justify-center bg-canvas">
          <Loader2 className="h-16 w-16 animate-spin text-ink" aria-hidden="true" />
        </div>
      }
    >
      <VerifyEmailContent />
    </Suspense>
  );
}
