import React, { useId } from 'react';

interface SwitchProps {
  checked: boolean;
  onChange: (checked: boolean) => void;
  /** Libellé visible (ou seulement lu par les lecteurs d'écran si labelHidden). */
  label: string;
  labelHidden?: boolean;
  description?: React.ReactNode;
  disabled?: boolean;
  busy?: boolean;
  id?: string;
}

/** Interrupteur accessible (role="switch"), zone tactile d'au moins 44 px. */
export const Switch: React.FC<SwitchProps> = ({
  checked,
  onChange,
  label,
  labelHidden = false,
  description,
  disabled = false,
  busy = false,
  id,
}) => {
  const autoId = useId();
  const switchId = id ?? autoId;
  const control = (
    <button
      type="button"
      id={switchId}
      role="switch"
      aria-checked={checked}
      aria-label={labelHidden ? label : undefined}
      aria-busy={busy || undefined}
      disabled={disabled || busy}
      onClick={() => onChange(!checked)}
      className="group inline-flex min-h-[44px] min-w-[52px] shrink-0 items-center justify-center rounded-full disabled:cursor-not-allowed disabled:opacity-60"
    >
      <span
        className={`relative inline-block h-7 w-12 rounded-full transition-colors ${
          checked ? 'bg-ok' : 'bg-line'
        } ${busy ? 'animate-pulse' : ''}`}
      >
        <span
          className={`absolute left-0.5 top-0.5 h-6 w-6 rounded-full bg-white shadow-sm transition-transform ${
            checked ? 'translate-x-5' : 'translate-x-0'
          }`}
        />
      </span>
    </button>
  );

  if (labelHidden) return control;

  return (
    <div className="flex items-center justify-between gap-4">
      <div className="min-w-0">
        <label htmlFor={switchId} className="block font-bold">
          {label}
        </label>
        {description && <p className="text-sm text-muted">{description}</p>}
      </div>
      {control}
    </div>
  );
};
