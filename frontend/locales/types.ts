// Forme des dictionnaires de traduction (voir fr.ts, la référence).

/** Pluriel, choisi avec le paramètre `count` (règles Intl.PluralRules de la langue). */
export type Plural = { one: string; other: string };

/** Accord en genre (« fermée » / « fermé »), choisi avec le paramètre `gender`. */
export type Gendered = { f: string; m: string };

export type Leaf = string | Plural | Gendered;

export type Dictionary = { [key: string]: Leaf | Dictionary };

/**
 * Une traduction a exactement les clés du français (clé manquante ou en trop : erreur TypeScript).
 * Un accord { f, m } peut y devenir un simple texte (l'anglais n'accorde pas).
 */
export type Translation<T> = {
  [K in keyof T]: T[K] extends string
    ? string
    : T[K] extends Plural
      ? Plural
      : T[K] extends Gendered
        ? string | Gendered
        : Translation<T[K]>;
};

/** Clés à points ("hero.openNow") de toutes les feuilles d'un dictionnaire. */
export type Paths<T> = {
  [K in keyof T & string]: T[K] extends Leaf ? K : `${K}.${Paths<T[K]>}`;
}[keyof T & string];
