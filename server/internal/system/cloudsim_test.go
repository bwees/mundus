package system

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTemp(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return path
}

func TestCloudEndpointFrom(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
		wantErr bool
	}{
		{
			name:    "device binding payload",
			content: `{"resultCode":100,"data":{"mqtt":{"address":"a2alhn2dfztqv9-ats.iot.us-east-1.amazonaws.com","clientID":"Thing_x","port":8883},"cert":{"CertificatePem":"x"}},"message":""}`,
			want:    "a2alhn2dfztqv9-ats.iot.us-east-1.amazonaws.com",
		},
		{
			name:    "missing address is an error, not an empty host",
			content: `{"data":{"mqtt":{"port":8883}}}`,
			wantErr: true,
		},
		{
			name:    "malformed json",
			content: `{"data":`,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := cloudEndpointFrom(writeTemp(t, "cert_info", tt.content))
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCloudEndpointFromMissingFile(t *testing.T) {
	if _, err := cloudEndpointFrom(filepath.Join(t.TempDir(), "absent")); err == nil {
		t.Fatal("expected an error for a missing cert_info")
	}
}
