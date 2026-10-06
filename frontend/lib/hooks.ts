'use client';

import { useCallback, useEffect, useRef, useState } from 'react';
import { useRouter } from 'next/navigation';
import { useAuthStore } from '@/lib/store';
import { devicesApi, findDeviceContext, getErrorMessage } from '@/lib/api';
import { stateFromStatus } from '@/lib/coop';
import type { CoopDetail, Device, DeviceStateView, DeviceStatus } from '@/types';

/**
 * Redirige vers /login sans jeton. Renvoie false tant que la vérification n'est pas faite
 * (premier rendu identique côté serveur et client : pas d'écart d'hydratation).
 */
export function useRequireAuth(): boolean {
  const router = useRouter();
  const token = useAuthStore((s) => s.token);
  const [ready, setReady] = useState(false);

  useEffect(() => {
    if (token || localStorage.getItem('token')) {
      setReady(true);
    } else {
      setReady(false);
      router.replace('/login');
    }
  }, [token, router]);

  return ready;
}

/**
 * Appelle `callback` toutes les `intervalMs` tant que la page est visible,
 * et immédiatement quand elle redevient visible (retour sur l'onglet / l'appli).
 */
export function usePolling(callback: () => void, intervalMs: number, enabled = true) {
  const saved = useRef(callback);
  useEffect(() => {
    saved.current = callback;
  }, [callback]);

  useEffect(() => {
    if (!enabled) return;
    const visible = () => document.visibilityState === 'visible';
    const id = window.setInterval(() => {
      if (visible()) saved.current();
    }, intervalMs);
    const onVisibility = () => {
      if (visible()) saved.current();
    };
    document.addEventListener('visibilitychange', onVisibility);
    return () => {
      window.clearInterval(id);
      document.removeEventListener('visibilitychange', onVisibility);
    };
  }, [intervalMs, enabled]);
}

/** Heure courante, rafraîchie régulièrement (pour « dans 2 h »). */
export function useNow(intervalMs = 30_000): number {
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    const id = window.setInterval(() => setNow(Date.now()), intervalMs);
    return () => window.clearInterval(id);
  }, [intervalMs]);
  return now;
}

/** État en direct d'un appareil (GET /devices/:id/status), rafraîchi toutes les 30 s. */
export function useDeviceStatus(deviceId: string, intervalMs = 30_000) {
  const [status, setStatus] = useState<DeviceStatus | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [updatedAt, setUpdatedAt] = useState<number | null>(null);
  const alive = useRef(true);

  useEffect(() => {
    alive.current = true;
    return () => {
      alive.current = false;
    };
  }, []);

  const refresh = useCallback(async () => {
    setLoading(true);
    try {
      const data = await devicesApi.status(deviceId);
      if (!alive.current) return;
      setStatus(data);
      setError(null);
      setUpdatedAt(Date.now());
    } catch (e) {
      if (!alive.current) return;
      setError(getErrorMessage(e, 'errors.deviceStatusUnavailable'));
    } finally {
      if (alive.current) setLoading(false);
    }
  }, [deviceId]);

  useEffect(() => {
    refresh();
  }, [refresh]);

  usePolling(refresh, intervalMs);

  return { status, error, loading, updatedAt, refresh };
}

/**
 * Appareil + poulailler qui le contient (le contrat n'a pas de GET /devices/:id).
 */
export function useDeviceContext(deviceId: string) {
  const [context, setContext] = useState<{ coop: CoopDetail; device: Device } | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  const reload = useCallback(async () => {
    try {
      const result = await findDeviceContext(deviceId);
      setContext(result);
      setError(null);
      return result;
    } catch (e) {
      setError(getErrorMessage(e, 'errors.deviceLoadFailed'));
      return null;
    } finally {
      setLoading(false);
    }
  }, [deviceId]);

  useEffect(() => {
    setLoading(true);
    reload();
  }, [reload]);

  return { coop: context?.coop ?? null, device: context?.device ?? null, error, loading, reload };
}

/**
 * État en direct d'un appareil au format des états mis en cache (lecture Omlet à l'ouverture,
 * puis toutes les `intervalMs`). Sert de source unique pour la porte principale.
 */
export function useLiveDeviceState(deviceId: string | undefined, intervalMs = 120_000) {
  const [live, setLive] = useState<DeviceStateView | null>(null);
  const refresh = useCallback(async () => {
    if (!deviceId) return;
    try {
      setLive(stateFromStatus(await devicesApi.status(deviceId)));
    } catch {
      // On garde l'état mis en cache par le serveur.
    }
  }, [deviceId]);

  useEffect(() => {
    refresh();
  }, [refresh]);
  usePolling(refresh, intervalMs, Boolean(deviceId));

  return { live, refresh };
}
