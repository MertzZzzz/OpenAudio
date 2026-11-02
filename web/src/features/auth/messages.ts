import { ApiError } from '@/shared/api/client';

/** Human-readable messages for API error codes (incl. OAuth callback `?error=`). */
const MESSAGES: Record<string, string> = {
  invalid_credentials: 'Email or password is incorrect.',
  email_taken: 'An account with this email already exists. Sign in instead.',
  rate_limited: 'Too many attempts. Wait a minute and try again.',
  token_invalid: 'This link is invalid or has expired. Request a new one.',
  oauth_denied: 'Sign-in was cancelled.',
  oauth_state_mismatch: 'Sign-in session expired or was tampered with. Please try again.',
  oauth_exchange_failed: 'The provider did not confirm your sign-in. Please try again.',
  oauth_email_missing: 'The provider did not share your email address. Allow email access and try again.',
  email_unverified_at_provider:
    'An account with this email already exists, but the provider has not verified the address. Sign in with your password instead.',
  provider_not_enabled: 'This sign-in method is not enabled on this server.',
  invalid_refresh_token: 'Your session has expired. Please sign in again.',
};

export function authErrorMessage(codeOrError: unknown): string {
  if (typeof codeOrError === 'string') return MESSAGES[codeOrError] ?? 'Something went wrong. Please try again.';
  if (codeOrError instanceof ApiError) return MESSAGES[codeOrError.code] ?? codeOrError.message;
  return 'Network error. Check your connection and try again.';
}
