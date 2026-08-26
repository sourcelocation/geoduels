export function formatDamageMultiplierLabel(damageMultiplier: number) {
  const roundedMultiplier = Number(damageMultiplier.toFixed(1));
  if (roundedMultiplier === 1) return null;
  return `${roundedMultiplier}x`;
}
