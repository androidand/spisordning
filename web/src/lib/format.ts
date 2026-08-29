export function formatPrice(value: number | null | undefined): string {
  if (value === null || value === undefined) return "—";
  return `${value.toFixed(2).replace(".", ",")} kr`;
}

export function formatDate(iso: string | null | undefined): string {
  if (!iso) return "—";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  return d.toLocaleDateString("en-GB", { day: "2-digit", month: "short", year: "numeric" });
}

export function weekdayOf(dateISO: string): string {
  const d = new Date(dateISO + "T00:00:00Z");
  if (Number.isNaN(d.getTime())) return dateISO;
  return d.toLocaleDateString("en-GB", { weekday: "long", timeZone: "UTC" });
}

export function quantityLabel(quantity: number, unit: string): string {
  const q = Number.isInteger(quantity) ? quantity.toString() : quantity.toFixed(2);
  return unit ? `${q} ${unit}` : q;
}

/** Whole days from today until the given date (negative = already past). */
export function daysUntil(iso: string | null | undefined): number | null {
  if (!iso) return null;
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return null;
  const today = new Date();
  today.setHours(0, 0, 0, 0);
  const target = new Date(d);
  target.setHours(0, 0, 0, 0);
  return Math.round((target.getTime() - today.getTime()) / 86_400_000);
}

/** Human label for a best-before date: "expired 2 d ago", "today", "in 3 d". */
export function expiryLabel(iso: string | null | undefined): string {
  const days = daysUntil(iso);
  if (days === null) return "no expiry";
  if (days < 0) return `expired ${Math.abs(days)} d ago`;
  if (days === 0) return "expires today";
  return `expires in ${days} d`;
}
