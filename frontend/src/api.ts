import type { Account, AuthStatus, Email, EmailPage, NewAccount } from "./types";

interface APIErrorBody {
  error?: { message?: string };
}

export class APIError extends Error {
  status: number;

  constructor(message: string, status: number) {
    super(message);
    this.name = "APIError";
    this.status = status;
  }
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  let response: Response;
  try {
    response = await fetch(path, {
      ...init,
      headers: {
        ...(init?.body ? { "Content-Type": "application/json" } : {}),
        ...init?.headers,
      },
    });
  } catch {
    throw new APIError("无法连接 Paperwing 服务，请确认后端已经启动", 0);
  }

  if (!response.ok) {
    let message = `请求失败（${response.status}）`;
    try {
      const body = (await response.json()) as APIErrorBody;
      message = body.error?.message || message;
    } catch {
      // Keep the useful status-based fallback for non-JSON responses.
    }
    if (response.status === 401 && !path.startsWith("/auth/")) {
      window.dispatchEvent(new CustomEvent("paperwing:unauthorized"));
    }
    throw new APIError(message, response.status);
  }

  if (response.status === 204) return undefined as T;
  return (await response.json()) as T;
}

export const api = {
  authStatus: () => request<AuthStatus>("/auth/status"),

  setupAuth: (username: string, password: string) =>
    request<AuthStatus>("/auth/setup", {
      method: "POST",
      body: JSON.stringify({ username, password }),
    }),

  login: (username: string, password: string) =>
    request<AuthStatus>("/auth/login", {
      method: "POST",
      body: JSON.stringify({ username, password }),
    }),

  logout: () => request<{ ok: true }>("/auth/logout", { method: "POST" }),

  listAccounts: async () => {
    const result = await request<{ items: Account[] }>("/accounts");
    return result.items;
  },

  testAccount: (account: NewAccount) =>
    request<{ ok: true }>("/accounts/test", {
      method: "POST",
      body: JSON.stringify(account),
    }),

  createAccount: (account: NewAccount) =>
    request<Account>("/accounts", {
      method: "POST",
      body: JSON.stringify(account),
    }),

  deleteAccount: (id: string) =>
    request<void>(`/accounts/${encodeURIComponent(id)}`, { method: "DELETE" }),

  syncAccount: (id: string) =>
    request<{ ok: true }>(`/accounts/${encodeURIComponent(id)}/sync`, {
      method: "POST",
    }),

  listEmails: (accountID: string | null, page = 1, pageSize = 50) => {
    const params = new URLSearchParams({
      page: String(page),
      page_size: String(pageSize),
    });
    if (accountID) params.set("account_id", accountID);
    return request<EmailPage>(`/emails?${params.toString()}`);
  },

  getEmail: (id: string) => request<Email>(`/emails/${encodeURIComponent(id)}`),

  attachmentURL: (emailID: string, attachmentID: string) =>
    `/emails/${encodeURIComponent(emailID)}/attachments/${encodeURIComponent(attachmentID)}`,
};
