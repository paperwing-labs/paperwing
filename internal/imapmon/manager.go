package imapmon

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
	"github.com/paperwing/paperwing/internal/domain"
	"github.com/paperwing/paperwing/internal/mailparse"
	"github.com/paperwing/paperwing/internal/store"
)

const fetchBatchSize = 25

type Manager struct {
	ctx            context.Context
	store          *store.Store
	attachmentRoot string
	logger         *slog.Logger

	mu      sync.Mutex
	workers map[string]*worker
	wg      sync.WaitGroup
}

type worker struct {
	cancel  context.CancelFunc
	syncReq chan chan error
	done    chan struct{}
}

func NewManager(ctx context.Context, store *store.Store, attachmentRoot string, logger *slog.Logger) *Manager {
	return &Manager{ctx: ctx, store: store, attachmentRoot: attachmentRoot, logger: logger, workers: make(map[string]*worker)}
}

func (m *Manager) Start(ctx context.Context) error {
	accounts, err := m.store.ListAccounts(ctx)
	if err != nil {
		return err
	}
	for _, account := range accounts {
		m.StartAccount(account.ID)
	}
	return nil
}

func (m *Manager) StartAccount(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.workers[id]; ok {
		return
	}
	ctx, cancel := context.WithCancel(m.ctx)
	w := &worker{cancel: cancel, syncReq: make(chan chan error), done: make(chan struct{})}
	m.workers[id] = w
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		defer close(w.done)
		defer func() {
			m.mu.Lock()
			delete(m.workers, id)
			m.mu.Unlock()
		}()
		m.run(ctx, id, w.syncReq)
	}()
}

func (m *Manager) StopAccount(id string) {
	m.mu.Lock()
	w := m.workers[id]
	m.mu.Unlock()
	if w != nil {
		w.cancel()
		<-w.done
	}
}

func (m *Manager) Sync(ctx context.Context, id string) error {
	m.mu.Lock()
	w := m.workers[id]
	m.mu.Unlock()
	if w == nil {
		return fmt.Errorf("mailbox monitor is not running")
	}
	result := make(chan error, 1)
	select {
	case w.syncReq <- result:
	case <-ctx.Done():
		return ctx.Err()
	}
	select {
	case err := <-result:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (m *Manager) Close() {
	m.mu.Lock()
	for _, w := range m.workers {
		w.cancel()
	}
	m.mu.Unlock()
	m.wg.Wait()
}

func TestConnection(ctx context.Context, account domain.AccountSecret) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	client, err := connect(account, nil)
	if err != nil {
		return err
	}
	defer client.Close()
	done := closeOnCancel(ctx, client)
	defer close(done)
	if err := client.Login(account.Username, account.Password).Wait(); err != nil {
		return fmt.Errorf("IMAP login: %w", err)
	}
	if !client.Caps().Has(imap.CapIdle) {
		return fmt.Errorf("IMAP server does not advertise IDLE support")
	}
	if _, err := client.Select("INBOX", &imap.SelectOptions{ReadOnly: true}).Wait(); err != nil {
		return fmt.Errorf("select INBOX: %w", err)
	}
	_ = client.Logout().Wait()
	return nil
}

func (m *Manager) run(ctx context.Context, id string, syncReq <-chan chan error) {
	backoff := time.Second
	for ctx.Err() == nil {
		account, err := m.store.AccountSecret(ctx, id)
		if err != nil {
			if !errors.Is(err, domain.ErrNotFound) && ctx.Err() == nil {
				m.logger.Error("load mailbox", "account_id", id, "error", err)
			}
			return
		}
		_ = m.store.UpdateMonitorState(ctx, id, "connecting", "")
		connectedAt := time.Now()
		err = m.connected(ctx, account, syncReq)
		if ctx.Err() != nil {
			return
		}
		message := err.Error()
		if time.Since(connectedAt) >= 5*time.Minute {
			backoff = time.Second
		}
		m.logger.Warn("mailbox disconnected", "account_id", id, "error", message, "retry_in", backoff)
		_ = m.store.UpdateMonitorState(context.Background(), id, "reconnecting", message)
		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case result := <-syncReq:
			result <- fmt.Errorf("mailbox is reconnecting: %w", err)
			if !timer.Stop() {
				<-timer.C
			}
		case <-timer.C:
		}
		if backoff < time.Minute {
			backoff *= 2
			if backoff > time.Minute {
				backoff = time.Minute
			}
		}
	}
}

