'use client';

import React, { useState } from 'react';
import { useRouter } from 'next/navigation';
import toast from 'react-hot-toast';
import { Mail } from 'lucide-react';
import { Card } from '@/components/ui/Card';
import { Button } from '@/components/ui/Button';
import { Input } from '@/components/ui/Input';
import { authApi, getErrorMessage } from '@/lib/api';
import { useI18n } from '@/lib/i18n';

export default function ResendVerificationPage() {
  const { t } = useI18n();
  const router = useRouter();
  const [email, setEmail] = useState('');
  const [isLoading, setIsLoading] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setIsLoading(true);
    try {
      await authApi.resendVerification(email.trim());
      toast.success(t('resend.sent'));
      setTimeout(() => router.push('/login'), 2000);
    } catch (error) {
      toast.error(getErrorMessage(error, 'resend.failed'));
      setIsLoading(false);
    }
  };

  return (
    <div className="flex min-h-screen items-center justify-center bg-canvas p-4">
      <Card className="w-full max-w-md">
        <div className="mb-6 text-center">
          <div className="mx-auto mb-4 flex h-16 w-16 items-center justify-center rounded-full bg-ink">
            <Mail className="h-8 w-8 text-white" aria-hidden="true" />
          </div>
          <h1 className="text-2xl font-bold text-ink">{t('resend.title')}</h1>
          <p className="mt-2 text-muted">{t('resend.intro')}</p>
        </div>
        <form onSubmit={handleSubmit} className="space-y-3">
          <Input
            label={t('resend.email')}
            type="email"
            autoComplete="email"
            inputMode="email"
            autoCapitalize="off"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            placeholder={t('resend.placeholder')}
            required
          />
          <Button type="submit" className="w-full" isLoading={isLoading} loadingText={t('common.sending')}>
            {t('resend.submit')}
          </Button>
          <Button variant="secondary" className="w-full" onClick={() => router.push('/login')}>
            {t('auth.backToLogin')}
          </Button>
        </form>
      </Card>
    </div>
  );
}
