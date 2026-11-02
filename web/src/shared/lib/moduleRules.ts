import type { Module, ModuleClass, SignalKind, ValidationIssue } from '@/shared/types/domain';

const EVENT_INPUTS: SignalKind[] = ['midi', 'transport'];

function hasIn(m: Module, signals: SignalKind[]): boolean {
  return (
    m.capabilities.accepts.some((s) => signals.includes(s)) ||
    m.ports.some((p) => p.direction === 'in' && signals.includes(p.signal))
  );
}

function hasOut(m: Module, signals: SignalKind[]): boolean {
  return (
    m.capabilities.produces.some((s) => signals.includes(s)) ||
    m.ports.some((p) => p.direction === 'out' && signals.includes(p.signal))
  );
}

type Rule = (m: Module) => string[];

const rules: Partial<Record<ModuleClass, Rule>> = {
  instrument: (m) => [
    ...(hasIn(m, EVENT_INPUTS) ? [] : ['instrument must accept an event input (midi or transport)']),
    ...(hasOut(m, ['audio']) ? [] : ['instrument must produce audio']),
  ],
  'audio-effect': (m) => [
    ...(hasIn(m, ['audio']) ? [] : ['audio-effect must accept audio']),
    ...(hasOut(m, ['audio']) ? [] : ['audio-effect must produce audio']),
  ],
  'note-effect': (m) => [
    ...(hasIn(m, ['midi']) ? [] : ['note-effect must accept midi']),
    ...(hasOut(m, ['midi']) ? [] : ['note-effect must produce midi']),
  ],
  'note-generator': (m) => (hasOut(m, ['midi']) ? [] : ['note-generator must produce midi']),
};

/** Validation rules from spec §5.4. Returns an empty list when the module is valid. */
export function validateModule(m: Module): ValidationIssue[] {
  const rule = rules[m.moduleClass];
  const issues: ValidationIssue[] = (rule ? rule(m) : []).map((message) => ({
    severity: 'error',
    path: 'module.capabilities',
    message,
  }));
  if (m.categories.length === 0) {
    issues.push({ severity: 'warning', path: 'module.categories', message: 'add at least one category' });
  }
  return issues;
}