func (m *Manager) connected(ctx context.Context, account domain.AccountSecret, syncReq <-chan chan error) error {
	notify := make(chan struct{}, 1)
	handler := &imapclient.UnilateralDataHandler{Mailbox: func(data *imapclient.UnilateralDataMailbox) {
		if data.NumMessages != nil {
			select {
			case notify <- struct{}{}:
			default:
			}
		}
	}}
	client, err := connect(account, handler)
	if err != nil {
		return err
	}
	defer client.Close()
	done := closeOnCancel(ctx, client)
	defer close(done)
	if err := client.Login(account.Username, account.Password).Wait(); err != nil {
		return fmt.Errorf("IMAP login: %w", err)
	}
	if !client.Caps().Has(imap.CapIdle) {
		return fmt.Errorf("IMAP server does not advertise IDLE support")
	}
	selected, err := client.Select("INBOX", &imap.SelectOptions{ReadOnly: true}).Wait()
	if err != nil {
		return fmt.Errorf("select INBOX: %w", err)
	}
	_ = m.store.UpdateMonitorState(ctx, account.ID, "syncing", "")
	if err := m.sync(client, &account, selected); err != nil {
		return fmt.Errorf("catch-up sync: %w", err)
	}
	_ = m.store.UpdateMonitorState(ctx, account.ID, "idle", "")
	for {
		idle, err := client.Idle()
		if err != nil {
			return fmt.Errorf("start IDLE: %w", err)
		}
		idleDone := make(chan error, 1)
		go func() { idleDone <- idle.Wait() }()

		var response chan error
		select {
		case <-ctx.Done():
			_ = idle.Close()
			<-idleDone
			_ = client.Logout().Wait()
			return ctx.Err()
		case err := <-idleDone:
			return fmt.Errorf("IDLE ended: %w", err)
		case <-notify:
		case response = <-syncReq:
		}
		if err := idle.Close(); err != nil {
			if response != nil {
				response <- err
			}
			return fmt.Errorf("stop IDLE: %w", err)
		}
		if err := <-idleDone; err != nil {
			if response != nil {
				response <- err
			}
			return fmt.Errorf("finish IDLE: %w", err)
		}
		_ = m.store.UpdateMonitorState(ctx, account.ID, "syncing", "")
		err = m.sync(client, &account, selected)
		if response != nil {
			response <- err
		}
		if err != nil {
			return fmt.Errorf("incremental sync: %w", err)
		}
		_ = m.store.UpdateMonitorState(ctx, account.ID, "idle", "")
	}
}

func closeOnCancel(ctx context.Context, client *imapclient.Client) chan struct{} {
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = client.Close()
		case <-done:
		}
	}()
	return done
}

func connect(account domain.AccountSecret, handler *imapclient.UnilateralDataHandler) (*imapclient.Client, error) {
	address := net.JoinHostPort(account.Host, strconv.Itoa(account.Port))
	options := &imapclient.Options{
		TLSConfig:             &tls.Config{ServerName: account.Host, MinVersion: tls.VersionTLS12},
		UnilateralDataHandler: handler,
		Dialer:                &net.Dialer{Timeout: 20 * time.Second, KeepAlive: 30 * time.Second},
	}
	var client *imapclient.Client
	var err error
	if account.TLS {
		client, err = imapclient.DialTLS(address, options)
	} else {
		client, err = imapclient.DialInsecure(address, options)
	}
	if err != nil {
		return nil, fmt.Errorf("connect to %s: %w", address, err)
	}
	return client, nil
}

