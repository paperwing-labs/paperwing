package mailaddr

import "testing"

func TestFormatKeepsUnicodeDisplayNameReadable(t *testing.T) {
	got := Format("腾讯云", "cloud_noreply@tencent.com")
	want := "腾讯云 <cloud_noreply@tencent.com>"
	if got != want {
		t.Fatalf("Format()=%q want %q", got, want)
	}
}

func TestNormalizeDecodesQuotedEncodedWord(t *testing.T) {
	got := Normalize(`"=?utf-8?B?6IW+6K6v5LqR?=" <cloud_noreply@tencent.com>`)
	want := "腾讯云 <cloud_noreply@tencent.com>"
	if got != want {
		t.Fatalf("Normalize()=%q want %q", got, want)
	}
}

func TestNormalizePreservesPlainAddress(t *testing.T) {
	got := Normalize("Alice Example <alice@example.com>")
	want := "Alice Example <alice@example.com>"
	if got != want {
		t.Fatalf("Normalize()=%q want %q", got, want)
	}
}
