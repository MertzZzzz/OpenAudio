export type Role = 'user' | 'publisher' | 'admin';
export type OAuthProviderId = 'google' | 'discord';

export interface AuthIdentity {
  provider: OAuthProviderId;
  email?: string;
  linkedAt: string;
}

export interface AuthUser {
  id: string;
  email: string;
  displayName: string;
  role: Role;
  avatarUrl: string | null;
  emailVerified: boolean;
  hasPassword: boolean;
  identities?: AuthIdentity[];
  createdAt: string;
}

export interface AuthProviders {
  email: boolean;
  oauth: { id: OAuthProviderId; startUrl: string }[];
}

export interface RegisterInput {
  email: string;
  password: string;
  displayName: string;
}

const RANK: Record<Role, number> = { user: 1, publisher: 2, admin: 3 };

export function hasRole(user: Pick<AuthUser, 'role'> | null | undefined, min: Role): boolean {
  return Boolean(user && RANK[user.role] >= RANK[min]);
}
