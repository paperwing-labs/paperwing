import { useCallback, useEffect, useRef, useState } from "react";
import { api } from "./api";
import { AccountDialog } from "./components/AccountDialog";
import { AccountManager } from "./components/AccountManager";
import { APITokenManager } from "./components/APITokenManager";
import { EmailReader } from "./components/EmailReader";
import { MessageList } from "./components/MessageList";
import { Sidebar } from "./components/Sidebar";
import { Toasts } from "./components/Toasts";
import type { Account, Email, EmailPage, EmailSummary, NewAccount, ToastMessage } from "./types";
import { cn } from "./utils";

const EMPTY_PAGE: EmailPage = { items: [], page: 1, page_size: 50, total: 0 };

export default function App({ onLogout }: { onLogout: () => Promise<void> }) {
  const [accounts, setAccounts] = useState<Account[]>([]);
  const [emailPage, setEmailPage] = useState<EmailPage>(EMPTY_PAGE);
  const [selectedAccountID, setSelectedAccountID] = useState<string | null>(null);
  const [selectedEmailID, setSelectedEmailID] = useState<string | null>(null);
  const [selectedEmail, setSelectedEmail] = useState<Email | null>(null);
  const [page, setPage] = useState(1);
  const [search, setSearch] = useState("");
  const [emailsLoading, setEmailsLoading] = useState(true);
  const [emailLoading, setEmailLoading] = useState(false);
  const [emailsError, setEmailsError] = useState<string | null>(null);
  const [emailError, setEmailError] = useState<string | null>(null);
  const [sidebarOpen, setSidebarOpen] = useState(false);
  const [accountDialogOpen, setAccountDialogOpen] = useState(false);
  const [accountManagerOpen, setAccountManagerOpen] = useState(false);
  const [tokenManagerOpen, setTokenManagerOpen] = useState(false);
  const [refreshing, setRefreshing] = useState(false);
  const [toasts, setToasts] = useState<ToastMessage[]>([]);
  const emailRequestID = useRef(0);
  const listRequestID = useRef(0);
  const toastID = useRef(0);

  const notify = useCallback((message: string, tone: ToastMessage["tone"] = "info") => {
    toastID.current += 1;
    const id = toastID.current;
    setToasts((current) => [...current, { id, tone, message }]);
    window.setTimeout(() => {
      setToasts((current) => current.filter((toast) => toast.id !== id));
    }, 4200);
  }, []);

  const loadAccounts = useCallback(async () => {
    try {
      const result = await api.listAccounts();
      setAccounts(result);
      return result;
    } catch (error) {
      setEmailsError(error instanceof Error ? error.message : "无法加载邮箱账号");
      throw error;
    }
  }, []);

  const loadEmails = useCallback(async (accountID: string | null, requestedPage: number) => {
    listRequestID.current += 1;
    const requestID = listRequestID.current;
    setEmailsLoading(true);
    setEmailsError(null);
    try {
      const result = await api.listEmails(accountID, requestedPage, 50);
      if (requestID !== listRequestID.current) return;
      setEmailPage(result);
      setPage(result.page);
    } catch (error) {
      if (requestID !== listRequestID.current) return;
      setEmailsError(error instanceof Error ? error.message : "无法加载邮件");
      setEmailPage((current) => ({ ...current, items: [], total: 0 }));
    } finally {
      if (requestID === listRequestID.current) setEmailsLoading(false);
    }
  }, []);

  useEffect(() => {
    void loadAccounts().catch(() => undefined);
  }, [loadAccounts]);

  useEffect(() => {
    void loadEmails(selectedAccountID, page);
    // Account changes reset page explicitly, so only user pagination needs page here.
  }, [loadEmails, page, selectedAccountID]);

  useEffect(() => {
    const timer = window.setInterval(() => {
      void api.listAccounts().then(setAccounts).catch(() => undefined);
    }, 5_000);
    return () => window.clearInterval(timer);
  }, []);

  const selectAccount = (id: string | null) => {
    setSelectedAccountID(id);
    setPage(1);
    setSearch("");
    setSelectedEmailID(null);
    setSelectedEmail(null);
    setEmailError(null);
    setSidebarOpen(false);
  };

  const loadEmail = useCallback(async (id: string) => {
    emailRequestID.current += 1;
    const requestID = emailRequestID.current;
    setEmailLoading(true);
    setEmailError(null);
    try {
      const result = await api.getEmail(id);
      if (requestID === emailRequestID.current) setSelectedEmail(result);
    } catch (error) {
      if (requestID === emailRequestID.current) {
        setSelectedEmail(null);
        setEmailError(error instanceof Error ? error.message : "无法打开邮件");
      }
    } finally {
      if (requestID === emailRequestID.current) setEmailLoading(false);
    }
  }, []);

  const selectEmail = (summary: EmailSummary) => {
    setSelectedEmailID(summary.id);
    setSelectedEmail(null);
    void loadEmail(summary.id);
  };

  const refresh = async () => {
    setRefreshing(true);
    try {
      await Promise.all([loadAccounts(), loadEmails(selectedAccountID, page)]);
      notify("收件箱已刷新", "success");
    } catch {
      notify("刷新失败，请检查服务状态", "error");
    } finally {
      setRefreshing(false);
    }
  };

  const createAccount = async (account: NewAccount) => {
    const created = await api.createAccount(account);
    setAccounts((current) => [...current, created]);
    notify(`${created.name} 已添加，正在同步邮件`, "success");
    void loadEmails(selectedAccountID, 1);
  };

  const syncAccount = async (account: Account) => {
    await api.syncAccount(account.id);
    notify(`${account.name} 同步完成`, "success");
    await Promise.all([loadAccounts(), loadEmails(selectedAccountID, page)]);
  };

  const deleteAccount = async (account: Account) => {
    await api.deleteAccount(account.id);
    if (selectedAccountID === account.id) selectAccount(null);
    if (selectedEmail?.account_id === account.id) {
      setSelectedEmailID(null);
      setSelectedEmail(null);
    }
    setAccounts((current) => current.filter((item) => item.id !== account.id));
    notify(`${account.name} 及其本地邮件已删除`, "info");
    void loadEmails(selectedAccountID === account.id ? null : selectedAccountID, 1);
  };

  const openAddAccount = () => {
    setSidebarOpen(false);
    setAccountManagerOpen(false);
    setTokenManagerOpen(false);
    setAccountDialogOpen(true);
  };

  const selectedAccount = accounts.find(
    (account) => account.id === (selectedEmail?.account_id || selectedAccountID),
  );

  return (
    <div className="flex h-dvh min-h-[540px] w-full overflow-hidden bg-[#f0efe9]">
      <Sidebar
        accounts={accounts}
        selectedAccountID={selectedAccountID}
        totalEmails={emailPage.total}
        open={sidebarOpen}
        syncing={refreshing}
        onSelectAccount={selectAccount}
        onAddAccount={openAddAccount}
        onManageAccounts={() => {
          setSidebarOpen(false);
          setTokenManagerOpen(false);
          setAccountManagerOpen(true);
        }}
        onManageTokens={() => {
          setSidebarOpen(false);
          setAccountManagerOpen(false);
          setTokenManagerOpen(true);
        }}
        onRefresh={() => void refresh()}
        onLogout={() => void onLogout()}
        onClose={() => setSidebarOpen(false)}
      />

      <main className="flex min-w-0 flex-1">
        <div
          className={cn(
            "h-full w-full md:w-auto",
            selectedEmailID ? "hidden md:block" : "block",
          )}
        >
          <MessageList
            emails={emailPage.items}
            accounts={accounts}
            selectedAccountID={selectedAccountID}
            selectedEmailID={selectedEmailID}
            total={emailPage.total}
            page={page}
            pageSize={emailPage.page_size}
            loading={emailsLoading}
            error={emailsError}
            search={search}
            onSearchChange={setSearch}
            onSelectEmail={selectEmail}
            onPageChange={setPage}
            onOpenMenu={() => setSidebarOpen(true)}
            onRetry={() => void refresh()}
          />
        </div>

        <div
          className={cn(
            "h-full min-w-0 flex-1",
            selectedEmailID ? "block" : "hidden md:block",
          )}
        >
          <EmailReader
            email={selectedEmail}
            account={selectedAccount}
            loading={emailLoading}
            error={emailError}
            attachmentURL={api.attachmentURL}
            onBack={() => {
              emailRequestID.current += 1;
              setSelectedEmailID(null);
              setSelectedEmail(null);
              setEmailLoading(false);
              setEmailError(null);
            }}
            onRetry={() => selectedEmailID && void loadEmail(selectedEmailID)}
          />
        </div>
      </main>

      <AccountDialog
        open={accountDialogOpen}
        onClose={() => setAccountDialogOpen(false)}
        onTest={(account) => api.testAccount(account).then(() => undefined)}
        onCreate={createAccount}
      />
      <AccountManager
        open={accountManagerOpen}
        accounts={accounts}
        onClose={() => setAccountManagerOpen(false)}
        onSync={syncAccount}
        onDelete={deleteAccount}
        onAdd={openAddAccount}
      />
      <APITokenManager open={tokenManagerOpen} onClose={() => setTokenManagerOpen(false)} />
      <Toasts
        toasts={toasts}
        onDismiss={(id) => setToasts((current) => current.filter((toast) => toast.id !== id))}
      />
    </div>
  );
}
