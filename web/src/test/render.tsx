import type { ReactElement } from 'react';
import { render } from '@testing-library/react';
import { createMemoryRouter, RouterProvider } from 'react-router-dom';
import { QueryClient } from '@tanstack/react-query';
import { Providers } from '@/app/providers';
import { routerFuture } from '@/app/router';

/** Renders an element at a route with all app providers (query client, toasts, player). */
export function renderAt(path: string, pattern: string, element: ReactElement) {
  const router = createMemoryRouter([{ path: pattern, element }, { path: '*', element: <div>other route</div> }], { initialEntries: [path], future: routerFuture });
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const utils = render(
    <Providers client={client}>
      <RouterProvider router={router} future={{ v7_startTransition: true }} />
    </Providers>,
  );
  return { ...utils, router };
}
