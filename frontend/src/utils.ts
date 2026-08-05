const AVATAR_COLORS = [
  "#df6656",
  "#617ec4",
  "#4f9276",
  "#a66fbd",
  "#d18b45",
  "#568ea3",
];

export function cn(...classes: Array<string | false | null | undefined>) {
  return classes.filter(Boolean).join(" ");
}

export function displayName(address?: string) {
  if (!address) return "未知发件人";
  const match = address.match(/^\s*"?([^"<]+?)"?\s*<[^>]+>\s*$/);
  if (match?.[1]) return match[1].trim();
  const email = address.match(/<([^>]+)>/)?.[1] || address;
  return email.split("@")[0] || address;
}

export function emailAddress(address?: string) {
  if (!address) return "";
  return address.match(/<([^>]+)>/)?.[1] || address;
}

export function initials(value?: string) {
  const name = displayName(value).trim();
  const parts = name.split(/[\s_-]+/).filter(Boolean);
  if (parts.length > 1) return (parts[0][0] + parts[1][0]).toUpperCase();
  return name.slice(0, 2).toUpperCase();
}

export function avatarColor(value: string) {
  let hash = 0;
  for (let index = 0; index < value.length; index += 1) {
    hash = value.charCodeAt(index) + ((hash << 5) - hash);
  }
  return AVATAR_COLORS[Math.abs(hash) % AVATAR_COLORS.length];
}

export function formatRelativeDate(value: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "";
  const now = new Date();
  const sameDay = date.toDateString() === now.toDateString();
  if (sameDay) {
    return new Intl.DateTimeFormat("zh-CN", {
      hour: "2-digit",
      minute: "2-digit",
      hour12: false,
    }).format(date);
  }
  const sameYear = date.getFullYear() === now.getFullYear();
  return new Intl.DateTimeFormat("zh-CN", {
    ...(sameYear ? { month: "short", day: "numeric" } : { year: "numeric", month: "short", day: "numeric" }),
  }).format(date);
}

export function formatFullDate(value: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "";
  return new Intl.DateTimeFormat("zh-CN", {
    year: "numeric",
    month: "long",
    day: "numeric",
    weekday: "short",
    hour: "2-digit",
    minute: "2-digit",
    hour12: false,
  }).format(date);
}

export function formatBytes(bytes: number) {
  if (!Number.isFinite(bytes) || bytes <= 0) return "0 B";
  const units = ["B", "KB", "MB", "GB"];
  const unit = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1);
  const value = bytes / 1024 ** unit;
  return `${value >= 10 || unit === 0 ? value.toFixed(0) : value.toFixed(1)} ${units[unit]}`;
}

export function statusLabel(status: string) {
  const labels: Record<string, string> = {
    starting: "启动中",
    connecting: "连接中",
    syncing: "同步中",
    idle: "实时同步",
    reconnecting: "重新连接",
  };
  return labels[status] || status;
}
