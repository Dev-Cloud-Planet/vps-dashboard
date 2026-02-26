"use client";

import { useState } from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { AuthProvider, useAuth } from "@/providers/auth-provider";
import { WebSocketProvider, useWS } from "@/providers/websocket-provider";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import {
  LayoutDashboard,
  Container,
  Shield,
  Bell,
  BarChart3,
  Settings,
  LogOut,
  Menu,
  X,
  ChevronDown,
  Server,
} from "lucide-react";

const navItems = [
  { href: "/dashboard", label: "Overview", icon: LayoutDashboard },
  { href: "/dashboard/containers", label: "Containers", icon: Container },
  { href: "/dashboard/security", label: "Security", icon: Shield },
  { href: "/dashboard/alerts", label: "Alerts", icon: Bell },
  { href: "/dashboard/metrics", label: "Metrics", icon: BarChart3 },
  { href: "/dashboard/settings", label: "Settings", icon: Settings },
];

function Sidebar({
  open,
  onClose,
}: {
  open: boolean;
  onClose: () => void;
}) {
  const pathname = usePathname();

  return (
    <>
      {/* Mobile overlay */}
      {open && (
        <div
          className="fixed inset-0 z-40 bg-black/60 lg:hidden"
          onClick={onClose}
        />
      )}

      <aside
        className={cn(
          "fixed inset-y-0 left-0 z-50 flex w-64 flex-col bg-[#111] border-r border-zinc-800 transition-transform duration-200 lg:translate-x-0 lg:static lg:z-auto",
          open ? "translate-x-0" : "-translate-x-full"
        )}
      >
        {/* Logo / Brand */}
        <div className="flex h-16 items-center gap-3 border-b border-zinc-800 px-6">
          <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-blue-600">
            <Server className="h-4 w-4 text-white" />
          </div>
          <span className="text-lg font-bold text-zinc-100">VPS Dashboard</span>
          <button
            onClick={onClose}
            className="ml-auto text-zinc-400 hover:text-zinc-100 lg:hidden"
          >
            <X className="h-5 w-5" />
          </button>
        </div>

        {/* Navigation */}
        <nav className="flex-1 space-y-1 px-3 py-4">
          {navItems.map((item) => {
            const isActive =
              item.href === "/dashboard"
                ? pathname === "/dashboard"
                : pathname?.startsWith(item.href);
            return (
              <Link
                key={item.href}
                href={item.href}
                onClick={onClose}
                className={cn(
                  "flex items-center gap-3 rounded-lg px-3 py-2.5 text-sm font-medium transition-colors",
                  isActive
                    ? "bg-zinc-800 text-zinc-100"
                    : "text-zinc-400 hover:bg-zinc-800/50 hover:text-zinc-200"
                )}
              >
                <item.icon className="h-5 w-5 shrink-0" />
                {item.label}
              </Link>
            );
          })}
        </nav>

        {/* Bottom info */}
        <div className="border-t border-zinc-800 p-4">
          <p className="text-xs text-zinc-500">VPS Monitor v1.0</p>
        </div>
      </aside>
    </>
  );
}

function TopBar({ onMenuClick }: { onMenuClick: () => void }) {
  const { user, logout } = useAuth();
  const { connected } = useWS();
  const [dropdownOpen, setDropdownOpen] = useState(false);

  return (
    <header className="sticky top-0 z-30 flex h-16 items-center justify-between border-b border-zinc-800 bg-[#0a0a0a]/80 px-4 backdrop-blur-sm lg:px-6">
      <div className="flex items-center gap-4">
        <button
          onClick={onMenuClick}
          className="text-zinc-400 hover:text-zinc-100 lg:hidden"
        >
          <Menu className="h-5 w-5" />
        </button>

        {/* Connection status */}
        <div className="flex items-center gap-2 text-sm">
          <div
            className={cn(
              "h-2.5 w-2.5 rounded-full",
              connected ? "bg-green-500 shadow-[0_0_6px_rgba(34,197,94,0.5)]" : "bg-red-500"
            )}
          />
          <span className="hidden text-zinc-400 sm:inline">
            {connected ? "Live" : "Disconnected"}
          </span>
        </div>
      </div>

      {/* User dropdown */}
      <div className="relative">
        <Button
          variant="ghost"
          size="sm"
          className="flex items-center gap-2 text-zinc-300"
          onClick={() => setDropdownOpen(!dropdownOpen)}
        >
          <div className="flex h-7 w-7 items-center justify-center rounded-full bg-zinc-800 text-xs font-medium text-zinc-300">
            {user?.username?.charAt(0).toUpperCase() || "U"}
          </div>
          <span className="hidden sm:inline">{user?.username || "User"}</span>
          <ChevronDown className="h-4 w-4" />
        </Button>

        {dropdownOpen && (
          <>
            <div
              className="fixed inset-0 z-40"
              onClick={() => setDropdownOpen(false)}
            />
            <div className="absolute right-0 z-50 mt-2 w-48 rounded-lg border border-zinc-800 bg-zinc-900 py-1 shadow-lg">
              <div className="border-b border-zinc-800 px-4 py-2">
                <p className="text-sm font-medium text-zinc-200">
                  {user?.username}
                </p>
                <p className="text-xs text-zinc-500">Administrator</p>
              </div>
              <button
                onClick={() => {
                  setDropdownOpen(false);
                  logout();
                }}
                className="flex w-full items-center gap-2 px-4 py-2 text-sm text-zinc-400 hover:bg-zinc-800 hover:text-zinc-100"
              >
                <LogOut className="h-4 w-4" />
                Sign out
              </button>
            </div>
          </>
        )}
      </div>
    </header>
  );
}

function DashboardShell({ children }: { children: React.ReactNode }) {
  const [sidebarOpen, setSidebarOpen] = useState(false);
  const { isLoading, isAuthenticated } = useAuth();

  if (isLoading) {
    return (
      <div className="flex h-screen items-center justify-center bg-[#0a0a0a]">
        <div className="h-8 w-8 animate-spin rounded-full border-2 border-zinc-700 border-t-blue-500" />
      </div>
    );
  }

  if (!isAuthenticated) {
    return null; // AuthProvider will redirect to login
  }

  return (
    <div className="flex h-screen overflow-hidden bg-[#0a0a0a]">
      <Sidebar open={sidebarOpen} onClose={() => setSidebarOpen(false)} />
      <div className="flex flex-1 flex-col overflow-hidden">
        <TopBar onMenuClick={() => setSidebarOpen(true)} />
        <main className="flex-1 overflow-auto p-4 lg:p-6">{children}</main>
      </div>
    </div>
  );
}

export default function DashboardLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <AuthProvider>
      <WebSocketProvider>
        <DashboardShell>{children}</DashboardShell>
      </WebSocketProvider>
    </AuthProvider>
  );
}
