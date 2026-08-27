package pbx

import "testing"

// TestIsAuthError is the regression test for a real incident (2026-08-27):
// a long-running backend's cached OnlinePBX session expired, but isAuthError
// only matched errorCode "WRONG_AUTH_DATA" — the malformed-key case — so the
// expired-but-well-formed-session response (errorCode "API_KEY_CHECK_FAILED",
// isNotAuth: true) was never recognized, the retry-with-a-fresh-session path
// in post() never fired, and every request failed with a bare "status 0"
// until the process was restarted.
func TestIsAuthError(t *testing.T) {
	tests := []struct {
		name string
		body string
		want bool
	}{
		{
			name: "expired session (isNotAuth, API_KEY_CHECK_FAILED) — the actual incident",
			body: `{"status":"0","comment":"not authorized: hash compare error","isNotAuth":true,"errorCode":"API_KEY_CHECK_FAILED"}`,
			want: true,
		},
		{
			name: "malformed auth_key (WRONG_AUTH_DATA)",
			body: `{"status":"0","comment":"wrong auth data","errorCode":"WRONG_AUTH_DATA"}`,
			want: true,
		},
		{
			name: "successful response",
			body: `{"status":"1","data":[]}`,
			want: false,
		},
		{
			name: "unrelated application-level error (not an auth problem)",
			body: `{"status":"0","comment":"some other failure","errorCode":"SOME_OTHER_CODE"}`,
			want: false,
		},
		{
			name: "unparseable body",
			body: `not json`,
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isAuthError([]byte(tt.body)); got != tt.want {
				t.Errorf("isAuthError(%s) = %v, want %v", tt.body, got, tt.want)
			}
		})
	}
}
