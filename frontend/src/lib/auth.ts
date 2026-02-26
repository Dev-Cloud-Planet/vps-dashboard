import type { AuthTokenResponse } from "./types";

const TOKEN_KEY = "vps_dashboard_token";
const API_BASE_URL =
  (process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080") + "/api";

/**
 * Get the auth token from localStorage.
 */
export function getToken(): string | null {
  if (typeof window === "undefined") return null;
  return localStorage.getItem(TOKEN_KEY);
}

/**
 * Store the auth token in localStorage.
 */
export function setToken(token: string): void {
  if (typeof window === "undefined") return;
  localStorage.setItem(TOKEN_KEY, token);
}

/**
 * Remove the auth token from localStorage.
 */
export function removeToken(): void {
  if (typeof window === "undefined") return;
  localStorage.removeItem(TOKEN_KEY);
}

/**
 * Check if the user is authenticated (has a token stored).
 */
export function isAuthenticated(): boolean {
  const token = getToken();
  return token !== null && token.length > 0;
}

/**
 * Log in with username and password.
 * Calls the API, stores the token on success.
 */
export async function login(
  username: string,
  password: string
): Promise<AuthTokenResponse> {
  const response = await fetch(`${API_BASE_URL}/auth/login`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ username, password }),
  });

  if (!response.ok) {
    const body = await response.text();
    let message = "Login failed";
    try {
      const parsed = JSON.parse(body);
      if (parsed.error) message = parsed.error;
    } catch {
      // use default message
    }
    throw new Error(message);
  }

  const data: AuthTokenResponse = await response.json();
  setToken(data.token);
  return data;
}

/**
 * Log out by removing the token and redirecting to login.
 */
export function logout(): void {
  removeToken();
  if (typeof window !== "undefined") {
    window.location.href = "/login";
  }
}
