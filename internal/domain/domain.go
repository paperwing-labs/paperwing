package domain

import (
	"errors"
	"time"
)

var ErrNotFound = errors.New("not found")

type Account struct {
	ID                    string     `json:"id"`
	Name                  string     `json:"name"`
	Host                  string     `json:"host"`
	Port                  int        `json:"port"`
	TLS                   bool       `json:"tls"`
	Username              string     `json:"username"`
	MonitorStatus         string     `json:"monitor_status"`
	LatestConnectionError string     `json:"latest_connection_error,omitempty"`
	LastSuccessfulSyncAt  *time.Time `json:"last_successful_sync_at,omitempty"`
	CreatedAt             time.Time  `json:"created_at"`
}

type AccountSecret struct {
	Account
	Password    string
	UIDValidity uint32
	LastUID     uint32
}

type NewAccount struct {
	Name     string `json:"name"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	TLS      bool   `json:"tls"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type Attachment struct {
	ID          string `json:"id"`
	EmailID     string `json:"email_id,omitempty"`
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	ContentID   string `json:"content_id,omitempty"`
	Size        int64  `json:"size"`
	Path        string `json:"-"`
}

type Email struct {
	ID          string              `json:"id"`
	AccountID   string              `json:"account_id"`
	UIDValidity uint32              `json:"-"`
	IMAPUID     uint32              `json:"-"`
	MessageID   string              `json:"message_id,omitempty"`
	Subject     string              `json:"subject"`
	From        []string            `json:"from"`
	To          []string            `json:"to"`
	Cc          []string            `json:"cc"`
	Headers     map[string][]string `json:"headers"`
	TextBody    string              `json:"text_body"`
	HTMLBody    string              `json:"html_body"`
	SentAt      *time.Time          `json:"sent_at,omitempty"`
	ReceivedAt  time.Time           `json:"received_at"`
	Size        int64               `json:"size"`
	Attachments []Attachment        `json:"attachments"`
	CreatedAt   time.Time           `json:"created_at"`
}

type EmailSummary struct {
	ID              string     `json:"id"`
	AccountID       string     `json:"account_id"`
	MessageID       string     `json:"message_id,omitempty"`
	Subject         string     `json:"subject"`
	From            []string   `json:"from"`
	To              []string   `json:"to"`
	SentAt          *time.Time `json:"sent_at,omitempty"`
	ReceivedAt      time.Time  `json:"received_at"`
	Size            int64      `json:"size"`
	AttachmentCount int        `json:"attachment_count"`
}

type EmailPage struct {
	Items    []EmailSummary `json:"items"`
	Page     int            `json:"page"`
	PageSize int            `json:"page_size"`
	Total    int            `json:"total"`
}

type ListEmailOptions struct {
	AccountID string
	Page      int
	PageSize  int
}
