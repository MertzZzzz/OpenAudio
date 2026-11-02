import { compare, maxSatisfying, satisfies } from './semver';

describe('semver', () => {
  it('compares numerically, not lexically', () => {
    expect(compare('1.10.0', '1.9.0')).toBeGreaterThan(0);
    expect(compare('2.0.0', '2.0.0')).toBe(0);
  });

  it.each([
    ['1.4.2', '^1.0.0', true],
    ['2.0.0', '^1.0.0', false],
    ['0.9.3', '^0.9.0', true],
    ['0.10.0', '^0.9.0', false],
    ['1.2.9', '~1.2.0', true],
    ['1.3.0', '~1.2.0', false],
    ['1.0.0', '1.0.0', true],
    ['3.0.0', '>=2.1.0', true],
    ['1.0.0', '*', true],
  ])('%s satisfies %s → %s', (v, r, ok) => {
    expect(satisfies(v, r)).toBe(ok);
  });

  it('picks the highest satisfying version', () => {
    expect(maxSatisfying(['1.0.0', '1.2.0', '2.0.1', '1.10.0'], '^1.0.0')).toBe('1.10.0');
    expect(maxSatisfying(['1.0.0'], '^2.0.0')).toBeNull();
  });
});
