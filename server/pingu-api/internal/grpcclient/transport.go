package grpcclient

import (
	"crypto/tls"
	"fmt"
	"net/url"
	"strings"

	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

// Transport converts an application endpoint into a gRPC target and transport
// credentials. Bare addresses keep the local-development behavior (h2c), while
// https:// endpoints use TLS for hosted environments such as Vercel.
func Transport(addr string) (string, credentials.TransportCredentials, error) {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return "", nil, fmt.Errorf("gRPC address is empty")
	}

	if !strings.Contains(addr, "://") {
		return addr, insecure.NewCredentials(), nil
	}

	endpoint, err := url.Parse(addr)
	if err != nil {
		return "", nil, fmt.Errorf("parse gRPC address: %w", err)
	}
	if endpoint.Host == "" || (endpoint.Path != "" && endpoint.Path != "/") || endpoint.RawQuery != "" || endpoint.Fragment != "" {
		return "", nil, fmt.Errorf("gRPC address must contain only a scheme and host")
	}

	switch endpoint.Scheme {
	case "http":
		return endpoint.Host, insecure.NewCredentials(), nil
	case "https":
		return endpoint.Host, credentials.NewTLS(&tls.Config{
			MinVersion: tls.VersionTLS12,
			ServerName: endpoint.Hostname(),
		}), nil
	default:
		return "", nil, fmt.Errorf("unsupported gRPC address scheme %q", endpoint.Scheme)
	}
}
