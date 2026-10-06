'use client';

// Traductions de l'interface : dictionnaires (locales/), t() et fournisseur React.
// La langue vit dans un petit magasin de module : les aides hors React (lib/time, lib/coop…) la lisent
// via t() / getLocale(), et le fournisseur s'y abonne (useSyncExternalStore) pour re-rendre l'interface.
//
// Hydratation : le serveur ne connaît ni localStorage ni la langue du navigateur. Le serveur et le premier
// rendu client affichent donc `fallback` (une roue sans texte), puis l'interface apparaît directement dans
// la bonne langue : jamais de texte français affiché puis remplacé par de l'anglais (ou l'inverse).

import { Fragment, createContext, useContext, useEffect, useMemo, useSyncExternalStore, type ReactNode } from 'react';
import { en } from '@/locales/en';
import { fr } from '@/locales/fr';
import type { Dictionary, Leaf, Paths } from '@/locales/types';
import { INTL_LOCALES, LOCALE_STORAGE_KEY, detectLocale, isLocale, type Locale } from '@/lib/locale';

export type { Locale } from '@/lib/locale';

/** Toutes les clés du dictionnaire français ("hero.openNow"…). */
export type MessageKey = Paths<typeof fr>;

/** {nom} est remplacé par params.nom ; `count` choisit le pluriel, `gender` ('f' | 'm') l'accord. */
export type Params = Record<string, string | number>;

export type TFunction = (key: MessageKey, params?: Params) => string;

/** Comme t(), mais les paramètres peuvent être des éléments React (lien, gras…). */
export type RichFunction = (key: MessageKey, params: Record<string, ReactNode>) => ReactNode;

// ---------------------------------------------------------------------------
// Dictionnaires
// ---------------------------------------------------------------------------

type Flat = Map<string, Leaf>;

function isLeaf(value: Leaf | Dictionary): value is Leaf {
  if (typeof value === 'string') return true;
  const keys = Object.keys(value).sort().join(',');
  return keys === 'one,other' || keys === 'f,m';
}

function flatten(dict: Dictionary, prefix = '', out: Flat = new Map()): Flat {
  for (const [key, value] of Object.entries(dict)) {
    const path = prefix ? `${prefix}.${key}` : key;
    if (isLeaf(value)) out.set(path, value);
    else flatten(value, path, out);
  }
  return out;
}

const DICTIONARIES: Record<Locale, Flat> = { fr: flatten(fr), en: flatten(en) };

const pluralRules = new Map<Locale, Intl.PluralRules>();

function pluralOf(locale: Locale, count: number): 'one' | 'other' {
  let rules = pluralRules.get(locale);
  if (!rules) {
    rules = new Intl.PluralRules(INTL_LOCALES[locale]);
    pluralRules.set(locale, rules);
  }
  return rules.select(count) === 'one' ? 'one' : 'other';
}

/** Texte brut d'une clé (le français sert de secours), pluriel et accord déjà choisis. */
function template(locale: Locale, key: MessageKey, params?: Record<string, unknown>): string {
  const leaf = DICTIONARIES[locale].get(key) ?? DICTIONARIES.fr.get(key);
  if (leaf === undefined) return key;
  if (typeof leaf === 'string') return leaf;
  if ('one' in leaf) return leaf[pluralOf(locale, Number(params?.count ?? 0))];
  return params?.gender === 'm' ? leaf.m : leaf.f;
}

const has = (params: object, name: string) => Object.prototype.hasOwnProperty.call(params, name);

export function translate(locale: Locale, key: MessageKey, params?: Params): string {
  const text = template(locale, key, params);
  if (!params) return text;
  return text.replace(/\{(\w+)\}/g, (match, name: string) => (has(params, name) ? String(params[name]) : match));
}

function translateRich(locale: Locale, key: MessageKey, params: Record<string, ReactNode>): ReactNode {
  return template(locale, key, params)
    .split(/(\{\w+\})/)
    .filter(Boolean)
    .map((part, i) => {
      const name = /^\{(\w+)\}$/.exec(part)?.[1];
      return name && has(params, name) ? <Fragment key={i}>{params[name]}</Fragment> : part;
    });
}

/** Vrai si la clé existe (libellés de valeurs venues du serveur : états, défauts…). */
export const hasMessage = (key: string): key is MessageKey => DICTIONARIES.fr.has(key);

// ---------------------------------------------------------------------------
// Langue courante (hors React)
// ---------------------------------------------------------------------------

// null côté serveur : la langue n'est connue que dans le navigateur.
let current: Locale | null = typeof window === 'undefined' ? null : detectLocale();
const listeners = new Set<() => void>();

/** Langue courante (anglais tant qu'elle n'est pas connue, c'est-à-dire côté serveur). */
export const getLocale = (): Locale => current ?? 'en';

/** Locale Intl des dates et heures : 'fr-FR' ou 'en-GB' (24 h). */
export const intlLocale = (): string => INTL_LOCALES[getLocale()];

/** Change la langue des textes produits hors React (lib/*) et prévient le fournisseur. */
export function setCurrentLocale(locale: Locale): void {
  if (locale === current) return;
  current = locale;
  listeners.forEach((listener) => listener());
}

/** Traduction dans la langue courante, pour le code hors React (lib/coop, lib/time…). */
export const t: TFunction = (key, params) => translate(getLocale(), key, params);

// ---------------------------------------------------------------------------
// React
// ---------------------------------------------------------------------------

export interface I18n {
  locale: Locale;
  /** Change la langue tout de suite et la garde (localStorage). */
  setLocale: (locale: Locale) => void;
  t: TFunction;
  tRich: RichFunction;
}

function changeLocale(locale: Locale) {
  try {
    window.localStorage.setItem(LOCALE_STORAGE_KEY, locale);
  } catch {
    // Stockage inaccessible : le choix vaut pour cette visite.
  }
  setCurrentLocale(locale);
}

const valueFor = (locale: Locale): I18n => ({
  locale,
  setLocale: changeLocale,
  t: (key, params) => translate(locale, key, params),
  tRich: (key, params) => translateRich(locale, key, params),
});

const I18nContext = createContext<I18n | null>(null);

const subscribe = (listener: () => void) => {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
};
const getSnapshot = () => current;
const getServerSnapshot = (): Locale | null => null;

export function I18nProvider({ children, fallback = null }: { children: ReactNode; fallback?: ReactNode }) {
  const locale = useSyncExternalStore(subscribe, getSnapshot, getServerSnapshot);
  const value = useMemo(() => (locale ? valueFor(locale) : null), [locale]);

  useEffect(() => {
    if (locale) document.documentElement.lang = locale;
  }, [locale]);

  // Langue changée dans un autre onglet.
  useEffect(() => {
    const onStorage = (event: StorageEvent) => {
      if (event.key === LOCALE_STORAGE_KEY && isLocale(event.newValue)) setCurrentLocale(event.newValue);
    };
    window.addEventListener('storage', onStorage);
    return () => window.removeEventListener('storage', onStorage);
  }, []);

  if (!value) return <>{fallback}</>;
  return <I18nContext.Provider value={value}>{children}</I18nContext.Provider>;
}

export function useI18n(): I18n {
  return useContext(I18nContext) ?? valueFor(getLocale());
}
