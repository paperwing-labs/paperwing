package accounts

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/paperwing/paperwing/internal/domain"
)

var ErrInvalidInput = errors.New("invalid account input")

type Repository interface {
	CreateAccount(context.Context, domain.AccountSecret) error
	ListAccounts(context.Context) ([]domain.Account, error)
	DeleteAccount(context.Context, string) error
}

type Monitor interface {
	StartAccount(string)
	StopAccount(string)
	Sync(context.Context, string) error
}

type Tester func(context.Context, domain.AccountSecret) error

type Service struct {
	repo           Repository
	monitor        Monitor
	testConnection Tester
	attachmentRoot string
}

func New(repo Repository, monitor Monitor, testConnection Tester, attachmentRoot string) *Service {
	return &Service{repo: repo, monitor: monitor, testConnection: testConnection, attachmentRoot: attachmentRoot}
}

func (s *Service) Test(ctx context.Context, input domain.NewAccount) error {
	account, err := prepare(input)
	if err != nil {
		return err
	}
	return s.testConnection(ctx, account)
}

func (s *Service) Create(ctx context.Context, input domain.NewAccount) (domain.Account, error) {
	account, err := prepare(input)
	if err != nil {
		return domain.Account{}, err
	}
	if err := s.testConnection(ctx, account); err != nil {
		return domain.Account{}, err
	}
	account.ID, err = newID("ac")
	if err != nil {
		return domain.Account{}, err
	}
	account.CreatedAt = time.Now().UTC()
	account.MonitorStatus = "starting"
	if err := s.repo.CreateAccount(ctx, account); err != nil {
		return domain.Account{}, err
	}
	s.monitor.StartAccount(account.ID)
	return account.Account, nil
}

func (s *Service) List(ctx context.Context) ([]domain.Account, error) {
	return s.repo.ListAccounts(ctx)
}

func (s *Service) Delete(ctx context.Context, id string) error {
	s.monitor.StopAccount(id)
	if err := s.repo.DeleteAccount(ctx, id); err != nil {
		return err
	}
	return os.RemoveAll(filepath.Join(s.attachmentRoot, id))
}

func (s *Service) Sync(ctx context.Context, id string) error {
	return s.monitor.Sync(ctx, id)
}

func prepare(input domain.NewAccount) (domain.AccountSecret, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.Host = strings.TrimSpace(input.Host)
	input.Username = strings.TrimSpace(input.Username)
	if input.Name == "" || input.Host == "" || input.Username == "" || input.Password == "" {
		return domain.AccountSecret{}, fmt.Errorf("%w: name, host, username and password are required", ErrInvalidInput)
	}
	if input.Port == 0 {
		if input.TLS {
			input.Port = 993
		} else {
			input.Port = 143
		}
	}
	if input.Port < 1 || input.Port > 65535 {
		return domain.AccountSecret{}, fmt.Errorf("%w: port must be between 1 and 65535", ErrInvalidInput)
	}
	return domain.AccountSecret{Account: domain.Account{
		Name: input.Name, Host: input.Host, Port: input.Port, TLS: input.TLS, Username: input.Username,
	}, Password: input.Password}, nil
}

func newID(prefix string) (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return prefix + "_" + hex.EncodeToString(b), nil
}