func (m *Manager) sync(client *imapclient.Client, account *domain.AccountSecret, selected *imap.SelectData) error {
	if selected.UIDValidity == 0 {
		return fmt.Errorf("server did not return UIDVALIDITY")
	}
	if account.UIDValidity != selected.UIDValidity {
		if err := m.store.ResetUIDValidity(context.Background(), account.ID, selected.UIDValidity); err != nil {
			return err
		}
		if err := os.RemoveAll(filepath.Join(m.attachmentRoot, account.ID)); err != nil {
			m.logger.Warn("remove obsolete attachments after UIDVALIDITY changed",
				"account_id", account.ID, "error", err)
		}
		account.UIDValidity = selected.UIDValidity
		account.LastUID = 0
	}
	set := imap.UIDSet{}
	set.AddRange(imap.UID(account.LastUID+1), 0)
	search, err := client.UIDSearch(&imap.SearchCriteria{UID: []imap.UIDSet{set}}, nil).Wait()
	if err != nil {
		return fmt.Errorf("search new messages: %w", err)
	}
	uids := search.AllUIDs()
	lastUID := account.LastUID
	for start := 0; start < len(uids); start += fetchBatchSize {
		end := start + fetchBatchSize
		if end > len(uids) {
			end = len(uids)
		}
		for _, uid := range uids[start:end] {
			if err := m.fetchOne(client, account, uid); err != nil {
				return err
			}
			if uint32(uid) > lastUID {
				lastUID = uint32(uid)
			}
		}
		if err := m.store.CompleteSync(context.Background(), account.ID, account.UIDValidity, lastUID); err != nil {
			return err
		}
		account.LastUID = lastUID
	}
	if len(uids) == 0 {
		return m.store.CompleteSync(context.Background(), account.ID, account.UIDValidity, lastUID)
	}
	return nil
}

func (m *Manager) fetchOne(client *imapclient.Client, account *domain.AccountSecret, uid imap.UID) error {
	section := &imap.FetchItemBodySection{Peek: true}
	cmd := client.Fetch(imap.UIDSetNum(uid), &imap.FetchOptions{
		UID: true, InternalDate: true, RFC822Size: true, BodySection: []*imap.FetchItemBodySection{section},
	})
	data := cmd.Next()
	if data == nil {
		_ = cmd.Close()
		return fmt.Errorf("message UID %d disappeared before fetch", uid)
	}
	receivedAt := time.Now().UTC()
	var size int64
	var rawPath string
	defer func() {
		if rawPath != "" {
			_ = os.Remove(rawPath)
		}
	}()
	for item := data.Next(); item != nil; item = data.Next() {
		switch item := item.(type) {
		case imapclient.FetchItemDataInternalDate:
			receivedAt = item.Time
		case imapclient.FetchItemDataRFC822Size:
			size = item.Size
		case imapclient.FetchItemDataBodySection:
			if item.Literal == nil {
				continue
			}
			if err := os.MkdirAll(m.attachmentRoot, 0o700); err != nil {
				_ = cmd.Close()
				return err
			}
			file, err := os.CreateTemp(m.attachmentRoot, ".message-*")
			if err != nil {
				_ = cmd.Close()
				return err
			}
			rawPath = file.Name()
			written, copyErr := io.Copy(file, item.Literal)
			closeErr := file.Close()
			if copyErr != nil {
				_ = cmd.Close()
				return fmt.Errorf("download UID %d: %w", uid, copyErr)
			}
			if closeErr != nil {
				_ = cmd.Close()
				return closeErr
			}
			if size == 0 {
				size = written
			}
		}
	}
	if err := cmd.Close(); err != nil {
		return fmt.Errorf("fetch UID %d: %w", uid, err)
	}
	if rawPath == "" {
		return fmt.Errorf("server returned no body for UID %d", uid)
	}
	file, err := os.Open(filepath.Clean(rawPath))
	if err != nil {
		return err
	}
	id := emailID(account.ID, account.UIDValidity, uint32(uid))
	email, parseErr := mailparse.Parse(file, id, account.ID, account.UIDValidity,
		uint32(uid), receivedAt, size, m.attachmentRoot)
	closeErr := file.Close()
	if parseErr != nil {
		return fmt.Errorf("parse UID %d: %w", uid, parseErr)
	}
	if closeErr != nil {
		return closeErr
	}
	if _, err := m.store.SaveEmail(context.Background(), email); err != nil {
		return fmt.Errorf("save UID %d: %w", uid, err)
	}
	return nil
}

func emailID(accountID string, uidValidity, uid uint32) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s:%d:%d", accountID, uidValidity, uid)))
	return "em_" + hex.EncodeToString(sum[:16])
}
