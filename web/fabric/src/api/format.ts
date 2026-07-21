export function enumLabel(value: number, labels: Readonly<Record<number, string>>): string {
  return labels[value] ?? `Unknown (${value})`;
}
