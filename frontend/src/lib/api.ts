import { getToken, removeToken } from "./auth";

const API_BASE_URL =
  (process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080") + "/api";

export class APIError extends Error {
  status: number;
  body: string;

  constructor(message: string, status: number, body: string) {
    super(message);
    this.name = "APIError";
    this.status = status;
    this.body = body;
  }
}

/**
 * Core fetch wrapper with auth token injection and error handling.
 */
async function fetchAPI<T>(
  path: string,
  options: RequestInit = {}
): Promise<T> {
  const token = getToken();

  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    ...(options.headers as Record<string, string>),
  };

  if (token) {
    headers["Authorization"] = `Bearer ${token}`;
  }

  const url = `${API_BASE_URL}${path}`;

  const response = await fetch(url, {
    ...options,
    headers,
  });

  // Handle 401 Unauthorized - clear token and redirect to login
  if (response.status === 401) {
    removeToken();
    if (typeof window !== "undefined") {
      window.location.href = "/login";
    }
    throw new APIError("Unauthorized", 401, "");
  }

  // Handle other error responses
  if (!response.ok) {
    const body = await response.text();
    throw new APIError(
      `API Error: ${response.status} ${response.statusText}`,
      response.status,
      body
    );
  }

  // Handle 204 No Content
  if (response.status === 204) {
    return undefined as T;
  }

  return response.json() as Promise<T>;
}

/**
 * GET request helper.
 */
export function get<T>(path: string): Promise<T> {
  return fetchAPI<T>(path, { method: "GET" });
}

/**
 * POST request helper.
 */
export function post<T>(path: string, body?: unknown): Promise<T> {
  return fetchAPI<T>(path, {
    method: "POST",
    body: body ? JSON.stringify(body) : undefined,
  });
}

/**
 * PUT request helper.
 */
export function put<T>(path: string, body?: unknown): Promise<T> {
  return fetchAPI<T>(path, {
    method: "PUT",
    body: body ? JSON.stringify(body) : undefined,
  });
}

/**
 * DELETE request helper.
 */
export function del<T>(path: string): Promise<T> {
  return fetchAPI<T>(path, { method: "DELETE" });
}

/**
 * SWR fetcher function for use with useSWR.
 */
export function swrFetcher<T>(url: string): Promise<T> {
  return get<T>(url);
}

export { fetchAPI };
export default { get, post, put, del, fetchAPI };
