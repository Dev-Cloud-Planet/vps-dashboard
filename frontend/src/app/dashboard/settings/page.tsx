"use client";

import { useState, useEffect } from "react";
import useSWR, { mutate } from "swr";
import { swrFetcher, put } from "@/lib/api";
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";
import {
  Settings,
  Bell,
  Shield,
  Save,
  Loader2,
  CheckCircle,
  AlertTriangle,
  Key,
} from "lucide-react";

// -------------------------------------------------------------------
// Settings types (local to this page, flexible shape)
// -------------------------------------------------------------------
interface SettingsData {
  cpu_threshold: number;
  ram_threshold: number;
  disk_threshold: number;
  check_interval: number;
  alerts_enabled: boolean;
  rate_limit_seconds: number;
  webhook_url: string;
  [key: string]: unknown;
}

// -------------------------------------------------------------------
// Toggle switch component
// -------------------------------------------------------------------
function Toggle({
  checked,
  onChange,
}: {
  checked: boolean;
  onChange: (v: boolean) => void;
}) {
  return (
    <button
      type="button"
      onClick={() => onChange(!checked)}
      className={cn(
        "relative inline-flex h-6 w-11 shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors",
        checked ? "bg-blue-600" : "bg-zinc-700"
      )}
    >
      <span
        className={cn(
          "pointer-events-none inline-block h-5 w-5 rounded-full bg-white shadow-lg transition-transform",
          checked ? "translate-x-5" : "translate-x-0"
        )}
      />
    </button>
  );
}

// -------------------------------------------------------------------
// Save status indicator
// -------------------------------------------------------------------
function SaveStatus({ status }: { status: "idle" | "saving" | "saved" | "error" }) {
  if (status === "idle") return null;
  return (
    <span className="flex items-center gap-1 text-sm">
      {status === "saving" && (
        <>
          <Loader2 className="h-4 w-4 animate-spin text-zinc-400" />
          <span className="text-zinc-400">Saving...</span>
        </>
      )}
      {status === "saved" && (
        <>
          <CheckCircle className="h-4 w-4 text-green-400" />
          <span className="text-green-400">Saved</span>
        </>
      )}
      {status === "error" && (
        <>
          <AlertTriangle className="h-4 w-4 text-red-400" />
          <span className="text-red-400">Failed to save</span>
        </>
      )}
    </span>
  );
}

