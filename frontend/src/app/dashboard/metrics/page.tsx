"use client";

import { useState } from "react";
import useSWR from "swr";
import { swrFetcher } from "@/lib/api";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/utils";
import type { SystemMetrics } from "@/lib/types";
import { Cpu, MemoryStick, HardDrive, Activity } from "lucide-react";
import {
  AreaChart,
  Area,
  LineChart,
  Line,
  XAxis,
  YAxis,
  Tooltip,
  ResponsiveContainer,
  CartesianGrid,
} from "recharts";
import { format, parseISO } from "date-fns";

// -------------------------------------------------------------------
// Time range options
// -------------------------------------------------------------------
const RANGES = [
  { label: "1h", value: "1h" },
  { label: "6h", value: "6h" },
  { label: "24h", value: "24h" },
  { label: "7d", value: "7d" },
  { label: "30d", value: "30d" },
];

// -------------------------------------------------------------------
// Format X axis ticks based on range
// -------------------------------------------------------------------
function formatXAxis(value: string, range: string): string {
  try {
    const date = parseISO(value);
    switch (range) {
      case "1h":
      case "6h":
        return format(date, "HH:mm");
      case "24h":
        return format(date, "HH:mm");
      case "7d":
        return format(date, "EEE HH:mm");
      case "30d":
        return format(date, "MMM d");
      default:
        return format(date, "HH:mm");
    }
  } catch {
    return value;
  }
}

// -------------------------------------------------------------------
// Custom tooltip
// -------------------------------------------------------------------
function ChartTooltip({
  active,
  payload,
  label,
}: {
  active?: boolean;
  payload?: Array<{ name: string; value: number; color: string }>;
  label?: string;
}) {
  if (!active || !payload || !payload.length) return null;

  let formattedLabel = label || "";
  try {
    formattedLabel = format(parseISO(label || ""), "MMM d, HH:mm:ss");
  } catch {
    // keep original
  }

  return (
    <div className="rounded-lg border border-zinc-700 bg-zinc-900 px-3 py-2 shadow-lg">
      <p className="mb-1 text-xs text-zinc-400">{formattedLabel}</p>
      {payload.map((entry, i) => (
        <p key={i} className="text-sm" style={{ color: entry.color }}>
          {entry.name}: {entry.value.toFixed(1)}
          {entry.name.includes("Load") ? "" : "%"}
        </p>
      ))}
    </div>
  );
}

