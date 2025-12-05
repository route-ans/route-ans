// Package store provides pluggable persistent storage implementations.
package store

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/route-ans/route-ans/internal/models"
)

// memoryStore is an in-memory store implementation for testing/development
type memoryStore struct {
	mu      sync.RWMutex
	records map[string]*models.ResolutionRecord
	maxSize int
}

// NewMemoryStore creates a new in-memory store
func NewMemoryStore(opts Options, config map[string]interface{}) (Provider, error) {
	if opts.MaxSize <= 0 {
		opts.MaxSize = 100000
	}

	return &memoryStore{
		records: make(map[string]*models.ResolutionRecord),
		maxSize: opts.MaxSize,
	}, nil
}

// Save stores or updates a record
func (s *memoryStore) Save(ctx context.Context, record *models.ResolutionRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.records) >= s.maxSize {
		return &ErrStoreFull{MaxSize: s.maxSize}
	}

	record.UpdatedAt = time.Now()
	s.records[record.ANSName] = record
	return nil
}

// Get retrieves a record by ANSName
func (s *memoryStore) Get(ctx context.Context, ansName string) (*models.ResolutionRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	record, exists := s.records[ansName]
	if !exists {
		return nil, nil
	}

	return record, nil
}

// GetByFQDN retrieves all records for a FQDN
func (s *memoryStore) GetByFQDN(ctx context.Context, fqdn string) ([]*models.ResolutionRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []*models.ResolutionRecord
	for _, record := range s.records {
		if record.FQDN == fqdn {
			results = append(results, record)
		}
	}

	// Sort by version (newest first)
	sort.Slice(results, func(i, j int) bool {
		return results[i].Version > results[j].Version
	})

	return results, nil
}

// Search performs a search query
func (s *memoryStore) Search(ctx context.Context, query *SearchQuery) (*SearchResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var matches []*models.ResolutionRecord

	for _, record := range s.records {
		if s.matchesQuery(record, query) {
			matches = append(matches, record)
		}
	}

	// Sort results
	s.sortResults(matches, query.OrderBy, query.OrderDesc)

	// Apply pagination
	total := int64(len(matches))
	limit := query.Limit
	if limit <= 0 {
		limit = 100
	}
	offset := query.Offset
	if offset < 0 {
		offset = 0
	}

	start := offset
	if start > len(matches) {
		start = len(matches)
	}
	end := start + limit
	if end > len(matches) {
		end = len(matches)
	}

	return &SearchResult{
		Records: matches[start:end],
		Total:   total,
		Limit:   limit,
		Offset:  offset,
		HasMore: end < len(matches),
	}, nil
}

func (s *memoryStore) matchesQuery(record *models.ResolutionRecord, query *SearchQuery) bool {
	// Text search (simple substring match)
	if query.Text != "" {
		text := strings.ToLower(query.Text)
		if !strings.Contains(strings.ToLower(record.ANSName), text) &&
			!strings.Contains(strings.ToLower(record.DisplayName), text) &&
			!strings.Contains(strings.ToLower(record.Description), text) {
			return false
		}
	}

	// Protocol filter
	if query.Protocol != "" && record.Protocol != query.Protocol {
		return false
	}

	// Status filter
	if query.Status != "" && record.Status != query.Status {
		return false
	}

	// Capability filter
	if query.Capability != "" {
		found := false
		for _, cap := range record.Capabilities {
			if cap == query.Capability {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Capabilities filter (must have all)
	if len(query.Capabilities) > 0 {
		recordCaps := make(map[string]bool)
		for _, cap := range record.Capabilities {
			recordCaps[cap] = true
		}
		for _, reqCap := range query.Capabilities {
			if !recordCaps[reqCap] {
				return false
			}
		}
	}

	return true
}

func (s *memoryStore) sortResults(records []*models.ResolutionRecord, orderBy string, desc bool) {
	if orderBy == "" {
		orderBy = "updated_at"
	}

	sort.Slice(records, func(i, j int) bool {
		var less bool
		switch orderBy {
		case "ans_name", "ansName":
			less = records[i].ANSName < records[j].ANSName
		case "fqdn":
			less = records[i].FQDN < records[j].FQDN
		case "version":
			less = records[i].Version < records[j].Version
		case "registered_at", "registeredAt":
			less = records[i].RegisteredAt.Before(records[j].RegisteredAt)
		case "updated_at", "updatedAt":
			less = records[i].UpdatedAt.Before(records[j].UpdatedAt)
		default:
			less = records[i].UpdatedAt.Before(records[j].UpdatedAt)
		}
		if desc {
			return !less
		}
		return less
	})
}

// Delete removes a record
func (s *memoryStore) Delete(ctx context.Context, ansName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.records, ansName)
	return nil
}

// List returns all records with pagination
func (s *memoryStore) List(ctx context.Context, opts *ListOptions) (*SearchResult, error) {
	if opts == nil {
		opts = DefaultListOptions()
	}

	return s.Search(ctx, &SearchQuery{
		Limit:     opts.Limit,
		Offset:    opts.Offset,
		OrderBy:   opts.OrderBy,
		OrderDesc: opts.Desc,
	})
}

// Count returns the total number of records
func (s *memoryStore) Count(ctx context.Context) (int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return int64(len(s.records)), nil
}

// Close releases resources
func (s *memoryStore) Close() error {
	return nil
}

// Name returns the provider name
func (s *memoryStore) Name() string {
	return "memory"
}

// Migrate is a no-op for memory store
func (s *memoryStore) Migrate(ctx context.Context) error {
	return nil
}

// ErrStoreFull is returned when the store is at capacity
type ErrStoreFull struct {
	MaxSize int
}

func (e *ErrStoreFull) Error() string {
	return "store is full"
}

func init() {
	Register("memory", func(opts Options, config map[string]interface{}) (Provider, error) {
		return NewMemoryStore(opts, config)
	})
}
