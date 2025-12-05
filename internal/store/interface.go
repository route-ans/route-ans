// Package store provides pluggable persistent storage implementations.
package store

import (
	"context"
	"time"

	"github.com/route-ans/route-ans/internal/models"
)

// Provider defines the interface for persistent storage implementations.
type Provider interface {
	// Save stores or updates a resolution record.
	Save(ctx context.Context, record *models.ResolutionRecord) error

	// Get retrieves a resolution record by ANSName.
	// Returns nil, nil if not found.
	Get(ctx context.Context, ansName string) (*models.ResolutionRecord, error)

	// GetByFQDN retrieves all records for a given FQDN (all versions).
	GetByFQDN(ctx context.Context, fqdn string) ([]*models.ResolutionRecord, error)

	// Search performs a search query and returns matching records.
	Search(ctx context.Context, query *SearchQuery) (*SearchResult, error)

	// Delete removes a record by ANSName.
	Delete(ctx context.Context, ansName string) error

	// List returns all records with pagination.
	List(ctx context.Context, opts *ListOptions) (*SearchResult, error)

	// Count returns the total number of records.
	Count(ctx context.Context) (int64, error)

	// Close releases any resources held by the store.
	Close() error

	// Name returns the provider name for logging/metrics.
	Name() string

	// Migrate runs any necessary database migrations.
	Migrate(ctx context.Context) error
}

// SearchQuery represents a search request
type SearchQuery struct {
	// Text is a free-text search query
	Text string

	// Protocol filters by protocol (e.g., "a2a", "mcp")
	Protocol string

	// ProviderID filters by provider ID
	ProviderID string

	// Capability filters by capability
	Capability string

	// Status filters by status (e.g., "active", "revoked")
	Status string

	// Capabilities filters by capabilities (agent must have all listed)
	Capabilities []string

	// Tags filters by tags
	Tags []string

	// Limit is the maximum number of results
	Limit int

	// Offset is the pagination offset
	Offset int

	// OrderBy specifies the sort field
	OrderBy string

	// OrderDesc specifies descending order
	OrderDesc bool
}

// SearchResult contains search results with pagination info
type SearchResult struct {
	// Records is the list of matching records
	Records []*models.ResolutionRecord

	// Total is the total number of matching records (before pagination)
	Total int64

	// Limit is the limit used in the query
	Limit int

	// Offset is the offset used in the query
	Offset int

	// HasMore indicates if there are more results
	HasMore bool
}

// ListOptions contains pagination options for List
type ListOptions struct {
	Limit   int
	Offset  int
	OrderBy string
	Desc    bool
}

// DefaultListOptions returns sensible defaults for listing
func DefaultListOptions() *ListOptions {
	return &ListOptions{
		Limit:   100,
		Offset:  0,
		OrderBy: "updated_at",
		Desc:    true,
	}
}

// Options contains common store configuration options
type Options struct {
	// MaxSize is the maximum number of records (for memory-based stores)
	MaxSize int

	// MigrationsEnabled enables automatic migrations
	MigrationsEnabled bool

	// MigrationsPath is the path to migration files
	MigrationsPath string
}

// DefaultOptions returns sensible default store options
func DefaultOptions() Options {
	return Options{
		MaxSize:           100000,
		MigrationsEnabled: true,
	}
}

// Factory is a function that creates a new store provider
type Factory func(opts Options, config map[string]interface{}) (Provider, error)

// registry holds registered store provider factories
var registry = make(map[string]Factory)

// Register registers a store provider factory
func Register(name string, factory Factory) {
	registry[name] = factory
}

// New creates a new store provider by name
func New(name string, opts Options, config map[string]interface{}) (Provider, error) {
	factory, ok := registry[name]
	if !ok {
		return nil, &ErrUnknownProvider{Name: name}
	}
	return factory(opts, config)
}

// Available returns the names of all registered store providers
func Available() []string {
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	return names
}

// ErrUnknownProvider is returned when an unknown store provider is requested
type ErrUnknownProvider struct {
	Name string
}

func (e *ErrUnknownProvider) Error() string {
	return "unknown store provider: " + e.Name
}

// ErrNotFound is returned when a record is not found
type ErrNotFound struct {
	ANSName string
}

func (e *ErrNotFound) Error() string {
	return "record not found: " + e.ANSName
}

// IndexEntry represents a lightweight index entry for search
type IndexEntry struct {
	ANSName      string    `json:"ansName"`
	FQDN         string    `json:"fqdn"`
	Protocol     string    `json:"protocol"`
	ProviderID   string    `json:"providerId"`
	Capability   string    `json:"capability"`
	Version      string    `json:"version"`
	Status       string    `json:"status"`
	DisplayName  string    `json:"displayName"`
	Description  string    `json:"description"`
	Capabilities []string  `json:"capabilities"`
	Endpoint     string    `json:"endpoint"`
	UpdatedAt    time.Time `json:"updatedAt"`
}
