'use client';

import React, { useEffect, useState } from 'react';
import { inputBaseClass } from '@/components/ui/Input';

interface NumberFieldProps
  extends Omit<React.InputHTMLAttributes<HTMLInputElement>, 'value' | 'onChange' | 'type' | 'min' | 'max'> {
  value: number;
  onValueChange: (value: number) => void;
  min: number;
  max: number;
}

/**
 * Champ entier borné. Le texte saisi est gardé tel quel pendant la frappe
 * (champ vide autorisé), la valeur transmise est toujours un entier entre min et max.
 */
export const NumberField: React.FC<NumberFieldProps> = ({
  value,
  onValueChange,
  min,
  max,
  className = 'w-full',
  onBlur,
  ...props
}) => {
  const [text, setText] = useState(String(value));

  useEffect(() => {
    setText((current) => (current !== '' && Number(current) === value ? current : String(value)));
  }, [value]);

  const handleChange = (event: React.ChangeEvent<HTMLInputElement>) => {
    const raw = event.target.value;
    setText(raw);
    if (raw.trim() === '') return;
    const parsed = Number(raw);
    if (!Number.isFinite(parsed)) return;
    const clamped = Math.min(max, Math.max(min, Math.round(parsed)));
    if (clamped !== parsed) setText(String(clamped));
    if (clamped !== value) onValueChange(clamped);
  };

  const handleBlur = (event: React.FocusEvent<HTMLInputElement>) => {
    if (text.trim() === '' || !Number.isFinite(Number(text))) setText(String(value));
    onBlur?.(event);
  };

  return (
    <input
      type="number"
      inputMode="numeric"
      step={1}
      min={min}
      max={max}
      value={text}
      onChange={handleChange}
      onBlur={handleBlur}
      className={`${inputBaseClass} tnum ${className}`}
      {...props}
    />
  );
};
