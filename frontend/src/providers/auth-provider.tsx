"use client";

import React, {
  createContext,
  useContext,
  useState,
  useEffect,
  useCallback,
} from "react";
import { usePathname, useRouter } from "next/navigation";
import {
  login as authLogin,
  logout as authLogout,
  isAuthenticated as checkAuth,
  getToken,
} from "@/lib/auth";
import type { User } from "@/lib/types";

interface AuthContextValue {
  user: User | null;
  isAuthenticated: boolean;
  isLoading: boolean;
  login: (username: string, password: string) => Promise<void>;
  logout: () => void;
}

const AuthContext = createContext<AuthContextValue | undefined>(undefined);

const PUBLIC_PATHS = ["/login"];

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [isAuthenticated, setIsAuthenticated] = useState(false);
  const [isLoading, setIsLoading] = useState(true);
  const router = useRouter();
  const pathname = usePathname();

  // Check token on mount
  useEffect(() => {
    const token = getToken();
    if (token && checkAuth()) {
      setIsAuthenticated(true);
      setUser({ username: "admin" });
    } else {
      setIsAuthenticated(false);
      setUser(null);
    }
    setIsLoading(false);
  }, []);

  // Redirect to login if not authenticated and not on a public path
  useEffect(() => {
    if (isLoading) return;

    const isPublicPath = PUBLIC_PATHS.some((p) => pathname?.startsWith(p));

    if (!isAuthenticated && !isPublicPath) {
      router.push("/login");
    }
  }, [isAuthenticated, isLoading, pathname, router]);

  const login = useCallback(
    async (username: string, password: string) => {
      await authLogin(username, password);
      setUser({ username });
      setIsAuthenticated(true);
      router.push("/dashboard");
    },
    [router]
  );

  const logout = useCallback(() => {
    authLogout();
    setUser(null);
    setIsAuthenticated(false);
  }, []);

  return (
    <AuthContext.Provider
      value={{ user, isAuthenticated, isLoading, login, logout }}
    >
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth(): AuthContextValue {
  const context = useContext(AuthContext);
  if (context === undefined) {
    throw new Error("useAuth must be used within an AuthProvider");
  }
  return context;
}
