import { createContext, useContext, useState, useEffect, useCallback, type ReactNode } from 'react';

// ── Configuration ──────────────────────────────────────────────
const KEYCLOAK_URL = import.meta.env.VITE_KEYCLOAK_URL || '';
const KEYCLOAK_REALM = import.meta.env.VITE_KEYCLOAK_REALM || 'keyip';
const KEYCLOAK_CLIENT_ID = import.meta.env.VITE_KEYCLOAK_CLIENT_ID || 'keyip-web';
const REDIRECT_URI = `${window.location.origin}/login`;

const STORAGE_KEYS = {
  accessToken: 'keyip_access_token',
  refreshToken: 'keyip_refresh_token',
  idToken: 'keyip_id_token',
  tokenExpiry: 'keyip_token_expiry',
  codeVerifier: 'keyip_code_verifier',
  userInfo: 'keyip_user_info',
  state: 'keyip_oauth_state',
  loginRedirect: 'keyip_login_redirect',
};

// ── Types ──────────────────────────────────────────────────────
export interface UserProfile {
  sub: string;
  preferred_username: string;
  email: string;
  name: string;
  given_name: string;
  family_name: string;
  picture?: string;
}

// ── PKCE Helpers ──────────────────────────────────────────────
function base64URLEncode(buffer: ArrayBuffer): string {
  return btoa(String.fromCharCode(...new Uint8Array(buffer)))
    .replace(/\+/g, '-')
    .replace(/\//g, '_')
    .replace(/=+$/, '');
}

function generateRandomString(length: number): string {
  const array = new Uint8Array(length);
  crypto.getRandomValues(array);
  return base64URLEncode(array.buffer);
}

async function generateCodeChallenge(verifier: string): Promise<string> {
  const encoder = new TextEncoder();
  const data = encoder.encode(verifier);
  const digest = await crypto.subtle.digest('SHA-256', data);
  return base64URLEncode(digest);
}

// ── JWT Helpers ───────────────────────────────────────────────
function decodeJWT(token: string): Record<string, unknown> | null {
  try {
    const parts = token.split('.');
    if (parts.length !== 3) return null;
    const payload = parts[1];
    const padded = payload.padEnd(payload.length + (4 - (payload.length % 4)) % 4, '=');
    const decoded = atob(padded.replace(/-/g, '+').replace(/_/g, '/'));
    return JSON.parse(decoded);
  } catch {
    return null;
  }
}

function isTokenExpired(token: string): boolean {
  const payload = decodeJWT(token);
  if (!payload || !payload.exp) return true;
  const expiry = (payload.exp as number) * 1000; // convert to ms
  // Consider token expired 30 seconds before actual expiry to avoid race conditions
  return Date.now() >= expiry - 30000;
}

// ── Token Storage ─────────────────────────────────────────────
export function getStoredAccessToken(): string | null {
  return sessionStorage.getItem(STORAGE_KEYS.accessToken);
}

function getStoredRefreshToken(): string | null {
  return sessionStorage.getItem(STORAGE_KEYS.refreshToken);
}

function clearTokens(): void {
  Object.values(STORAGE_KEYS).forEach(key => sessionStorage.removeItem(key));
}

function storeTokens(accessToken: string, refreshToken: string, idToken?: string): void {
  sessionStorage.setItem(STORAGE_KEYS.accessToken, accessToken);
  sessionStorage.setItem(STORAGE_KEYS.refreshToken, refreshToken);
  if (idToken) {
    sessionStorage.setItem(STORAGE_KEYS.idToken, idToken);
  }
  // Store expiry
  const payload = decodeJWT(accessToken);
  if (payload?.exp) {
    sessionStorage.setItem(STORAGE_KEYS.tokenExpiry, String(payload.exp));
  }
  // Store user info from ID token or access token
  const userInfo = extractUserInfo(idToken || accessToken);
  if (userInfo) {
    sessionStorage.setItem(STORAGE_KEYS.userInfo, JSON.stringify(userInfo));
  }
}

function extractUserInfo(token: string): UserProfile | null {
  const payload = decodeJWT(token);
  if (!payload) return null;
  return {
    sub: (payload.sub as string) || '',
    preferred_username: (payload.preferred_username as string) || (payload.sub as string) || '',
    email: (payload.email as string) || '',
    name: (payload.name as string) || (payload.preferred_username as string) || '',
    given_name: (payload.given_name as string) || '',
    family_name: (payload.family_name as string) || '',
    picture: payload.picture as string | undefined,
  };
}

export function getStoredUserProfile(): UserProfile | null {
  const stored = sessionStorage.getItem(STORAGE_KEYS.userInfo);
  if (!stored) return null;
  try {
    return JSON.parse(stored) as UserProfile;
  } catch {
    return null;
  }
}

// ── Keycloak URL Builders ─────────────────────────────────────
function keycloakBaseUrl(): string {
  return `${KEYCLOAK_URL}/realms/${KEYCLOAK_REALM}/protocol/openid-connect`;
}

function isKeycloakConfigured(): boolean {
  return !!KEYCLOAK_URL;
}

// ── Public API ────────────────────────────────────────────────

/** Check if we can use local email+password sign-in (backend auth, no Keycloak). */
export function isLocalAuthAvailable(): boolean {
  return true;
}

/**
 * Sign in via the backend's local auth endpoint (email + password → JWT).
 * Does NOT redirect — returns a result object with success/error.
 */
export async function localSignIn(email: string, password: string): Promise<{ success: boolean; error?: string }> {
  try {
    // Use same-origin /api/v1/auth/signin (nginx proxies to apiserver)
    const resp = await fetch('/api/v1/auth/signin', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ email, password }),
    });

    if (!resp.ok) {
      const body = await resp.json().catch(() => null);
      const msg = body?.error?.message || body?.message || `HTTP ${resp.status}`;
      return { success: false, error: msg };
    }

    const body = await resp.json();
    // API returns flat { access_token, token_type, expires_in } (not wrapped in data)
    const token = body.access_token;
    if (!token) {
      return { success: false, error: 'No access_token in response' };
    }

    // Store locally (same format as Keycloak flow)
    storeTokens(token, body.refresh_token || '');
    sessionStorage.setItem(STORAGE_KEYS.tokenExpiry, String(Math.floor(Date.now() / 1000) + (body.expires_in || 86400)));

    // Decode user info from JWT
    const userInfo = extractUserInfo(token);
    if (userInfo) {
      sessionStorage.setItem(STORAGE_KEYS.userInfo, JSON.stringify(userInfo));
    }

    return { success: true };
  } catch (err) {
    return { success: false, error: err instanceof Error ? err.message : String(err) };
  }
}

