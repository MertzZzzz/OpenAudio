import type { ReactElement } from 'react';
import { render } from '@testing-library/react';
import { createMemoryRouter, RouterProvider, type RouteObject } from 'react-router-dom';
import type { AuthApi } from '@/features/auth/authApi';
import { QueryClient } from '@tanstack/react-query';
import { Providers } from '@/app/providers';
import { routerFuture } from '@/app/router';

/** Renders an element at a route with all app providers (query client, toasts, player). */
export function renderAt(path: string, pattern: string, element: ReactElement, opts: { authApi?: AuthApi; extraRoutes?: RouteObject[] } = {}) {
  const router = createMemoryRouter([{ path: pattern, element }, ...(opts.extraRoutes ?? []), { path: '*', element: <div>other route</div> }], { initialEntries: [path], future: routerFuture });
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const utils = render(
    <Providers client={client} authApi={opts.authApi}>
      <RouterProvider router={router} future={{ v7_startTransition: true }} />
    </Providers>,
  );
  return { ...utils, router };
}
