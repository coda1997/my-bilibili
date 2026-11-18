package catalog

import (
	"context"
	"time"
)

type EntryKind uint8

const (
	BVEntry EntryKind = iota + 1
	AVEntry
)

// Entry is a locally parsed video identity. Page zero selects the first part.
type Entry struct {
	Kind EntryKind
	ID   string
	Page int
}

type PartRef struct {
	Video VideoID
	Part  PartID
}

// Code is a safe content-operation category, never a raw service message.
type Code string

const (
	None                 Code = ""
	Invalid              Code = "invalid-input"
	Unsupported          Code = "unsupported"
	Canceled             Code = "canceled"
	Timeout              Code = "timeout"
	Network              Code = "network"
	InvalidResponse      Code = "invalid-response"
	Unavailable          Code = "unavailable"
	Restricted           Code = "restricted"
	LoginRequired        Code = "login-required"
	SessionExpired       Code = "session-expired"
	VerificationRequired Code = "verification-required"
	RateLimited          Code = "rate-limited"
	ServiceUnavailable   Code = "service-unavailable"
	QualityUnavailable   Code = "quality-unavailable"
	Closed               Code = "closed"
)

func (c Code) Error() string { return string(c) }

type Summary struct {
	ID                      VideoID
	Title, Creator, CoverID string
	Duration                time.Duration
}

type Page struct {
	Number, TotalPages, TotalResults int
	Videos                           []Summary
}

type SearchSource interface {
	Search(context.Context, string, int) (Page, error)
}

// Cover owns normalized pixels without exposing a service URL to presentation.
type Cover struct {
	Width, Height int
	RGBA          []byte
}

type CoverSource interface {
	Cover(context.Context, string) (Cover, error)
}

type SearchPhase string

const (
	SearchInitial SearchPhase = "initial"
	SearchLoading SearchPhase = "loading"
	SearchReady   SearchPhase = "ready"
	SearchFailed  SearchPhase = "failed"
	SearchClosed  SearchPhase = "closed"
)

type SearchState struct {
	Phase                          SearchPhase
	Query                          string
	Generation                     uint64
	Results                        []Summary
	Page, TotalPages, TotalResults int
	LoadingMore, AtLimit           bool
	Error                          Code
}
