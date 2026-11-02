import { lazy } from 'react';
import { createBrowserRouter, type RouteObject } from 'react-router-dom';
import { AppLayout } from './layout/AppLayout';

const DiscoverPage = lazy(() => import('@/pages/DiscoverPage'));
const SearchPage = lazy(() => import('@/pages/SearchPage'));
const PackagePage = lazy(() => import('@/pages/PackagePage'));
const SamplesPage = lazy(() => import('@/pages/SamplesPage'));
const PublisherPage = lazy(() => import('@/pages/PublisherPage'));
const UploadPage = lazy(() => import('@/pages/UploadPage'));
const LibraryPage = lazy(() => import('@/pages/LibraryPage'));
const NotFoundPage = lazy(() => import('@/pages/NotFoundPage'));

export const routes: RouteObject[] = [
  {
    element: <AppLayout />,
    children: [
      { index: true, element: <DiscoverPage /> },
      { path: 'search', element: <SearchPage /> },
      { path: 'packages/:publisher/:slug', element: <PackagePage /> },
      { path: 'samples', element: <SamplesPage /> },
      { path: 'publisher', element: <PublisherPage /> },
      { path: 'publisher/upload', element: <UploadPage /> },
      { path: 'library', element: <LibraryPage /> },
      { path: '*', element: <NotFoundPage /> },
    ],
  },
];

export const routerFuture = {
  v7_relativeSplatPath: true,
  v7_fetcherPersist: true,
  v7_normalizeFormMethod: true,
  v7_partialHydration: true,
  v7_skipActionErrorRevalidation: true,
} as const;

export const router = createBrowserRouter(routes, { future: routerFuture });
