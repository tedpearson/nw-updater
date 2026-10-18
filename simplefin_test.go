package main

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestAccessUrlRoundTrip stores and reads back access urls whose ciphertext lengths cover every
// base64 padding case.
func TestAccessUrlRoundTrip(t *testing.T) {
	sf := SimpleFin{Passphrase: "test passphrase", TokenDir: t.TempDir()}
	for _, url := range []string{
		"https://user:pass@example.com/simplefin",
		"https://user:pass@example.com/simplefin/",
		"https://user:pass@example.com/simplefin//",
	} {
		if err := sf.storeAccessUrl([]byte(url), sf.Passphrase); err != nil {
			t.Fatalf("storeAccessUrl(%q): %v", url, err)
		}
		got, err := sf.GetAccessUrl()
		if err != nil {
			t.Fatalf("GetAccessUrl after storing %q: %v", url, err)
		}
		if got != url {
			t.Errorf("GetAccessUrl = %q, want %q", got, url)
		}
	}
}

// TestAuthenticateRejectsFailedClaim makes sure an error response isn't stored as the access url.
func TestAuthenticateRejectsFailedClaim(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "token already claimed", http.StatusForbidden)
	}))
	defer server.Close()
	sf := SimpleFin{Passphrase: "test passphrase", TokenDir: t.TempDir()}
	token := base64.StdEncoding.EncodeToString([]byte(server.URL + "/claim"))
	err := sf.Authenticate(token)
	if err == nil || !strings.Contains(err.Error(), "403") {
		t.Fatalf("Authenticate error = %v, want a 403 error", err)
	}
	if sf.IsAuthenticated() {
		t.Error("access url file was written for a failed claim")
	}
}
