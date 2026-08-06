export type MonitorStatus =
  | "starting"
  | "connecting"
  | "syncing"
  | "idle"
  | "reconnecting";

export interface Account {
  id: string;
  name: string;
  host: string;
  port: number;
  tls: boolean;
  username: string;
  monitor_status: MonitorStatus;
  latest_connection_error?: string;
  last_successful_sync_at?: string;
  created_at: string;
}

export interface NewAccount {
  name: string;
  host: string;
  port: number;
  tls: boolean;
  username: string;
  password: string;
}

export interface EmailSummary {
  id: string;
  account_id: string;
  message_id?: string;
  subject: string;
  from: string[];
  to: string[];
  sent_at?: string;
  received_at: string;
  size: number;
  attachment_count: number;
}

export interface Attachment {
  id: string;
  email_id?: string;
  filename: string;
  content_type: string;
  content_id?: string;
  size: number;
}

export interface Email extends Omit<EmailSummary, "attachment_count"> {
  cc: string[];
  headers: Record<string, string[]>;
  text_body: string;
  html_body: string;
  attachments: Attachment[];
  created_at: string;
}

export interface EmailPage {
  items: EmailSummary[];
  page: number;
  page_size: number;
  total: number;
}

export interface ToastMessage {
  id: number;
  tone: "success" | "error" | "info";
  message: string;
}

export interface AuthStatus {
  configured: boolean;
  authenticated: boolean;
  username: string;
}

export type APITokenScope =
  | "mail:read"
  | "accounts:read"
  | "accounts:write"
  | "sync:write";

export interface APIToken {
  id: string;
  name: string;
  token_prefix: string;
  scopes: APITokenScope[];
  created_at: string;
  last_used_at?: string;
  expires_at?: string;
}

export interface IssuedAPIToken extends APIToken {
  token: string;
}
