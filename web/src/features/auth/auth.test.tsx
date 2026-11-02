import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { renderAt } from '@/test/render';
import { createMockAuthApi } from './mockAuthApi';
import { RequireAuth } from './RequireAuth';
import { useAuth } from './AuthContext';
import { LoginView } from './views/LoginView';
import { RegisterView } from './views/RegisterView';
import { AuthCallbackView } from './views/CallbackViews';
import { ResetPasswordView, ForgotPasswordView } from './views/PasswordViews';
import { safeReturnTo } from './forms';

const mock = () => createMockAuthApi({ latencyMs: 0 });

function Whoami() {
  const { user } = useAuth();
  return <p>signed in as {user?.email ?? 'nobody'}</p>;
}
const whoami = [{ path: '/library', element: <Whoami /> }, { path: '/account', element: <Whoami /> }, { path: '/', element: <Whoami /> }];

describe('LoginView', () => {
  it('validates input, reports wrong credentials and signs in to the return path', async () => {
    const user = userEvent.setup();
    renderAt('/login?returnTo=/library', '/login', <LoginView />, { authApi: mock(), extraRoutes: whoami });

    await user.click(await screen.findByRole('button', { name: 'Sign in' }));
    expect(await screen.findByText('Email is required')).toBeInTheDocument();
    expect(screen.getByText('Password is required')).toBeInTheDocument();

    await user.type(screen.getByLabelText('Email'), 'demo@openaudio.dev');
    await user.type(screen.getByLabelText('Password'), 'wrong-password');
    await user.click(screen.getByRole('button', { name: 'Sign in' }));
    expect(await screen.findByRole('alert')).toHaveTextContent('Email or password is incorrect.');

    await user.clear(screen.getByLabelText('Password'));
    await user.type(screen.getByLabelText('Password'), 'openaudio-demo');
    await user.click(screen.getByRole('button', { name: 'Sign in' }));
    expect(await screen.findByText('signed in as demo@openaudio.dev')).toBeInTheDocument();
  });

  it('offers Google and Discord and signs in with them', async () => {
    const user = userEvent.setup();
    renderAt('/login?returnTo=/library', '/login', <LoginView />, { authApi: mock(), extraRoutes: whoami });
    expect(await screen.findByRole('button', { name: /Continue with Google/ })).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: /Continue with Discord/ }));
    expect(await screen.findByText('signed in as discord.user@example.com')).toBeInTheDocument();
  });
});

describe('RegisterView', () => {
  it('checks the password confirmation, then creates the account', async () => {
    const user = userEvent.setup();
    const api = mock();
    renderAt('/register', '/register', <RegisterView />, { authApi: api, extraRoutes: whoami });

    await user.type(await screen.findByLabelText('Display name'), 'Beat Maker');
    await user.type(screen.getByLabelText('Email'), 'Beat@Example.com');
    await user.type(screen.getByLabelText('Password'), 'long-password-1');
    await user.type(screen.getByLabelText('Confirm password'), 'long-password-2');
    await user.click(screen.getByRole('button', { name: 'Create account' }));
    expect(await screen.findByText('Passwords do not match')).toBeInTheDocument();

    await user.clear(screen.getByLabelText('Confirm password'));
    await user.type(screen.getByLabelText('Confirm password'), 'long-password-1');
    await user.click(screen.getByRole('button', { name: 'Create account' }));
    expect(await screen.findByText('signed in as beat@example.com')).toBeInTheDocument();
    expect(api.lastEmailLink()).toMatch(/^\/verify-email\?token=/);
  });

  it('shows a conflict for an existing email', async () => {
    const user = userEvent.setup();
    renderAt('/register', '/register', <RegisterView />, { authApi: mock() });
    await user.type(await screen.findByLabelText('Display name'), 'Dup');
    await user.type(screen.getByLabelText('Email'), 'demo@openaudio.dev');
    await user.type(screen.getByLabelText('Password'), 'long-password-1');
    await user.type(screen.getByLabelText('Confirm password'), 'long-password-1');
    await user.click(screen.getByRole('button', { name: 'Create account' }));
    expect(await screen.findByRole('alert')).toHaveTextContent('already exists');
  });
});

