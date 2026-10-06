import React from 'react';

export interface ChipOption<T extends string> {
  value: T;
  label: string;
  disabled?: boolean;
  /** Langue du libellé s'il diffère de la page (ex. « English » dans l'interface française). */
  lang?: string;
}

/** Choix exclusif en gros boutons (plus rapide au doigt qu'une liste déroulante). */
export function Chips<T extends string>({
  label,
  value,
  options,
  onChange,
}: {
  label: string;
  value: T | null;
  options: ChipOption<T>[];
  onChange: (value: T) => void;
}) {
  return (
    <div role="radiogroup" aria-label={label} className="flex flex-wrap gap-2">
      {options.map((o) => {
        const active = o.value === value;
        return (
          <button
            key={o.value}
            type="button"
            role="radio"
            aria-checked={active}
            disabled={o.disabled}
            lang={o.lang}
            onClick={() => onChange(o.value)}
            className={`min-h-[44px] rounded-xl px-3.5 text-[15px] font-bold transition-colors disabled:opacity-40 ${
              active ? 'bg-ink text-canvas' : 'bg-canvas text-ink ring-1 ring-inset ring-line hover:bg-line/40'
            }`}
          >
            {o.label}
          </button>
        );
      })}
    </div>
  );
}
