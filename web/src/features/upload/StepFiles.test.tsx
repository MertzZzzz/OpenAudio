import { fireEvent, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { FormProvider, useForm } from 'react-hook-form';
import { renderAt } from '@/test/render';
import { StepFiles } from './StepFiles';
import { defaultValues, type WizardValues } from './schema';

let latest: WizardValues | undefined;

function Harness({ kind = 'effect' }: { kind?: WizardValues['kind'] }) {
  const form = useForm<WizardValues>({ defaultValues: { ...defaultValues, kind } });
  latest = form.watch();
  return (
    <FormProvider {...form}>
      <StepFiles />
    </FormProvider>
  );
}

/** A File as a folder picker returns it: with webkitRelativePath. */
function inFolder(path: string, bytes: Uint8Array | string = 'x'): File {
  const f = new File([bytes], path.split('/').pop()!, { type: '' });
  Object.defineProperty(f, 'webkitRelativePath', { value: path });
  return f;
}

/** Minimal Mach-O universal header (arm64 + x86_64). */
function fatMachO(): Uint8Array {
  const v = new DataView(new ArrayBuffer(64));
  v.setUint32(0, 0xcafebabe, false);
  v.setUint32(4, 2, false);
  v.setUint32(8, 0x0100000c, false);
  v.setUint32(28, 0x01000007, false);
  return new Uint8Array(v.buffer);
}

describe('StepFiles with plug-in bundles', () => {
  it('uploads a folder, detects the VST3 bundle and its platforms, skips OS clutter', async () => {
    renderAt('/u', '/u', <Harness />);
    fireEvent.change(screen.getByTestId('folder-input'), {
      target: {
        files: [
          inFolder('Tape.vst3/Contents/Info.plist', '<plist/>'),
          inFolder('Tape.vst3/Contents/MacOS/Tape', fatMachO()),
          inFolder('Tape.vst3/Contents/x86_64-win/Tape.vst3'),
          inFolder('Tape.vst3/.DS_Store'),
        ],
      },
    });

    const builds = await screen.findByRole('list', { name: 'Plug-in builds' });
    await waitFor(() => expect(within(builds).getAllByRole('listitem')).toHaveLength(2));
    await waitFor(() => expect(latest?.files.every((f) => f.status === 'done')).toBe(true));
    expect(latest?.files.map((f) => f.path).sort()).toEqual(['Tape.vst3/Contents/Info.plist', 'Tape.vst3/Contents/MacOS/Tape', 'Tape.vst3/Contents/x86_64-win/Tape.vst3']);
    await waitFor(() =>
      expect(latest?.plugins).toEqual([
        { format: 'vst3', bundle: 'Tape.vst3', os: 'macos', arch: ['x86_64', 'arm64'], auto: true },
        { format: 'vst3', bundle: 'Tape.vst3', os: 'windows', arch: ['x86_64'], auto: true },
      ]),
    );
  });

  it('lets the publisher correct a build and warns about the package kind', async () => {
    const user = userEvent.setup();
    renderAt('/u', '/u', <Harness kind="sample-pack" />);
    fireEvent.change(screen.getByTestId('file-input'), { target: { files: [new File(['?'], 'tape.clap')] } });
    await screen.findByRole('list', { name: 'Plug-in builds' });
    expect(screen.getByText(/VST3\/CLAP builds are published with instrument, effect/)).toBeInTheDocument();

    await user.selectOptions(screen.getByLabelText('Operating system for tape.clap'), 'windows');
    await user.click(within(screen.getByRole('group', { name: /Architectures for tape.clap/ })).getByLabelText('ARM64'));
    await user.type(screen.getByLabelText(/Plug-in ids for tape.clap/), 'com.acme.tape');
    await waitFor(() => expect(latest?.plugins).toEqual([{ format: 'clap', bundle: 'tape.clap', os: 'windows', arch: ['x86_64', 'arm64'], pluginIds: ['com.acme.tape'], auto: false }]));

    await user.click(screen.getByRole('button', { name: 'Remove build tape.clap (Windows)' }));
    expect(latest?.plugins).toEqual([]);
  });
});
