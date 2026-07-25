package semantic

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/hypertrial/intentci/internal/contract"
)

// TokenEnv is the environment variable for HTTP bearer credentials.
const TokenEnv = "INTENTCI_SEMANTIC_TOKEN"

// Provider analyzes a semantic request.
type Provider interface {
	Analyze(ctx context.Context, req Request) (Response, error)
}

// NewProvider constructs a local or HTTP provider from contract policy.
func NewProvider(p *contract.SemanticProvider) (Provider, error) {
	if p == nil {
		return nil, fmt.Errorf("semantic provider is not configured")
	}
	timeout := 2 * time.Minute
	if p.Timeout != "" {
		d, err := contract.ParseTimeout(p.Timeout)
		if err != nil {
			return nil, fmt.Errorf("semantic provider timeout: %w", err)
		}
		timeout = d
	}
	switch p.Type {
	case "local":
		return &LocalProvider{Command: p.Command, Timeout: timeout, Dir: ""}, nil
	case "http":
		return &HTTPProvider{
			URL:     p.URL,
			Timeout: timeout,
			Token:   os.Getenv(TokenEnv),
		}, nil
	default:
		return nil, fmt.Errorf("unsupported semantic provider type %q", p.Type)
	}
}
