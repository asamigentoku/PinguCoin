package grpcclient

import "testing"

func TestTransport(t *testing.T) {
	tests := []struct {
		name    string
		address string
		target  string
		wantErr bool
	}{
		{name: "local bare address", address: "localhost:8080", target: "localhost:8080"},
		{name: "local HTTP URL", address: "http://localhost:8080", target: "localhost:8080"},
		{name: "hosted HTTPS URL", address: "https://orcan-staging.vercel.app", target: "orcan-staging.vercel.app"},
		{name: "empty address", address: "", wantErr: true},
		{name: "path is rejected", address: "https://example.com/rpc", wantErr: true},
		{name: "unsupported scheme", address: "ftp://example.com", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target, credentials, err := Transport(tt.address)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected an error")
				}
				return
			}
			if err != nil {
				t.Fatalf("Transport() error = %v", err)
			}
			if target != tt.target {
				t.Fatalf("Transport() target = %q, want %q", target, tt.target)
			}
			if credentials == nil {
				t.Fatal("Transport() returned nil credentials")
			}
		})
	}
}
