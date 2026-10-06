// Langue de l'interface : constantes et détection, sans React (importable côté serveur, ex. layout).

export type Locale = 'fr' | 'en';

export const LOCALES: readonly Locale[] = ['fr', 'en'];

/** Noms des langues, chacun dans sa propre langue (jamais traduits). */
export const LOCALE_NAMES: Record<Locale, string> = { fr: 'Français', en: 'English' };

/** Formats de dates et d'heures : horloge sur 24 h dans les deux langues. */
export const INTL_LOCALES: Record<Locale, string> = { fr: 'fr-FR', en: 'en-GB' };

/** Choix explicite de l'utilisateur, gardé dans localStorage. */
export const LOCALE_STORAGE_KEY = 'locale';

export const isLocale = (value: unknown): value is Locale => value === 'fr' || value === 'en';

/** Choix enregistré, sinon langue du navigateur : français si elle commence par « fr », anglais sinon. */
export function detectLocale(): Locale {
  try {
    const saved = window.localStorage.getItem(LOCALE_STORAGE_KEY);
    if (isLocale(saved)) return saved;
  } catch {
    // Stockage inaccessible (navigation privée stricte…) : on suit le navigateur.
  }
  return (navigator.language || '').toLowerCase().startsWith('fr') ? 'fr' : 'en';
}

/**
 * La même règle que detectLocale(), en script inline (layout) : <html lang> est juste dès le premier
 * affichage, avant React (lecteurs d'écran, proposition de traduction du navigateur).
 */
export const LOCALE_SCRIPT = `(function(){var l;try{l=localStorage.getItem('${LOCALE_STORAGE_KEY}')}catch(e){}if(l!=='fr'&&l!=='en'){l=(navigator.language||'').toLowerCase().indexOf('fr')===0?'fr':'en'}document.documentElement.lang=l})();`;
