import { useState } from 'react';
import { Link, useSearchParams } from 'react-router-dom';
import { BadgeCheck, KeyRound, LogOut, MailWarning } from 'lucide-react';
import { Badge } from '@/shared/ui/Badge';
import { Button, ButtonLink } from '@/shared/ui/Button';
import { Card, CardBody, CardHeader, CardTitle } from '@/shared/ui/Card';
import { useToast } from '@/shared/ui/Toast';
import { formatDate } from '@/shared/lib/format';
import { useAuth } from '../AuthContext';
import { authErrorMessage } from '../messages';
import { UserAvatar } from '../UserMenu';

const PROVIDER_LABEL = { google: 'Google', discord: 'Discord' } as const;

export function AccountView() {
  const { user, api, logout } = useAuth();
  const { toast } = useToast();
  const [sp] = useSearchParams();
  const [sending, setSending] = useState(false);
  if (!user) return null;

  const resend = async () => {
    setSending(true);
    try {
      await api.resendVerification();
      toast({ tone: 'success', title: 'Verification email sent', description: `Check ${user.email}.` });
    } catch (e) {
      toast({ tone: 'error', title: 'Could not send email', description: authErrorMessage(e) });
    } finally {
      setSending(false);
    }
  };

  return (
    <div className="mx-auto max-w-3xl space-y-6">
      {sp.get('welcome') && (
        <div className="rounded-lg border border-primary/40 bg-primary/10 p-4 text-body">
          Welcome to OpenAudio, {user.displayName}! <Link to="/" className="text-primary hover:underline">Start exploring →</Link>
        </div>
      )}
      <div className="flex items-center gap-4">
        <UserAvatar user={user} size="lg" />
        <div className="min-w-0">
          <h1 className="truncate text-display">{user.displayName}</h1>
          <p className="text-fg-muted">
            {user.email} · member since {formatDate(user.createdAt)}
          </p>
        </div>
        <Badge tone={user.role === 'admin' ? 'danger' : user.role === 'publisher' ? 'primary' : 'neutral'} className="ml-auto">
          {user.role}
        </Badge>
      </div>

      {!user.emailVerified && (
        <div className="flex flex-wrap items-center gap-3 rounded-lg border border-warning/40 bg-warning/10 p-4" role="status">
          <MailWarning className="h-5 w-5 text-warning" aria-hidden />
          <p className="flex-1 text-body">Confirm your email address to publish packages and recover your account.</p>
          <Button size="sm" variant="secondary" loading={sending} onClick={() => void resend()}>
            Resend email
          </Button>
        </div>
      )}

      <Card>
        <CardHeader>
          <CardTitle>Sign-in methods</CardTitle>
        </CardHeader>
        <CardBody className="p-0">
          <ul className="divide-y divide-border">
            <li className="flex items-center gap-3 px-6 py-4">
              <KeyRound className="h-5 w-5 text-fg-muted" aria-hidden />
              <div className="flex-1">
                <p className="font-medium">Email & password</p>
                <p className="text-caption text-fg-subtle">{user.hasPassword ? 'Password set' : 'No password yet — you signed up with a provider.'}</p>
              </div>
              <ButtonLink to="/forgot-password" size="sm" variant="ghost">{user.hasPassword ? 'Change' : 'Set password'}</ButtonLink>
            </li>
            {(['google', 'discord'] as const).map((p) => {
              const linked = user.identities?.find((i) => i.provider === p);
              return (
                <li key={p} className="flex items-center gap-3 px-6 py-4">
                  <BadgeCheck className={linked ? 'h-5 w-5 text-success' : 'h-5 w-5 text-fg-subtle'} aria-hidden />
                  <div className="flex-1">
                    <p className="font-medium">{PROVIDER_LABEL[p]}</p>
                    <p className="text-caption text-fg-subtle">
                      {linked ? `Linked ${formatDate(linked.linkedAt)}${linked.email ? ` · ${linked.email}` : ''}` : 'Not linked — sign in with it once using the same verified email to link.'}
                    </p>
                  </div>
                  {linked && <Badge tone="success">linked</Badge>}
                </li>
              );
            })}
          </ul>
        </CardBody>
      </Card>

      <div className="flex justify-end">
        <Button
          variant="danger"
          leftIcon={<LogOut className="h-4 w-4" />}
          onClick={() => void logout()}
        >
          Sign out
        </Button>
      </div>
    </div>
  );
}
