import { screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { renderAt } from '@/test/render';
import { PackageDetailsView } from './PackageDetailsView';

describe('PackageDetailsView', () => {
  it('renders metadata, switches tabs and adds the package to the library', async () => {
    const user = userEvent.setup();
    renderAt('/p', '/p', <PackageDetailsView publisher="acme" slug="liquid-dnb-essentials" />);

    expect(await screen.findByRole('heading', { level: 1, name: 'Liquid DnB Essentials' })).toBeInTheDocument();
    expect(screen.getByText('Drum & Bass')).toBeInTheDocument();

    await user.click(screen.getByRole('tab', { name: /Versions/ }));
    expect(await screen.findByRole('cell', { name: /1\.4\.2/ })).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Add to library' }));
    expect(await screen.findByRole('button', { name: 'In library' })).toBeDisabled();
    expect(await screen.findByText('Added Liquid DnB Essentials to your library')).toBeInTheDocument();
  });

  it('shows a not-found state for unknown packages', async () => {
    renderAt('/p', '/p', <PackageDetailsView publisher="acme" slug="nope" />);
    expect(await screen.findByText('Package not found')).toBeInTheDocument();
  });

  it('renders the module diagram for module packages', async () => {
    renderAt('/p', '/p', <PackageDetailsView publisher="lattice" slug="plate-verb" />);
    expect(await screen.findByText('Module definition')).toBeInTheDocument();
    expect(screen.getByText('Audio → Audio')).toBeInTheDocument();
  });
});
