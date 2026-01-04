package commands

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"
)

const testVersion = "1.3.4"

func TestSelfUpgrade_NetworkError(t *testing.T) {
	t.Parallel()

	client := &http.Client{
		Transport: errorTransport{err: errors.New("network unavailable")},
	}

	err := SelfUpgradeWithClient(client, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var upgradeErr *UpgradeError
	if !errors.As(err, &upgradeErr) {
		t.Fatalf("expected UpgradeError, got %T", err)
	}

	if !strings.Contains(upgradeErr.Msg, "failed to check for updates") {
		t.Errorf("expected message to contain 'failed to check for updates', got: %s", upgradeErr.Msg)
	}

	if upgradeErr.Hint == "" {
		t.Error("expected hint to be set")
	}
}

func TestSelfUpgrade_UnsupportedPlatform(t *testing.T) {
	t.Skip("Cannot test unsupported platform - runtime.GOOS is not modifiable")
}

func TestSelfUpgrade_AlreadyUpToDate(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"tag_name": "v" + testVersion,
			"html_url": "https://github.com/ozacod/cpx/releases/" + testVersion,
		})
	}))
	defer server.Close()

	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.String() == "https://api.github.com/repos/ozacod/cpx/releases/latest" {
				return http.Get(server.URL)
			}
			return http.DefaultTransport.RoundTrip(req)
		}),
	}

	err := SelfUpgradeWithClient(client, nil)
	if err != nil {
		t.Errorf("expected nil error (already up to date), got: %v", err)
	}
}

func TestSelfUpgrade_NoReleasesFound(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.String() == "https://api.github.com/repos/ozacod/cpx/releases/latest" {
				return http.Get(server.URL)
			}
			return http.DefaultTransport.RoundTrip(req)
		}),
	}

	err := SelfUpgradeWithClient(client, nil)
	if err != nil {
		t.Errorf("expected nil error (no releases), got: %v", err)
	}
}

func TestSelfUpgrade_APIError(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}))
	defer server.Close()

	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.String() == "https://api.github.com/repos/ozacod/cpx/releases/latest" {
				return http.Get(server.URL)
			}
			return http.DefaultTransport.RoundTrip(req)
		}),
	}

	err := SelfUpgradeWithClient(client, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var upgradeErr *UpgradeError
	if !errors.As(err, &upgradeErr) {
		t.Fatalf("expected UpgradeError, got %T", err)
	}

	if !strings.Contains(upgradeErr.Msg, "failed to check for updates") {
		t.Errorf("expected message to contain 'failed to check for updates', got: %s", upgradeErr.Msg)
	}
}

func TestSelfUpgrade_DownloadError(t *testing.T) {
	t.Parallel()
	goos := runtime.GOOS
	goarch := runtime.GOARCH
	binaryName := "cpx-" + goos + "-" + goarch
	if goos == "windows" {
		binaryName += ".exe"
	}

	latestServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"tag_name": "v99.99.99",
			"html_url": "https://github.com/ozacod/cpx/releases/v99.99.99",
		})
	}))
	defer latestServer.Close()

	downloadServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer downloadServer.Close()

	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.String() == "https://api.github.com/repos/ozacod/cpx/releases/latest" {
				return http.Get(latestServer.URL)
			}
			if strings.Contains(req.URL.Path, binaryName) {
				return http.Get(downloadServer.URL)
			}
			return http.DefaultTransport.RoundTrip(req)
		}),
	}

	err := SelfUpgradeWithClient(client, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var upgradeErr *UpgradeError
	if !errors.As(err, &upgradeErr) {
		t.Fatalf("expected UpgradeError, got %T", err)
	}

	if !strings.Contains(upgradeErr.Msg, "download failed") {
		t.Errorf("expected message to contain 'download failed', got: %s", upgradeErr.Msg)
	}
}

func TestSelfUpgrade_JSONDecodeError(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.String() == "https://api.github.com/repos/ozacod/cpx/releases/latest" {
				return http.Get(server.URL)
			}
			return http.DefaultTransport.RoundTrip(req)
		}),
	}

	err := SelfUpgradeWithClient(client, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var upgradeErr *UpgradeError
	if !errors.As(err, &upgradeErr) {
		t.Fatalf("expected UpgradeError, got %T", err)
	}

	if !strings.Contains(upgradeErr.Msg, "failed to parse release info") {
		t.Errorf("expected message to contain 'failed to parse release info', got: %s", upgradeErr.Msg)
	}
}

func TestSelfUpgrade_RenameError(t *testing.T) {
	t.Skip("Skipping - requires specific test environment setup")
}

func TestSelfUpgrade_Success(t *testing.T) {
	t.Skip("Skipping - requires specific test environment setup")
}

func TestUpgradeError_Error(t *testing.T) {
	t.Parallel()
	err := &UpgradeError{
		Msg:  "test error message",
		Hint: "test hint",
	}

	if err.Error() != "test error message" {
		t.Errorf("expected Error() to return Msg, got: %s", err.Error())
	}
}

func TestUpgradeError_Unwrap(t *testing.T) {
	t.Parallel()
	err := &UpgradeError{
		Msg:  "test error message",
		Hint: "test hint",
	}

	unwrapped := errors.Unwrap(err)
	if unwrapped == nil {
		t.Error("expected unwrapped error to not be nil")
	}
	if unwrapped.Error() != "test error message" {
		t.Errorf("expected unwrapped error message to match, got: %s", unwrapped.Error())
	}
}

func TestUpgradeError_As(t *testing.T) {
	t.Parallel()
	err := &UpgradeError{
		Msg:  "test error message",
		Hint: "test hint",
	}

	var upgradeErr *UpgradeError
	if !errors.As(err, &upgradeErr) {
		t.Error("expected errors.As to succeed")
	}

	if upgradeErr.Msg != "test error message" {
		t.Errorf("expected Msg to match, got: %s", upgradeErr.Msg)
	}

	if upgradeErr.Hint != "test hint" {
		t.Errorf("expected Hint to match, got: %s", upgradeErr.Hint)
	}
}

type errorTransport struct {
	err error
}

func (e errorTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return nil, e.err
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
