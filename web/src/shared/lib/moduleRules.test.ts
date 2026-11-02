import type { Module } from '@/shared/types/domain';
import { validateModule } from './moduleRules';

const mod = (m: Partial<Module> & Pick<Module, 'moduleClass'>): Module => ({
  capabilities: { accepts: [], produces: [], placement: [] },
  ports: [],
  categories: ['x'],
  ...m,
});

describe('module validation rules (§5.4)', () => {
  it('instrument needs an event input and audio output', () => {
    expect(validateModule(mod({ moduleClass: 'instrument', capabilities: { accepts: ['midi'], produces: ['audio'], placement: [] } }))).toEqual([]);
    const issues = validateModule(mod({ moduleClass: 'instrument', capabilities: { accepts: ['audio'], produces: ['midi'], placement: [] } }));
    expect(issues.map((i) => i.message)).toEqual(['instrument must accept an event input (midi or transport)', 'instrument must produce audio']);
  });

  it('audio-effect needs audio in and out', () => {
    expect(validateModule(mod({ moduleClass: 'audio-effect', capabilities: { accepts: ['audio'], produces: ['audio'], placement: [] } }))).toEqual([]);
    expect(validateModule(mod({ moduleClass: 'audio-effect', capabilities: { accepts: ['midi'], produces: ['audio'], placement: [] } }))).toHaveLength(1);
  });

  it('note-generator may lack midi input but must output midi', () => {
    expect(validateModule(mod({ moduleClass: 'note-generator', capabilities: { accepts: ['transport'], produces: ['midi'], placement: [] } }))).toEqual([]);
    expect(validateModule(mod({ moduleClass: 'note-generator', capabilities: { accepts: [], produces: ['audio'], placement: [] } }))[0]?.severity).toBe('error');
  });

  it('analyzer does not need an audio output', () => {
    expect(validateModule(mod({ moduleClass: 'analyzer', capabilities: { accepts: ['audio'], produces: ['analysis'], placement: [] } }))).toEqual([]);
  });

  it('accepts ports as an alternative to capabilities', () => {
    const m = mod({ moduleClass: 'note-effect', ports: [{ id: 'i', direction: 'in', signal: 'midi' }, { id: 'o', direction: 'out', signal: 'midi' }] });
    expect(validateModule(m)).toEqual([]);
  });

  it('warns when categories are empty', () => {
    expect(validateModule(mod({ moduleClass: 'utility', categories: [] }))[0]).toMatchObject({ severity: 'warning', path: 'module.categories' });
  });
});
