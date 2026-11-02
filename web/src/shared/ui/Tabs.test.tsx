import { useState } from 'react';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { Tabs } from './Tabs';

function Harness() {
  const [v, setV] = useState<'a' | 'b' | 'c'>('a');
  return (
    <>
      <Tabs label="Sections" value={v} onChange={setV} items={[{ value: 'a', label: 'Alpha' }, { value: 'b', label: 'Beta' }, { value: 'c', label: 'Gamma' }]} />
      <p>current: {v}</p>
    </>
  );
}

describe('Tabs', () => {
  it('exposes ARIA roles and supports arrow-key navigation with wrap-around', async () => {
    const user = userEvent.setup();
    render(<Harness />);
    expect(screen.getByRole('tablist', { name: 'Sections' })).toBeInTheDocument();
    const alpha = screen.getByRole('tab', { name: 'Alpha' });
    expect(alpha).toHaveAttribute('aria-selected', 'true');
    expect(screen.getByRole('tab', { name: 'Beta' })).toHaveAttribute('tabindex', '-1');

    alpha.focus();
    await user.keyboard('{ArrowRight}');
    expect(screen.getByText('current: b')).toBeInTheDocument();
    expect(screen.getByRole('tab', { name: 'Beta' })).toHaveFocus();

    await user.keyboard('{ArrowLeft}{ArrowLeft}');
    expect(screen.getByText('current: c')).toBeInTheDocument();

    await user.keyboard('{Home}');
    expect(screen.getByText('current: a')).toBeInTheDocument();
  });
});
