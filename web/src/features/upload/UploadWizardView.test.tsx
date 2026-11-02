import { screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { renderAt } from '@/test/render';
import { UploadWizardView } from './UploadWizardView';
import { buildManifest } from './buildManifest';
import { defaultValues } from './schema';
import { validateManifest } from '@/shared/lib/manifest';

describe('buildManifest', () => {
  it('produces a schema-valid manifest from wizard values', () => {
    const manifest = buildManifest(
      {
        ...defaultValues,
        packageSlug: 'deep-house-toolkit',
        primaryGenre: 'deep-house',
        moods: ['warm'],
        bpm: 124,
        bpmMin: 122,
        bpmMax: 126,
        keyTonic: 'F',
        freeTags: 'Tape, analog ,',
        files: [{ id: '1', path: 'loops/a.wav', size: 12, contentType: 'audio/wav', digest: `sha256:${'b'.repeat(64)}`, status: 'done', progress: 1 }],
      },
      { id: 'org.acme.deep-house-toolkit', name: 'Deep House Toolkit', publisher: { id: 'org.acme', slug: 'acme', name: 'Acme Audio', verified: true } },
    );
    expect(validateManifest(manifest).filter((i) => i.severity === 'error')).toEqual([]);
    expect(manifest).toMatchObject({ musical: { bpm: { value: 124, range: [122, 126] }, key: { tonic: 'F', mode: 'minor' } }, tags: { free: ['tape', 'analog'] } });
  });
});

const owner = { slug: 'acme', name: 'Acme Audio', role: 'owner' as const };
const ctx = { publisher: owner, all: [owner], select: () => undefined };

describe('buildManifest number coercion', () => {
  it('turns raw input strings into numbers and drops empty fields', () => {
    const raw = { ...defaultValues, packageSlug: 'kit', primaryGenre: 'techno', bpm: '124', bpmMin: '', bpmMax: '', energy: '0.7' } as unknown as Parameters<typeof buildManifest>[0];
    const m = buildManifest(raw, undefined) as { musical: { bpm: unknown }; taxonomy: { energy: unknown } };
    expect(m.musical.bpm).toEqual({ value: 124, sync: true });
    expect(m.taxonomy.energy).toBe(0.7);
  });
});

describe('UploadWizardView', () => {
  it('blocks the first step until required fields are valid, prefilled from ?package', async () => {
    const user = userEvent.setup();
    renderAt('/publisher/upload?package=deep-house-toolkit', '/publisher/upload', <UploadWizardView ctx={ctx} />);

    const version = await screen.findByLabelText('Version');
    // 1.3.0 draft exists → next minor is suggested.
    expect(version).toHaveValue('1.4.0');

    await user.click(screen.getByRole('button', { name: /Continue/ }));
    expect(await screen.findByText('Describe what changed')).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Step 1: Package' })).toBeInTheDocument();

    await user.clear(version);
    await user.type(version, '1.4');
    await user.type(screen.getByLabelText('Changelog'), 'New loops');
    await user.click(screen.getByRole('button', { name: /Continue/ }));
    expect(await screen.findByText('Use semver: MAJOR.MINOR.PATCH')).toBeInTheDocument();

    await user.type(version, '.0');
    await user.click(screen.getByRole('button', { name: /Continue/ }));
    expect(await screen.findByRole('heading', { name: 'Step 2: Taxonomy' })).toBeInTheDocument();
  });
});
