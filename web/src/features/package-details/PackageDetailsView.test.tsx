import { screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { renderAt } from '@/test/render';
import { PackageDetailsView } from './PackageDetailsView';
import { createMockAuthApi } from '@/features/auth/mockAuthApi';

describe('PackageDetailsView', () => {
  it('renders metadata, switches tabs and adds the package to the library', async () => {
    const user = userEvent.setup();
    const authApi = createMockAuthApi({ latencyMs: 0 });
    await authApi.login('demo@openaudio.dev', 'openaudio-demo');
    renderAt('/p', '/p', <PackageDetailsView publisher="acme" slug="liquid-dnb-essentials" />, { authApi });

    expect(await screen.findByRole('heading', { level: 1, name: 'Liquid DnB Essentials' })).toBeInTheDocument();
    expect(screen.getByText('Drum & Bass')).toBeInTheDocument();

    await user.click(screen.getByRole('tab', { name: /Versions/ }));
    expect(await screen.findByRole('cell', { name: /1\.4\.2/ })).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Add to library' }));
    expect(await screen.findByRole('button', { name: 'In library' })).toBeDisabled();
    expect(await screen.findByText('Added Liquid DnB Essentials to your library')).toBeInTheDocument();
  });

  it('sends anonymous visitors to sign in before adding to the library', async () => {
    const user = userEvent.setup();
    const { router } = renderAt('/packages/acme/plate-verb', '/packages/:publisher/:slug', <PackageDetailsView publisher="lattice" slug="plate-verb" />, {
      authApi: createMockAuthApi({ latencyMs: 0 }),
      extraRoutes: [{ path: '/login', element: <p>login page</p> }],
    });
    await user.click(await screen.findByRole('button', { name: 'Add to library' }));
    expect(await screen.findByText('login page')).toBeInTheDocument();
    expect(router.state.location.search).toBe('?returnTo=%2Fpackages%2Facme%2Fplate-verb');
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
