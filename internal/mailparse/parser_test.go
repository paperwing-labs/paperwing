package mailparse

import (
	"strings"
	"testing"
	"time"
)

func TestParseMultipart(t *testing.T) {
	raw := "From: Alice <alice@example.com>\r\n" +
		"To: Bob <bob@example.com>\r\nSubject: =?UTF-8?Q?Hello_World?=\r\n" +
		"Message-ID: <one@example.com>\r\nContent-Type: multipart/mixed; boundary=x\r\n\r\n" +
		"--x\r\nContent-Type: text/plain; charset=utf-8\r\n\r\nbody\r\n" +
		"--x\r\nContent-Type: application/octet-stream\r\nContent-Disposition: attachment; filename=test.bin\r\n\r\ndata\r\n--x--\r\n"
	email, err := Parse(strings.NewReader(raw), "e1", "ac1", 7, 8, time.Now(), int64(len(raw)), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if email.Subject != "Hello World" || !strings.Contains(email.TextBody, "body") {
		t.Fatalf("unexpected parsed email: %#v", email)
	}
	if len(email.Attachments) != 1 || email.Attachments[0].Filename != "test.bin" {
		t.Fatalf("unexpected attachments: %#v", email.Attachments)
	}
}

func TestParseKeepsEncodedAddressNameReadable(t *testing.T) {
	raw := "From: \"=?utf-8?B?6IW+6K6v5LqR?=\" <cloud_noreply@tencent.com>\r\n" +
		"To: \"=?utf-8?B?eWFuZy5saXU=?=\" <yang.liu@example.com>\r\n" +
		"Subject: Login notice\r\nContent-Type: text/plain; charset=utf-8\r\n\r\nbody\r\n"
	email, err := Parse(strings.NewReader(raw), "e1", "ac1", 7, 8, time.Now(), int64(len(raw)), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if got, want := email.From[0], "腾讯云 <cloud_noreply@tencent.com>"; got != want {
		t.Fatalf("From=%q want %q", got, want)
	}
	if got, want := email.To[0], "yang.liu <yang.liu@example.com>"; got != want {
		t.Fatalf("To=%q want %q", got, want)
	}
}