export async function login(redirectTo?: string): Promise<void> {
  if (!isKeycloakConfigured()) {
    console.warn('[Auth] Keycloak is not configured. Set VITE_KEYCLOAK_URL env var.');
    return;
  }

  // Store the intended redirect path so we can come back after login
  if (redirectTo) {
    sessionStorage.setItem(STORAGE_KEYS.loginRedirect, redirectTo);
  }

  const codeVerifier = generateRandomString(32);
  sessionStorage.setItem(STORAGE_KEYS.codeVerifier, codeVerifier);

  const state = generateRandomString(16);
  sessionStorage.setItem(STORAGE_KEYS.state, state);

  const codeChallenge = await generateCodeChallenge(codeVerifier);

  const params = new URLSearchParams({
    response_type: 'code',
    client_id: KEYCLOAK_CLIENT_ID,
    redirect_uri: REDIRECT_URI,
    code_challenge: codeChallenge,
    code_challenge_method: 'S256',
    state,
    scope: 'openid profile email',
  });

  const authUrl = `${keycloakBaseUrl()}/auth?${params.toString()}`;
  window.location.href = authUrl;
}

export async function handleCallback(): Promise<{ success: boolean; error?: string }> {
  const params = new URLSearchParams(window.location.search);
  const code = params.get('code');
  const state = params.get('state');
  const oauthError = params.get('error');

  // Check for OAuth error
  if (oauthError) {
    return { success: false, error: `OAuth error: ${oauthError}` };
  }

  if (!code) {
    return { success: false, error: 'No authorization code found in URL' };
  }

  // Verify state to prevent CSRF
  const storedState = sessionStorage.getItem(STORAGE_KEYS.state);
  if (state && storedState && state !== storedState) {
    return { success: false, error: 'State mismatch. Possible CSRF attack.' };
  }

  // Get code verifier for PKCE
  const codeVerifier = sessionStorage.getItem(STORAGE_KEYS.codeVerifier);
  if (!codeVerifier) {
    return { success: false, error: 'No code verifier found. Login session expired.' };
  }

  try {
    const tokenResponse = await fetch(`${keycloakBaseUrl()}/token`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
      body: new URLSearchParams({
        grant_type: 'authorization_code',
        client_id: KEYCLOAK_CLIENT_ID,
        code,
        redirect_uri: REDIRECT_URI,
        code_verifier: codeVerifier,
      }),
    });

    if (!tokenResponse.ok) {
      const errorBody = await tokenResponse.text();
      return { success: false, error: `Token exchange failed: ${tokenResponse.status} ${errorBody}` };
    }

    const tokens = await tokenResponse.json();
    storeTokens(tokens.access_token, tokens.refresh_token, tokens.id_token);

    // Clean up PKCE artifacts
    sessionStorage.removeItem(STORAGE_KEYS.codeVerifier);
    sessionStorage.removeItem(STORAGE_KEYS.state);

    return { success: true };
  } catch (err) {
    return { success: false, error: `Token exchange failed: ${err instanceof Error ? err.message : String(err)}` };
  }
}

