// Package channel owns the fleet conversation transport and its durable state.
package channel

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

type MessageRef struct {
	ID       string `json:"id"`
	ThreadID string `json:"threadID"`
}

type Cursor string

type Inbound struct {
	Ref      MessageRef `json:"ref"`
	ThreadID string     `json:"threadID"`
	UserID   string     `json:"userID"`
	Text     string     `json:"text"`
	At       time.Time  `json:"at"`
}

type CredentialIdentity struct {
	UserID string `json:"userID"`
}

type DestinationConfig struct {
	Name      string
	Provider  string
	ChannelID string
	Token     string
	APIBase   string
	Secrets   []string
}

type Provider interface {
	Post(context.Context, DestinationConfig, string, *MessageRef) (MessageRef, error)
	Receive(context.Context, DestinationConfig, []MessageRef, Cursor) ([]Inbound, Cursor, error)
	Credential(context.Context, DestinationConfig) (CredentialIdentity, error)
}

type ErrorKind string

const (
	Unconfigured  ErrorKind = "unconfigured"
	SendFailed    ErrorKind = "send failed"
	ReceiveFailed ErrorKind = "receive failed"
)

type ProviderError struct {
	Kind    ErrorKind
	Problem string
}

func (e *ProviderError) Error() string { return string(e.Kind) + ": " + e.Problem }
func IsKind(err error, kind ErrorKind) bool {
	var e *ProviderError
	return errors.As(err, &e) && e.Kind == kind
}
func ErrUnconfigured(problem string) error {
	return &ProviderError{Kind: Unconfigured, Problem: problem}
}
func ErrSendFailed(problem string) error { return &ProviderError{Kind: SendFailed, Problem: problem} }
func ErrReceiveFailed(problem string) error {
	return &ProviderError{Kind: ReceiveFailed, Problem: problem}
}

func Scrub(problem string, secrets ...string) string {
	for _, secret := range secrets {
		if secret != "" {
			problem = strings.ReplaceAll(problem, secret, "[REDACTED]")
		}
	}
	return problem
}

type Factory func(DestinationConfig) (Provider, DestinationConfig, error)
type Registry struct{ factories map[string]Factory }

func NewRegistry() *Registry                        { return &Registry{factories: map[string]Factory{}} }
func (r *Registry) Register(name string, f Factory) { r.factories[name] = f }
func (r *Registry) Resolve(name string, cfg DestinationConfig) (Provider, DestinationConfig, error) {
	f := r.factories[name]
	if f == nil {
		return nil, cfg, fmt.Errorf("unknown channel adapter %q", name)
	}
	return f(cfg)
}
