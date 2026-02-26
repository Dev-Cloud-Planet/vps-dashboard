import { clsx, type ClassValue } from "clsx";
import { twMerge } from "tailwind-merge";
import { format, formatDistanceToNow, parseISO } from "date-fns";

/**
 * Merge Tailwind CSS classes with clsx and tailwind-merge.
 */
export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

/**
 * Format bytes into human-readable string (KB, MB, GB, TB).
 */
export function formatBytes(bytes: number, decimals = 1): string {
  if (bytes === 0) return "0 B";

  const k = 1024;
  const sizes = ["B", "KB", "MB", "GB", "TB", "PB"];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  const value = bytes / Math.pow(k, i);

  return `${value.toFixed(decimals)} ${sizes[i]}`;
}

/**
 * Format uptime seconds into human-readable string.
 * e.g. 90061 => "1d 1h 1m"
 */
export function formatUptime(seconds: number): string {
  if (seconds < 0) return "0s";

  const days = Math.floor(seconds / 86400);
  const hours = Math.floor((seconds % 86400) / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);

  const parts: string[] = [];
  if (days > 0) parts.push(`${days}d`);
  if (hours > 0) parts.push(`${hours}h`);
  if (minutes > 0) parts.push(`${minutes}m`);

  return parts.length > 0 ? parts.join(" ") : "< 1m";
}

/**
 * Format an ISO date string into a readable format.
 * e.g. "2024-01-15T10:30:00Z" => "Jan 15, 2024 10:30 AM"
 */
export function formatDate(
  dateString: string,
  formatStr = "MMM d, yyyy h:mm a"
): string {
  try {
    const date = parseISO(dateString);
    return format(date, formatStr);
  } catch {
    return dateString;
  }
}

/**
 * Format an ISO date string into a relative time string.
 * e.g. "2024-01-15T10:30:00Z" => "5 minutes ago"
 */
export function formatRelativeTime(dateString: string): string {
  try {
    const date = parseISO(dateString);
    return formatDistanceToNow(date, { addSuffix: true });
  } catch {
    return dateString;
  }
}

/**
 * Clamp a number between min and max.
 */
export function clamp(value: number, min: number, max: number): number {
  return Math.min(Math.max(value, min), max);
}

/**
 * Get a color class based on a percentage value (for gauges).
 */
export function getPercentColor(percent: number): string {
  if (percent >= 90) return "text-red-500";
  if (percent >= 75) return "text-orange-500";
  if (percent >= 50) return "text-yellow-500";
  return "text-green-500";
}

/**
 * Get a background color class based on a percentage value.
 */
export function getPercentBgColor(percent: number): string {
  if (percent >= 90) return "bg-red-500";
  if (percent >= 75) return "bg-orange-500";
  if (percent >= 50) return "bg-yellow-500";
  return "bg-green-500";
}