describe('RequireAuth', () => {
  it('redirects anonymous visitors to /login with a return path', async () => {
    const { router } = renderAt('/library?tab=x', '/library', <RequireAuth><p>secret</p></RequireAuth>, {
      authApi: mock(),
      extraRoutes: [{ path: '/login', element: <p>login page</p> }],
    });
    expect(await screen.findByText('login page')).toBeInTheDocument();
    expect(router.state.location.search).toBe('?returnTo=%2Flibrary%3Ftab%3Dx');
  });

  it('enforces roles', async () => {
    const api = mock();
    await api.startOAuth('google', '/');
    renderAt('/publisher', '/publisher', <RequireAuth role="publisher"><p>dashboard</p></RequireAuth>, { authApi: api });
    expect(await screen.findByText('Publisher access required')).toBeInTheDocument();
    expect(screen.queryByText('dashboard')).not.toBeInTheDocument();
  });
});

describe('AuthCallbackView', () => {
  it('explains provider errors', async () => {
    renderAt('/auth/callback?error=oauth_denied&returnTo=/samples', '/auth/callback', <AuthCallbackView />, { authApi: mock() });
    expect(await screen.findByRole('alert')).toHaveTextContent('Sign-in was cancelled.');
    expect(screen.getByRole('link', { name: 'Back to sign in' })).toHaveAttribute('href', '/login?returnTo=%2Fsamples');
  });
});

describe('password reset', () => {
  it('requests a link and sets a new password with it', async () => {
    const user = userEvent.setup();
    const api = mock();
    const first = renderAt('/forgot-password', '/forgot-password', <ForgotPasswordView />, { authApi: api });
    await user.type(await screen.findByLabelText('Email'), 'demo@openaudio.dev');
    await user.click(screen.getByRole('button', { name: 'Send reset link' }));
    expect(await screen.findByRole('status')).toHaveTextContent('reset link is on its way');
    first.unmount();

    const link = api.lastEmailLink()!;
    renderAt(link, '/reset-password', <ResetPasswordView />, { authApi: api });
    await user.type(await screen.findByLabelText('New password'), 'brand-new-password');
    await user.type(screen.getByLabelText('Confirm password'), 'brand-new-password');
    await user.click(screen.getByRole('button', { name: 'Set password' }));
    expect(await screen.findByText('Password updated.')).toBeInTheDocument();
    await waitFor(async () => expect((await api.login('demo@openaudio.dev', 'brand-new-password')).email).toBe('demo@openaudio.dev'));
  });
});

describe('safeReturnTo', () => {
  it.each([
    ['/library', '/library'],
    [null, '/'],
    ['https://evil.example', '/'],
    ['//evil.example', '/'],
    ['/\\evil.example', '/'],
  ])('%s → %s', (input, out) => expect(safeReturnTo(input)).toBe(out));
});

describe('sign out', () => {
  it('sends the user home instead of to the login page', async () => {
    const user = userEvent.setup();
    const api = mock();
    await api.login('demo@openaudio.dev', 'openaudio-demo');
    function SignOut() {
      const { logout } = useAuth();
      return <button onClick={() => void logout()}>bye</button>;
    }
    const { router } = renderAt('/publisher', '/publisher', <RequireAuth role="publisher"><SignOut /></RequireAuth>, {
      authApi: api,
      extraRoutes: [{ path: '/', element: <p>home</p> }],
    });
    await user.click(await screen.findByRole('button', { name: 'bye' }));
    expect(await screen.findByText('home')).toBeInTheDocument();
    expect(router.state.location.pathname).toBe('/');
  });
});