// -------------------------------------------------------------------
// Settings page
// -------------------------------------------------------------------
export default function SettingsPage() {
  const { data: settings, isLoading } = useSWR<SettingsData>(
    "/settings",
    swrFetcher
  );

  // Monitoring thresholds
  const [cpuThreshold, setCpuThreshold] = useState(90);
  const [ramThreshold, setRamThreshold] = useState(90);
  const [diskThreshold, setDiskThreshold] = useState(90);
  const [monitorStatus, setMonitorStatus] = useState<"idle" | "saving" | "saved" | "error">("idle");

  // Alert settings
  const [alertsEnabled, setAlertsEnabled] = useState(true);
  const [rateLimitSeconds, setRateLimitSeconds] = useState(300);
  const [webhookUrl, setWebhookUrl] = useState("");
  const [alertStatus, setAlertStatus] = useState<"idle" | "saving" | "saved" | "error">("idle");

  // Password change
  const [currentPassword, setCurrentPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [passwordStatus, setPasswordStatus] = useState<"idle" | "saving" | "saved" | "error">("idle");
  const [passwordError, setPasswordError] = useState("");

  // Populate form when settings load
  useEffect(() => {
    if (settings) {
      setCpuThreshold(settings.cpu_threshold ?? 90);
      setRamThreshold(settings.ram_threshold ?? 90);
      setDiskThreshold(settings.disk_threshold ?? 90);
      setAlertsEnabled(settings.alerts_enabled ?? true);
      setRateLimitSeconds(settings.rate_limit_seconds ?? 300);
      setWebhookUrl(settings.webhook_url ?? "");
    }
  }, [settings]);

  // Save monitoring thresholds
  const saveMonitoring = async () => {
    setMonitorStatus("saving");
    try {
      await put("/settings", {
        ...settings,
        cpu_threshold: cpuThreshold,
        ram_threshold: ramThreshold,
        disk_threshold: diskThreshold,
      });
      mutate("/settings");
      setMonitorStatus("saved");
      setTimeout(() => setMonitorStatus("idle"), 2000);
    } catch {
      setMonitorStatus("error");
      setTimeout(() => setMonitorStatus("idle"), 3000);
    }
  };

  // Save alert settings
  const saveAlerts = async () => {
    setAlertStatus("saving");
    try {
      await put("/settings", {
        ...settings,
        alerts_enabled: alertsEnabled,
        rate_limit_seconds: rateLimitSeconds,
        webhook_url: webhookUrl,
      });
      mutate("/settings");
      setAlertStatus("saved");
      setTimeout(() => setAlertStatus("idle"), 2000);
    } catch {
      setAlertStatus("error");
      setTimeout(() => setAlertStatus("idle"), 3000);
    }
  };

  // Change password
  const changePassword = async () => {
    setPasswordError("");

    if (newPassword !== confirmPassword) {
      setPasswordError("Passwords do not match");
      return;
    }

    if (newPassword.length < 8) {
      setPasswordError("Password must be at least 8 characters");
      return;
    }

    setPasswordStatus("saving");
    try {
      await put("/auth/password", {
        current_password: currentPassword,
        new_password: newPassword,
      });
      setPasswordStatus("saved");
      setCurrentPassword("");
      setNewPassword("");
      setConfirmPassword("");
      setTimeout(() => setPasswordStatus("idle"), 2000);
    } catch (err) {
      setPasswordStatus("error");
      setPasswordError(err instanceof Error ? err.message : "Failed to change password");
      setTimeout(() => setPasswordStatus("idle"), 3000);
    }
  };

  if (isLoading) {
    return (
      <div className="space-y-6">
        <h1 className="text-2xl font-bold">Settings</h1>
        <div className="space-y-4">
          {Array.from({ length: 3 }).map((_, i) => (
            <div key={i} className="h-48 animate-pulse rounded-xl bg-zinc-800" />
          ))}
        </div>
      </div>
    );
  }

  return (
    <div className="mx-auto max-w-3xl space-y-6">
      <h1 className="text-2xl font-bold text-zinc-100">Settings</h1>

      {/* Monitoring thresholds */}
      <Card>
        <CardHeader>
          <div className="flex items-center gap-2">
            <Settings className="h-5 w-5 text-blue-400" />
            <CardTitle className="text-base">Monitoring Thresholds</CardTitle>
          </div>
          <CardDescription>
            Alert when resource usage exceeds these percentages
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
            <div className="space-y-2">
              <label className="text-sm font-medium text-zinc-300">
                CPU Threshold (%)
              </label>
              <Input
                type="number"
                min={0}
                max={100}
                value={cpuThreshold}
                onChange={(e) => setCpuThreshold(Number(e.target.value))}
              />
            </div>
            <div className="space-y-2">
              <label className="text-sm font-medium text-zinc-300">
                RAM Threshold (%)
              </label>
              <Input
                type="number"
                min={0}
                max={100}
                value={ramThreshold}
                onChange={(e) => setRamThreshold(Number(e.target.value))}
              />
            </div>
            <div className="space-y-2">
              <label className="text-sm font-medium text-zinc-300">
                Disk Threshold (%)
              </label>
              <Input
                type="number"
                min={0}
                max={100}
                value={diskThreshold}
                onChange={(e) => setDiskThreshold(Number(e.target.value))}
              />
            </div>
          </div>

          <div className="flex items-center justify-between border-t border-zinc-800 pt-4">
            <SaveStatus status={monitorStatus} />
            <Button onClick={saveMonitoring} disabled={monitorStatus === "saving"}>
              <Save className="h-4 w-4" />
              Save Thresholds
            </Button>
          </div>
        </CardContent>
      </Card>

      {/* Alert settings */}
      <Card>
        <CardHeader>
          <div className="flex items-center gap-2">
            <Bell className="h-5 w-5 text-green-400" />
            <CardTitle className="text-base">Alert Settings</CardTitle>
          </div>
          <CardDescription>
            Configure how and when alerts are sent
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm font-medium text-zinc-300">
                Alerts Enabled
              </p>
              <p className="text-xs text-zinc-500">
                Send webhook alerts when thresholds are exceeded
              </p>
            </div>
            <Toggle checked={alertsEnabled} onChange={setAlertsEnabled} />
          </div>

          <div className="space-y-2">
            <label className="text-sm font-medium text-zinc-300">
              Rate Limit (seconds)
            </label>
            <Input
              type="number"
              min={0}
              value={rateLimitSeconds}
              onChange={(e) => setRateLimitSeconds(Number(e.target.value))}
            />
            <p className="text-xs text-zinc-500">
              Minimum seconds between repeated alerts for the same key
            </p>
          </div>

          <div className="space-y-2">
            <label className="text-sm font-medium text-zinc-300">
              Webhook URL
            </label>
            <Input
              type="url"
              value={webhookUrl}
              onChange={(e) => setWebhookUrl(e.target.value)}
              placeholder="https://discord.com/api/webhooks/..."
            />
          </div>

          <div className="flex items-center justify-between border-t border-zinc-800 pt-4">
            <SaveStatus status={alertStatus} />
            <Button onClick={saveAlerts} disabled={alertStatus === "saving"}>
              <Save className="h-4 w-4" />
              Save Alert Settings
            </Button>
          </div>
        </CardContent>
      </Card>

      {/* Change password */}
      <Card>
        <CardHeader>
          <div className="flex items-center gap-2">
            <Key className="h-5 w-5 text-amber-400" />
            <CardTitle className="text-base">Change Password</CardTitle>
          </div>
          <CardDescription>
            Update your account password
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          {passwordError && (
            <div className="rounded-md bg-red-900/30 border border-red-800 px-4 py-3 text-sm text-red-400">
              {passwordError}
            </div>
          )}

          <div className="space-y-2">
            <label className="text-sm font-medium text-zinc-300">
              Current Password
            </label>
            <Input
              type="password"
              value={currentPassword}
              onChange={(e) => setCurrentPassword(e.target.value)}
              placeholder="Enter current password"
              autoComplete="current-password"
            />
          </div>

          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
            <div className="space-y-2">
              <label className="text-sm font-medium text-zinc-300">
                New Password
              </label>
              <Input
                type="password"
                value={newPassword}
                onChange={(e) => setNewPassword(e.target.value)}
                placeholder="Enter new password"
                autoComplete="new-password"
              />
            </div>
            <div className="space-y-2">
              <label className="text-sm font-medium text-zinc-300">
                Confirm Password
              </label>
              <Input
                type="password"
                value={confirmPassword}
                onChange={(e) => setConfirmPassword(e.target.value)}
                placeholder="Confirm new password"
                autoComplete="new-password"
              />
            </div>
          </div>

          <div className="flex items-center justify-between border-t border-zinc-800 pt-4">
            <SaveStatus status={passwordStatus} />
            <Button
              onClick={changePassword}
              disabled={
                passwordStatus === "saving" ||
                !currentPassword ||
                !newPassword ||
                !confirmPassword
              }
            >
              <Shield className="h-4 w-4" />
              Change Password
            </Button>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