// -------------------------------------------------------------------
// Metrics page
// -------------------------------------------------------------------
export default function MetricsPage() {
  const [range, setRange] = useState("24h");

  const { data: metricsResp, isLoading } = useSWR<{ data: SystemMetrics[]; points: number }>(
    `/metrics/history?range=${range}`,
    swrFetcher,
    { refreshInterval: range === "1h" ? 10000 : 60000 }
  );

  const data = metricsResp?.data ?? [];

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <h1 className="text-2xl font-bold text-zinc-100">Metrics</h1>

        {/* Time range selector */}
        <div className="flex gap-1 rounded-lg border border-zinc-800 bg-zinc-900/50 p-1">
          {RANGES.map((r) => (
            <Button
              key={r.value}
              variant={range === r.value ? "secondary" : "ghost"}
              size="sm"
              onClick={() => setRange(r.value)}
              className="px-3"
            >
              {r.label}
            </Button>
          ))}
        </div>
      </div>

      {isLoading && data.length === 0 ? (
        <div className="space-y-4">
          {Array.from({ length: 4 }).map((_, i) => (
            <Skeleton key={i} className="h-72 rounded-xl" />
          ))}
        </div>
      ) : (
        <div className="space-y-4">
          {/* CPU chart */}
          <Card>
            <CardHeader className="pb-2">
              <CardTitle className="flex items-center gap-2 text-base">
                <Cpu className="h-4 w-4 text-blue-400" />
                CPU Usage
              </CardTitle>
            </CardHeader>
            <CardContent>
              <div className="h-64">
                <ResponsiveContainer width="100%" height="100%">
                  <AreaChart data={data}>
                    <CartesianGrid strokeDasharray="3 3" stroke="#27272a" />
                    <XAxis
                      dataKey="timestamp"
                      tick={{ fill: "#71717a", fontSize: 11 }}
                      tickFormatter={(v) => formatXAxis(v, range)}
                      minTickGap={50}
                    />
                    <YAxis
                      domain={[0, 100]}
                      tick={{ fill: "#71717a", fontSize: 11 }}
                      tickFormatter={(v) => `${v}%`}
                    />
                    <Tooltip content={<ChartTooltip />} />
                    <Area
                      type="monotone"
                      dataKey="cpu_percent"
                      name="CPU"
                      stroke="#3b82f6"
                      fill="#3b82f6"
                      fillOpacity={0.15}
                      strokeWidth={2}
                    />
                  </AreaChart>
                </ResponsiveContainer>
              </div>
            </CardContent>
          </Card>

          {/* RAM + Disk overlay */}
          <Card>
            <CardHeader className="pb-2">
              <CardTitle className="flex items-center gap-2 text-base">
                <MemoryStick className="h-4 w-4 text-purple-400" />
                RAM &
                <HardDrive className="h-4 w-4 text-amber-400" />
                Disk Usage
              </CardTitle>
            </CardHeader>
            <CardContent>
              <div className="h-64">
                <ResponsiveContainer width="100%" height="100%">
                  <LineChart data={data}>
                    <CartesianGrid strokeDasharray="3 3" stroke="#27272a" />
                    <XAxis
                      dataKey="timestamp"
                      tick={{ fill: "#71717a", fontSize: 11 }}
                      tickFormatter={(v) => formatXAxis(v, range)}
                      minTickGap={50}
                    />
                    <YAxis
                      domain={[0, 100]}
                      tick={{ fill: "#71717a", fontSize: 11 }}
                      tickFormatter={(v) => `${v}%`}
                    />
                    <Tooltip content={<ChartTooltip />} />
                    <Line
                      type="monotone"
                      dataKey="ram_percent"
                      name="RAM"
                      stroke="#a855f7"
                      strokeWidth={2}
                      dot={false}
                    />
                    <Line
                      type="monotone"
                      dataKey="disk_percent"
                      name="Disk"
                      stroke="#f59e0b"
                      strokeWidth={2}
                      dot={false}
                    />
                  </LineChart>
                </ResponsiveContainer>
              </div>
            </CardContent>
          </Card>

          {/* Load Average chart */}
          <Card>
            <CardHeader className="pb-2">
              <CardTitle className="flex items-center gap-2 text-base">
                <Activity className="h-4 w-4 text-green-400" />
                Load Average
              </CardTitle>
            </CardHeader>
            <CardContent>
              <div className="h-64">
                <ResponsiveContainer width="100%" height="100%">
                  <LineChart data={data}>
                    <CartesianGrid strokeDasharray="3 3" stroke="#27272a" />
                    <XAxis
                      dataKey="timestamp"
                      tick={{ fill: "#71717a", fontSize: 11 }}
                      tickFormatter={(v) => formatXAxis(v, range)}
                      minTickGap={50}
                    />
                    <YAxis
                      tick={{ fill: "#71717a", fontSize: 11 }}
                      tickFormatter={(v) => v.toFixed(1)}
                    />
                    <Tooltip content={<ChartTooltip />} />
                    <Line
                      type="monotone"
                      dataKey="load_1m"
                      name="Load 1m"
                      stroke="#ef4444"
                      strokeWidth={2}
                      dot={false}
                    />
                    <Line
                      type="monotone"
                      dataKey="load_5m"
                      name="Load 5m"
                      stroke="#f59e0b"
                      strokeWidth={2}
                      dot={false}
                    />
                    <Line
                      type="monotone"
                      dataKey="load_15m"
                      name="Load 15m"
                      stroke="#22c55e"
                      strokeWidth={2}
                      dot={false}
                    />
                  </LineChart>
                </ResponsiveContainer>
              </div>
            </CardContent>
          </Card>

          {data.length === 0 && (
            <div className="flex h-48 items-center justify-center rounded-xl border border-zinc-800 text-zinc-500">
              No metric data available for this range
            </div>
          )}
        </div>
      )}
    </div>
  );
}
