package selfupdate

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIsNewer(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"0.1.5", "0.1.4", true},
		{"0.1.4", "0.1.5", false},
		{"0.1.5", "0.1.5", false},
		{"v0.1.5", "0.1.4", true},          // tolerate "v" prefix
		{"0.2.0", "0.1.9", true},           // minor bump
		{"1.0.0", "0.9.9", true},           // major bump
		{"0.1.5", "0.1.5-rc.1", true},      // release > prerelease
		{"0.1.5-rc.2", "0.1.5-rc.1", true}, // rc ordering
		{"0.1.5-rc.1", "0.1.5", false},
		{"0.1.5", "dev", true},      // unparseable local → assume outdated
		{"garbage", "0.1.4", false}, // unparseable remote → not newer
	}
	for _, tc := range cases {
		if got := IsNewer(tc.a, tc.b); got != tc.want {
			t.Errorf("IsNewer(%q, %q) = %v, want %v", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestParseVersion(t *testing.T) {
	if got := ParseVersion("v0.1.5"); got == nil || got[0] != 0 || got[1] != 1 || got[2] != 5 {
		t.Errorf("ParseVersion(v0.1.5) = %v", got)
	}
	if ParseVersion("dev") != nil {
		t.Error("ParseVersion(dev) should be nil")
	}
	if ParseVersion("01.0.0") != nil {
		t.Error("ParseVersion with leading zero should be nil")
	}
}

func TestFetchLatest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"version":"0.1.6","name":"@model-go/cli"}`))
	}))
	defer srv.Close()

	oldURL, oldClient := registryURL, DefaultClient
	registryURL, DefaultClient = srv.URL, srv.Client()
	defer func() { registryURL, DefaultClient = oldURL, oldClient }()

	v, err := FetchLatest()
	if err != nil {
		t.Fatalf("FetchLatest error: %v", err)
	}
	if v != "0.1.6" {
		t.Errorf("FetchLatest = %q, want 0.1.6", v)
	}
}

func TestFetchLatest_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	oldURL, oldClient := registryURL, DefaultClient
	registryURL, DefaultClient = srv.URL, srv.Client()
	defer func() { registryURL, DefaultClient = oldURL, oldClient }()

	if _, err := FetchLatest(); err == nil {
		t.Error("expected error on HTTP 500, got nil")
	}
}
