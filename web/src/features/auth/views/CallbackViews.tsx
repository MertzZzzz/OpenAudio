import { useEffect, useRef, useState } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { Loader2 } from 'lucide-react';
import { ButtonLink } from '@/shared/ui/Button';
import { useAuth } from '../AuthContext';
import { authErrorMessage } from '../messages';
import { safeReturnTo } from '../forms';
import { Alert, AuthCard } from './AuthCard';

/** Landing page after Google/Discord: the API already set the refresh cookie (or passed ?error=). */
export function AuthCallbackView() {
  const { completeOAuth } = useAuth();
  const [sp] = useSearchParams();
  const navigate = useNavigate();
  const returnTo = safeReturnTo(sp.get('returnTo'));
  const errorCode = sp.get('error');
  const [error, setError] = useState<string | null>(errorCode ? authErrorMessage(errorCode) : null);
  const started = useRef(false);

  useEffect(() => {
    if (errorCode || started.current) return;
    started.current = true;
    completeOAuth()
      .then(() => navigate(returnTo, { replace: true }))
      .catch((e: unknown) => setError(authErrorMessage(e)));
  }, [completeOAuth, errorCode, navigate, returnTo]);

  if (error) {
    return (
      <AuthCard title="Sign-in failed">
        <Alert tone="error">{error}</Alert>
        <ButtonLink to={`/login?returnTo=${encodeURIComponent(returnTo)}`} size="lg" className="w-full">Back to sign in</ButtonLink>
      </AuthCard>
    );
  }
  return (
    <div className="flex items-center justify-center gap-3 py-24 text-fg-muted" role="status">
      <Loader2 className="h-5 w-5 animate-spin" aria-hidden /> Finishing sign-in…
    </div>
  );
}

export function VerifyEmailView() {
  const { api, status, reload } = useAuth();
  const [sp] = useSearchParams();
  const token = sp.get('token') ?? '';
  const [state, setState] = useState<'pending' | 'ok' | 'error'>(token ? 'pending' : 'error');
  const [message, setMessage] = useState(token ? '' : authErrorMessage('token_invalid'));
  const started = useRef(false);

  useEffect(() => {
    if (!token || started.current) return;
    started.current = true;
    api
      .verifyEmail(token)
      .then(() => {
        setState('ok');
        if (status === 'authenticated') void reload();
      })
      .catch((e: unknown) => {
        setState('error');
        setMessage(authErrorMessage(e));
      });
  }, [api, token, status, reload]);

  return (
    <AuthCard title="Confirm your email">
      {state === 'pending' && (
        <p className="flex items-center gap-2 text-fg-muted" role="status">
          <Loader2 className="h-4 w-4 animate-spin" aria-hidden /> Verifying…
        </p>
      )}
      {state === 'ok' && <Alert tone="success">Your email address is confirmed. Thanks!</Alert>}
      {state === 'error' && <Alert tone="error">{message}</Alert>}
      <ButtonLink to={status === 'authenticated' ? '/account' : '/login'} variant="secondary" className="w-full">{status === 'authenticated' ? 'Go to account' : 'Sign in'}</ButtonLink>
    </AuthCard>
  );
}
