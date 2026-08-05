package mailparse

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/emersion/go-message"
	"github.com/emersion/go-message/mail"
	"github.com/paperwing/paperwing/internal/domain"
	"github.com/paperwing/paperwing/internal/mailaddr"
)

const maxTextBodyBytes = 10 << 20

func Parse(r io.Reader, emailID, accountID string, uidValidity, uid uint32, receivedAt time.Time, size int64, attachmentRoot string) (domain.Email, error) {
	reader, err := mail.CreateReader(r)
	if err != nil && !message.IsUnknownCharset(err) {
		return domain.Email{}, fmt.Errorf("parse message: %w", err)
	}
	now := time.Now().UTC()
	email := domain.Email{
		ID: emailID, AccountID: accountID, UIDValidity: uidValidity, IMAPUID: uid,
		Headers: map[string][]string{}, Attachments: []domain.Attachment{}, ReceivedAt: receivedAt.UTC(), Size: size, CreatedAt: now,
	}
	if email.ReceivedAt.IsZero() {
		email.ReceivedAt = now
	}
	email.Subject, _ = reader.Header.Subject()
	email.MessageID, _ = reader.Header.MessageID()
	email.From = addresses(reader.Header, "From")
	email.To = addresses(reader.Header, "To")
	email.Cc = addresses(reader.Header, "Cc")
	if date, dateErr := reader.Header.Date(); dateErr == nil && !date.IsZero() {
		email.SentAt = &date
	}
	fields := reader.Header.Fields()
	for fields.Next() {
		value, textErr := fields.Text()
		if textErr != nil {
			value = fields.Value()
		}
		email.Headers[fields.Key()] = append(email.Headers[fields.Key()], value)
	}

	dir := filepath.Join(attachmentRoot, accountID, emailID)
	attachmentIndex := 0
	for {
		part, partErr := reader.NextPart()
		if partErr == io.EOF {
			break
		}
		if partErr != nil && !message.IsUnknownCharset(partErr) {
			return domain.Email{}, fmt.Errorf("read MIME part: %w", partErr)
		}
		switch header := part.Header.(type) {
		case *mail.InlineHeader:
			mediaType, typeParams, _ := header.ContentType()
			if !strings.HasPrefix(strings.ToLower(mediaType), "text/") {
				_, dispositionParams, _ := header.ContentDisposition()
				filename := dispositionParams["filename"]
				if filename == "" {
					filename = typeParams["name"]
				}
				attachmentIndex++
				attachment, saveErr := saveAttachment(part.Body, dir, emailID, attachmentIndex,
					filename, mediaType, strings.Trim(header.Get("Content-ID"), "<>"))
				if saveErr != nil {
					return domain.Email{}, saveErr
				}
				email.Attachments = append(email.Attachments, attachment)
				continue
			}
			body, readErr := io.ReadAll(io.LimitReader(part.Body, maxTextBodyBytes+1))
			if readErr != nil {
				return domain.Email{}, readErr
			}
			if len(body) > maxTextBodyBytes {
				body = body[:maxTextBodyBytes]
			}
			switch strings.ToLower(mediaType) {
			case "text/plain":
				if email.TextBody == "" {
					email.TextBody = string(body)
				}
			case "text/html":
				if email.HTMLBody == "" {
					email.HTMLBody = string(body)
				}
			}
		case *mail.AttachmentHeader:
			attachmentIndex++
			filename, _ := header.Filename()
			mediaType, _, _ := header.ContentType()
			attachment, saveErr := saveAttachment(part.Body, dir, emailID, attachmentIndex,
				filename, mediaType, strings.Trim(header.Get("Content-ID"), "<>"))
			if saveErr != nil {
				return domain.Email{}, saveErr
			}
			email.Attachments = append(email.Attachments, attachment)
		}
	}
	return email, nil
}

func saveAttachment(body io.Reader, dir, emailID string, index int, filename, mediaType, contentID string) (domain.Attachment, error) {
	if filename == "" {
		filename = fmt.Sprintf("attachment-%d", index)
	}
	if mediaType == "" {
		mediaType = "application/octet-stream"
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return domain.Attachment{}, err
	}
	id := fmt.Sprintf("%s-a%03d", emailID, index)
	path := filepath.Join(dir, id)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return domain.Attachment{}, err
	}
	written, copyErr := io.Copy(file, body)
	closeErr := file.Close()
	if copyErr != nil || closeErr != nil {
		_ = os.Remove(path)
		if copyErr != nil {
			return domain.Attachment{}, copyErr
		}
		return domain.Attachment{}, closeErr
	}
	return domain.Attachment{ID: id, EmailID: emailID, Filename: filename,
		ContentType: mediaType, ContentID: contentID, Size: written, Path: path}, nil
}

func addresses(header mail.Header, key string) []string {
	list, err := header.AddressList(key)
	if err != nil {
		return []string{}
	}
	out := make([]string, 0, len(list))
	for _, address := range list {
		out = append(out, mailaddr.Format(address.Name, address.Address))
	}
	return out
}
