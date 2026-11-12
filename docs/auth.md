# Authentication

OpenAudio supports three ways to sign in:

| Method | How it works |
|---|---|
| **Email + password** | Argon2id hashes (`m=64MiB, t=3, p=2`, PHC string format; re-hashed on login when parameters grow). Email verification and password reset via single-use links. |
| **Google** | OAuth 2.0 authorization code + PKCE (S256), scopes `openid email profile`. |
| **Discord** | OAuth 2.0 authorization code, scopes `identify email`. |

## Sessions

- **Access token** — JWT (HS256), 15 minutes, carries `sub` (user id) and `role`. Sent as `Authorization: Bearer`.
  The web app keeps it **in memory only**.
- **Refresh token** — 256-bit random value, stored **hashed** (SHA-256) in `refresh_tokens`, 30 days.
  Delivered as an `HttpOnly; SameSite=Lax; Path=/api/v1/auth` cookie (`oa_refresh`) and in the JSON body for
  non-browser clients. Every refresh **rotates** it; replaying an already rotated token revokes the whole token
  family (theft detection). Logout revokes the family; a password reset revokes all of the user's sessions.
- On page load the SPA calls `POST /auth/refresh`; the cookie turns into a fresh access token.

## Linking accounts

When someone signs in with Google/Discord:

1. Known `(provider, provider user id)` → that account.
2. Otherwise, if the provider says the email is **verified** and an account with that email exists → the
   provider is linked to it (and the email becomes verified).
3. An **unverified** provider email never takes over an existing account (`email_unverified_at_provider`).
4. Otherwise a new account without a password is created. Such users can add a password through
   "Forgot password".

## OAuth flow

```text
Browser ──GET /api/v1/auth/oauth/google/start?returnTo=/library──▶ API
        ◀─302 accounts.google.com … + Set-Cookie oa_oauth (signed: state, PKCE verifier, returnTo; 10 min)
Browser ──consent──▶ Google ──302──▶ /api/v1/auth/oauth/google/callback?code&state
API: verify cookie signature + state, exchange code (+verifier), fetch userinfo, sign in / link / sign up
        ◀─302 {AUTH_CALLBACK_URL}?returnTo=/library + Set-Cookie oa_refresh
SPA:  POST /api/v1/auth/refresh → access token → GET /me → navigate to /library
```

Errors come back as `{AUTH_CALLBACK_URL}?error=<code>` (`oauth_denied`, `oauth_state_mismatch`,
`oauth_exchange_failed`, `oauth_email_missing`, `email_unverified_at_provider`, …) and are shown on that page.
`returnTo` only accepts relative paths (no open redirects).

### Where users land after signing in

| Variable | Default | Meaning |
|---|---|---|
| `AUTH_REDIRECT_PATH` | `/` | Page after sign-in (email or OAuth) when the user did not come from a protected page. Also used when `returnTo` is missing or unsafe. Must be a site path (`/library`). docker compose passes it to the SPA as `VITE_AUTH_REDIRECT_PATH`; set that one yourself when running Vite outside compose. |
| `AUTH_CALLBACK_URL` | `{WEB_URL}/auth/callback` | The SPA page OAuth finishes at; it receives `?returnTo=` and `?error=` (existing query parameters are kept). Change it only if the frontend lives elsewhere. |

`GET /api/v1/auth/providers` returns the server's `redirectPath`.

## Base domain

`BASE_DOMAIN=openaudio.example.com` (host with an optional port, no scheme) is the one setting a deployment needs:
`WEB_URL` and `PUBLIC_URL` default to `https://openaudio.example.com` (`http://` for `localhost`/`127.x`;
override with `BASE_SCHEME`), and CORS, cookies (`Secure` on https), OAuth redirect URIs, email links,
`AUTH_CALLBACK_URL` and the mail sender (`no-reply@openaudio.example.com`) follow. Explicit `WEB_URL` /
`PUBLIC_URL` still win. The Vite dev server also accepts that host name (`server.allowedHosts`; more hosts via
`VITE_ALLOWED_HOSTS`). `S3_PUBLIC_ENDPOINT` is separate: set it to the address browsers use to reach object storage ([storage.md](storage.md)).

In development the Vite dev server proxies `/api` to the Go API, so the API, cookies and redirect URIs all live on
`http://localhost:5173`.

## Setting up providers

### Google
1. <https://console.cloud.google.com/apis/credentials> → *Create credentials* → *OAuth client ID* → *Web application*.
2. Authorized redirect URI: `http://localhost:5173/api/v1/auth/oauth/google/callback`
   (with `BASE_DOMAIN`: `https://<BASE_DOMAIN>/api/v1/auth/oauth/google/callback`).
3. Put the client id/secret into `.env` as `GOOGLE_CLIENT_ID` / `GOOGLE_CLIENT_SECRET`.

### Discord
1. <https://discord.com/developers/applications> → *New Application* → *OAuth2*.
2. Add redirect: `http://localhost:5173/api/v1/auth/oauth/discord/callback`
   (with `BASE_DOMAIN`: `https://<BASE_DOMAIN>/api/v1/auth/oauth/discord/callback`).
3. Copy *Client ID* / *Client Secret* into `DISCORD_CLIENT_ID` / `DISCORD_CLIENT_SECRET`.

A provider is enabled when both its id and secret are set; `GET /api/v1/auth/providers` tells the UI which buttons to show.
The API logs the exact redirect URI for each enabled provider on start-up.

## Email

Without `SMTP_HOST` the API writes verification and reset emails to its log
(`docker compose logs api | grep verify-email`). Configure `SMTP_*` to send real mail.

## Roles

`user` (default) → `publisher` → `admin`.

**Default admin.** With `ADMIN_EMAIL` and `ADMIN_PASSWORD` set, the API makes sure on every start that this account
exists, is an admin and has a verified email. `.env` is the source of truth for its password: if the stored one
differs (changed in `.env`, or through the UI), it is reset to `ADMIN_PASSWORD` and the admin's sessions are
revoked. An existing account with that email is promoted rather than duplicated. The password must satisfy the
normal rules (≥ 10 characters); the `.env.example` placeholder is refused with `APP_ENV=production`.

Emails listed in `ADMIN_EMAILS` also become admins when their account is created. Admins change roles with `PATCH /api/v1/admin/users/{id}/role`. The role is embedded in access tokens,
so a change takes effect on the next refresh (≤ 15 minutes).

## Abuse protection

Register, login, verify, resend, forgot and reset endpoints are rate limited per client IP
(10/min, burst 5; set `TRUST_PROXY=true` behind a reverse proxy so `X-Forwarded-For` is honoured).
Login spends the same Argon2 time for unknown emails, and "forgot password" always answers 202,
so neither reveals which emails are registered.
