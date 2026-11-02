/** Minimal semver helpers for the mock resolver (exact, ^, ~, >= ranges). */

export type SemVer = [number, number, number];

export function parse(version: string): SemVer | null {
  const m = /^(\d+)\.(\d+)\.(\d+)$/.exec(version.trim());
  if (!m) return null;
  return [Number(m[1]), Number(m[2]), Number(m[3])];
}

export function compare(a: string, b: string): number {
  const pa = parse(a);
  const pb = parse(b);
  if (!pa || !pb) return a.localeCompare(b);
  for (let i = 0; i < 3; i += 1) {
    const d = pa[i]! - pb[i]!;
    if (d !== 0) return d;
  }
  return 0;
}

export function satisfies(version: string, range: string): boolean {
  const v = parse(version);
  if (!v) return false;
  const r = range.trim();
  if (r === '*' || r === '') return true;
  if (r.startsWith('>=')) return compare(version, r.slice(2)) >= 0;
  if (r.startsWith('^') || r.startsWith('~')) {
    const base = parse(r.slice(1));
    if (!base) return false;
    if (compare(version, r.slice(1)) < 0) return false;
    if (r.startsWith('^')) return base[0] === 0 ? v[0] === 0 && v[1] === base[1] : v[0] === base[0];
    return v[0] === base[0] && v[1] === base[1];
  }
  return compare(version, r) === 0;
}

export function maxSatisfying(versions: string[], range: string): string | null {
  const ok = versions.filter((v) => satisfies(v, range)).sort(compare);
  return ok.length ? ok[ok.length - 1]! : null;
}