export async function refreshAccessToken(): Promise<string | null> {
  const refreshToken = getStoredRefreshToken();
  if (!refreshToken) return null;

  try {
    const response = await fetch(`${keycloakBaseUrl()}/token`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
      body: new URLSearchParams({
        grant_type: 'refresh_token',
        client_id: KEYCLOAK_CLIENT_ID,
        refresh_token: refreshToken,
      }),
    });

    if (!response.ok) {
      clearTokens();
      return null;
    }

    const tokens = await response.json();
    storeTokens(tokens.access_token, tokens.refresh_token || refreshToken, tokens.id_token);
    return tokens.access_token;
  } catch {
    clearTokens();
    return null;
  }
}

/**
 * Get a valid access token for API requests.
 * If the token is expired, attempts to refresh it.
 * Returns null if no valid token is available or auth is not applicable.
 */
export async function getAccessToken(): Promise<string | null> {
  const token = getStoredAccessToken();
  if (!token) return null;

  if (isKeycloakConfigured()) {
    if (isTokenExpired(token)) {
      return await refreshAccessToken();
    }
    return token;
  }

  // Local auth: just check expiry
  if (isTokenExpired(token)) {
    clearTokens();
    return null;
  }
  return token;
}

export function isAuthenticated(): boolean {
  const token = getStoredAccessToken();
  if (!token) return false;
  return !isTokenExpired(token);
}

export function logout(): void {
  const idToken = sessionStorage.getItem(STORAGE_KEYS.idToken);
  clearTokens();

  if (isKeycloakConfigured()) {
    const params = new URLSearchParams({
      client_id: KEYCLOAK_CLIENT_ID,
      post_logout_redirect_uri: window.location.origin,
    });
    if (idToken) {
      params.set('id_token_hint', idToken);
    }
    window.location.href = `${keycloakBaseUrl()}/logout?${params.toString()}`;
  } else {
    window.location.href = '/';
  }
}

/** Retrieve the stored redirect path (cleared after reading) */
export function getLoginRedirect(): string | null {
  const path = sessionStorage.getItem(STORAGE_KEYS.loginRedirect);
  sessionStorage.removeItem(STORAGE_KEYS.loginRedirect);
  return path;
}

// ── React Context ─────────────────────────────────────────────
export interface AuthContextValue {
  isAuthenticated: boolean;
  isLoading: boolean;
  user: UserProfile | null;
  login: (redirectTo?: string) => void;
  logout: () => void;
  getAccessToken: () => Promise<string | null>;
}

const AuthContext = createContext<AuthContextValue>({
  isAuthenticated: false,
  isLoading: true,
  user: null,
  login: () => {},
  logout: () => {},
  getAccessToken: async () => null,
});

export function AuthProvider({ children }: { children: ReactNode }) {
  const [isLoading, setIsLoading] = useState(true);
  const [authState, setAuthState] = useState<{
    isAuthenticated: boolean;
    user: UserProfile | null;
  }>({
    isAuthenticated: false,
    user: null,
  });

  // Check auth state on mount
  useEffect(() => {
    const check = () => {
      const authed = isAuthenticated();
      const user = authed ? getStoredUserProfile() : null;
      setAuthState({ isAuthenticated: authed, user });
      setIsLoading(false);
    };
    check();
  }, []);

  const handleLogin = useCallback((redirectTo?: string) => {
    login(redirectTo);
  }, []);

  const handleLogout = useCallback(() => {
    logout();
  }, []);

  const value: AuthContextValue = {
    ...authState,
    isLoading,
    login: handleLogin,
    logout: handleLogout,
    getAccessToken,
  };

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth(): AuthContextValue {
  return useContext(AuthContext);
}
