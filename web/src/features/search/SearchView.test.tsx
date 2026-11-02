import { screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { renderAt } from '@/test/render';
import { SearchView } from './SearchView';
import { parseSearchParams, toSearchString } from './params';

describe('search params', () => {
  it('round-trips through the URL', () => {
    const p = parseSearchParams(new URLSearchParams('q=tape&kind=effect,instrument&bpmMin=120&sort=newest&page=2&kind=bogus'));
    expect(p).toMatchObject({ q: 'tape', kind: ['effect', 'instrument'], bpmMin: 120, sort: 'newest', page: 2 });
    expect(toSearchString(p)).toBe('q=tape&kind=effect%2Cinstrument&bpmMin=120&sort=newest&page=2');
  });
});

describe('SearchView', () => {
  it('filters results with a facet checkbox and syncs the URL', async () => {
    const user = userEvent.setup();
    const { router } = renderAt('/search', '/search', <SearchView />);
    expect(await screen.findByText('16 packages')).toBeInTheDocument();

    const typeGroup = await screen.findByRole('group', { name: 'Type' });
    await user.click(within(typeGroup).getByRole('checkbox', { name: /Effect/ }));

    expect(await screen.findByText('3 packages')).toBeInTheDocument();
    expect(router.state.location.search).toBe('?kind=effect');
    expect(screen.getByRole('button', { name: 'Remove filter Effect' })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Plate Verb' })).toBeInTheDocument();
  });

  it('shows an empty state when nothing matches', async () => {
    renderAt('/search?q=zzzznothing', '/search', <SearchView />);
    expect(await screen.findByText('No packages match')).toBeInTheDocument();
  });
});
