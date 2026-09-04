// Package channel owns the fleet conversation transport and its durable state.
package channel

import (
	"context"
	"errors"
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
	SentAt   time.Time  `json:"sentAt"`
	Ack      Cursor     `json:"ack"`
	UpdateID int64      `json:"updateID"`
}

type CredentialIdentity struct {
	UserID string `json:"userID"`
}

type DestinationConfig struct {
	Name        string
	Provider    string
	ChannelID   string
	Token       string
	APIBase     string
	Secrets     []string
	HTTPTimeout time.Duration
}

type Provider interface {
	Post(context.Context, DestinationConfig, string, *MessageRef) (MessageRef, error)
	Receive(context.Context, DestinationConfig, []MessageRef, Cursor) ([]Inbound, Cursor, error)
	Confirm(context.Context, DestinationConfig, Cursor) error
	Credential(context.Context, DestinationConfig) (CredentialIdentity, error)
}

type ErrorKind string

const (
	Unconfigured  ErrorKind = "unconfigured"
	SendFailed    ErrorKind = "send failed"
	ReceiveFailed ErrorKind = "receive failed"
	Busy          ErrorKind = "busy"
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
func ErrBusy(problem string) error { return &ProviderError{Kind: Busy, Problem: problem} }

func Scrub(problem string, secrets ...string) string {
	for _, secret := range secrets {
		if secret != "" {
			problem = strings.ReplaceAll(problem, secret, "[REDACTED]")
		}
	}
	return problem
}
