import { lazy } from 'react';
import { createBrowserRouter, type RouteObject } from 'react-router-dom';
import { RequireAuth } from '@/features/auth/RequireAuth';
import { AppLayout } from './layout/AppLayout';

const DiscoverPage = lazy(() => import('@/pages/DiscoverPage'));
const SearchPage = lazy(() => import('@/pages/SearchPage'));
const PackagePage = lazy(() => import('@/pages/PackagePage'));
const SamplesPage = lazy(() => import('@/pages/SamplesPage'));
const PublisherPage = lazy(() => import('@/pages/PublisherPage'));
const UploadPage = lazy(() => import('@/pages/UploadPage'));
const LibraryPage = lazy(() => import('@/pages/LibraryPage'));
const NotFoundPage = lazy(() => import('@/pages/NotFoundPage'));
const LoginPage = lazy(() => import('@/pages/auth/LoginPage'));
const RegisterPage = lazy(() => import('@/pages/auth/RegisterPage'));
const ForgotPasswordPage = lazy(() => import('@/pages/auth/ForgotPasswordPage'));
const ResetPasswordPage = lazy(() => import('@/pages/auth/ResetPasswordPage'));
const VerifyEmailPage = lazy(() => import('@/pages/auth/VerifyEmailPage'));
const AuthCallbackPage = lazy(() => import('@/pages/auth/AuthCallbackPage'));
const AccountPage = lazy(() => import('@/pages/auth/AccountPage'));

export const routes: RouteObject[] = [
  {
    element: <AppLayout />,
    children: [
      { index: true, element: <DiscoverPage /> },
      { path: 'search', element: <SearchPage /> },
      { path: 'packages/:publisher/:slug', element: <PackagePage /> },
      { path: 'samples', element: <SamplesPage /> },
      { path: 'publisher', element: <RequireAuth role="publisher"><PublisherPage /></RequireAuth> },
      { path: 'publisher/upload', element: <RequireAuth role="publisher"><UploadPage /></RequireAuth> },
      { path: 'library', element: <RequireAuth><LibraryPage /></RequireAuth> },
      { path: 'account', element: <RequireAuth><AccountPage /></RequireAuth> },
      { path: 'login', element: <LoginPage /> },
      { path: 'register', element: <RegisterPage /> },
      { path: 'forgot-password', element: <ForgotPasswordPage /> },
      { path: 'reset-password', element: <ResetPasswordPage /> },
      { path: 'verify-email', element: <VerifyEmailPage /> },
      { path: 'auth/callback', element: <AuthCallbackPage /> },
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
