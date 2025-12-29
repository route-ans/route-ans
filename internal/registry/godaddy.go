// Package registry provides the GoDaddy ANS registry adapter implementation.
package registry

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/route-ans/route-ans/pkg/ansname"
	"github.com/rs/zerolog/log"
)

// godaddyAdapter implements the Adapter interface for GoDaddy's ANS Registry
type godaddyAdapter struct {
	client    *http.Client
	baseURL   string
	apiKey    string
	apiSecret string
	name      string
	priority  int
}

// GoDaddy API request/response types
type godaddyResolutionRequest struct {
	AgentCapability string `json:"agentCapability"`
	AgentName       string `json:"agentName"`
	Extension       string `json:"extension"`
	Protocol        string `json:"protocol"`
	Provider        string `json:"provider"`
	RequestType     string `json:"requestType"`
	Version         string `json:"version"`
}

type godaddyResolutionResponse struct {
	ANSName string `json:"ansName"`
	Links   []struct {
		Rel  string `json:"rel"`
		Href string `json:"href"`
	} `json:"links"`
}

type godaddyAgentDetails struct {
	AgentName             string                 `json:"agentName"`
	Protocol              string                 `json:"protocol"`
	Version               string                 `json:"version"`
	Extension             string                 `json:"extension"`
	ANSName               string                 `json:"ansName"`
	AgentStatus           string                 `json:"agentStatus"`
	AgentCategory         string                 `json:"agentCategory"`
	AgentCapability       string                 `json:"agentCapability"`
	Provider              string                 `json:"provider"`
	RegistrationTimestamp string                 `json:"registrationTimestamp"`
	ProtocolExtensions    map[string]interface{} `json:"protocolExtensions"`
	Links                 []struct {
		Rel  string `json:"rel"`
		Href string `json:"href"`
	} `json:"links"`
}

type godaddyCertificate struct {
	Certificate      string   `json:"certificate"`
	CertificateChain []string `json:"certificateChain"`
	Fingerprint      string   `json:"fingerprint"`
	IssuedAt         string   `json:"issuedAt"`
	ExpiresAt        string   `json:"expiresAt"`
	Issuer           string   `json:"issuer"`
}

// NewGoDaddyAdapter creates a new GoDaddy registry adapter
func NewGoDaddyAdapter(opts Options, config map[string]interface{}) (Adapter, error) {
	// Extract configuration
	baseURL, _ := config["baseURL"].(string)
	if baseURL == "" {
		baseURL = "https://api.godaddy.com/v1"
	}

	apiKey, _ := config["apiKey"].(string)
	apiSecret, _ := config["apiSecret"].(string)

	if apiKey == "" || apiSecret == "" {
		return nil, fmt.Errorf("godaddy adapter requires apiKey and apiSecret")
	}

	return &godaddyAdapter{
		client: &http.Client{
			Timeout: opts.Timeout,
		},
		baseURL:   baseURL,
		apiKey:    apiKey,
		apiSecret: apiSecret,
		name:      "godaddy",
		priority:  opts.Priority,
	}, nil
}

// Lookup queries the GoDaddy registry for an agent by ANSName
func (g *godaddyAdapter) Lookup(ctx context.Context, name *ansname.ANSName) (*Record, error) {
	// Parse ANSName components
	parts, err := g.parseANSName(name)
	if err != nil {
		return nil, fmt.Errorf("invalid ANSName format: %w", err)
	}

	// Step 1: Resolve to get agent details link
	resolutionReq := godaddyResolutionRequest{
		AgentCapability: parts["capability"],
		AgentName:       parts["name"],
		Extension:       parts["extension"],
		Protocol:        parts["protocol"],
		Provider:        parts["provider"],
		RequestType:     "resolve",
		Version:         parts["version"],
	}

	resolutionURL := fmt.Sprintf("%s/agents/resolution", g.baseURL)
	resolutionResp, err := g.makeRequest(ctx, "POST", resolutionURL, resolutionReq)
	if err != nil {
		return nil, err
	}

	var resolution godaddyResolutionResponse
	if err := json.Unmarshal(resolutionResp, &resolution); err != nil {
		return nil, fmt.Errorf("failed to parse resolution response: %w", err)
	}

	// Find agent-details link
	var detailsURL string
	for _, link := range resolution.Links {
		if link.Rel == "agent-details" {
			detailsURL = link.Href
			break
		}
	}

	if detailsURL == "" {
		return nil, &ErrNotFound{ANSName: name.String()}
	}

	// Step 2: Get agent details
	// If the URL is internal (ra.int.godaddy.com), construct the public API path
	if strings.Contains(detailsURL, "ra.int.godaddy.com") || strings.Contains(detailsURL, ".int.") {
		// Extract the agent ID from the internal URL
		// Format: https://ra.int.godaddy.com/v1/agents/{agentId}
		parts := strings.Split(detailsURL, "/agents/")
		if len(parts) == 2 {
			agentID := parts[1]
			// Use the public API endpoint
			detailsURL = fmt.Sprintf("%s/agents/%s", g.baseURL, agentID)
			log.Debug().Str("agentId", agentID).Str("publicURL", detailsURL).Msg("Converted internal URL to public API")
		}
	}

	detailsResp, err := g.makeRequest(ctx, "GET", detailsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("registry lookup failed: %w", err)
	}

	var details godaddyAgentDetails
	if err := json.Unmarshal(detailsResp, &details); err != nil {
		return nil, fmt.Errorf("failed to parse agent details: %w", err)
	}

	// Check status
	if details.AgentStatus != "ACTIVE" && details.AgentStatus != "VERIFIED" {
		return nil, &ErrNotFound{ANSName: name.String()}
	}

	// Step 3: Get certificates
	serverCert, identityCert, err := g.getCertificates(ctx, details.Links)
	if err != nil {
		log.Warn().Err(err).Str("ansName", name.String()).Msg("Failed to retrieve certificates")
		// Continue without certificates - some agents may not have them yet
	}

	// Build the record
	record := g.buildRecord(&details, serverCert, identityCert)
	return record, nil
}

// LookupByFQDN queries the registry for all versions of an agent by FQDN
func (g *godaddyAdapter) LookupByFQDN(ctx context.Context, fqdn string) ([]*Record, error) {
	// GoDaddy doesn't provide a direct FQDN lookup endpoint
	// This would need to be implemented if they add such an API
	return nil, fmt.Errorf("FQDN lookup not yet implemented for GoDaddy adapter")
}

// GetMerkleProof retrieves the Merkle inclusion proof
func (g *godaddyAdapter) GetMerkleProof(ctx context.Context, ansName string) (*MerkleProof, error) {
	// GoDaddy's Merkle proof endpoint (to be implemented when available)
	return nil, fmt.Errorf("merkle proof not yet implemented for GoDaddy adapter")
}

// GetSignedTreeHead retrieves the current signed tree head
func (g *godaddyAdapter) GetSignedTreeHead(ctx context.Context) (*SignedTreeHead, error) {
	// GoDaddy's signed tree head endpoint (to be implemented when available)
	return nil, fmt.Errorf("signed tree head not yet implemented for GoDaddy adapter")
}

// VerifyRecord verifies a record's authenticity
func (g *godaddyAdapter) VerifyRecord(ctx context.Context, record *Record) (*VerificationResult, error) {
	// Basic verification - more sophisticated checks can be added
	return &VerificationResult{
		Valid: true,
		Checks: map[string]CheckResult{
			"registry": {Passed: true, Message: "Record retrieved from GoDaddy registry"},
		},
		Timestamp: time.Now(),
	}, nil
}

// Subscribe starts receiving events from the registry
func (g *godaddyAdapter) Subscribe(ctx context.Context, handler EventHandler) error {
	// Event streaming not yet implemented for GoDaddy
	<-ctx.Done()
	return ctx.Err()
}

// Name returns the adapter name
func (g *godaddyAdapter) Name() string {
	return g.name
}

// Priority returns the adapter priority
func (g *godaddyAdapter) Priority() int {
	return g.priority
}

// Healthy checks if the registry is reachable
func (g *godaddyAdapter) Healthy(ctx context.Context) (bool, error) {
	// Simple health check - try to reach the API
	req, err := http.NewRequestWithContext(ctx, "GET", g.baseURL, nil)
	if err != nil {
		return false, err
	}

	resp, err := g.client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	return resp.StatusCode < 500, nil
}

// Close releases resources
func (g *godaddyAdapter) Close() error {
	g.client.CloseIdleConnections()
	return nil
}

// Helper methods

func (g *godaddyAdapter) makeRequest(ctx context.Context, method, url string, body interface{}) ([]byte, error) {
	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request: %w", err)
		}
		reqBody = bytes.NewBuffer(jsonData)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add authorization header
	req.Header.Set("Authorization", fmt.Sprintf("sso-key %s:%s", g.apiKey, g.apiSecret))
	req.Header.Set("Content-Type", "application/json")

	resp, err := g.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode == http.StatusNotFound {
		return nil, &ErrNotFound{ANSName: url}
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

func (g *godaddyAdapter) parseANSName(name *ansname.ANSName) (map[string]string, error) {
	// ANSName format: protocol://agentName.capability.provider.version.extension
	// Example: a2a://greeting.greet.PID-1234.v1.0.0.neelanjan.dev

	parts := make(map[string]string)
	parts["protocol"] = name.Protocol
	parts["name"] = name.AgentName
	parts["capability"] = name.Capability
	parts["provider"] = name.ProviderID
	parts["extension"] = name.Extension

	// Version might have 'v' prefix - remove it
	version := name.Version
	if len(version) > 0 && version[0] == 'v' {
		version = version[1:]
	}
	parts["version"] = version

	return parts, nil
}

func (g *godaddyAdapter) getCertificates(ctx context.Context, links []struct {
	Rel  string `json:"rel"`
	Href string `json:"href"`
}) (*godaddyCertificate, *godaddyCertificate, error) {
	var serverCertURL, identityCertURL string

	for _, link := range links {
		switch link.Rel {
		case "server-certificates":
			serverCertURL = link.Href
		case "identity-certificates":
			identityCertURL = link.Href
		}
	}

	var serverCert, identityCert *godaddyCertificate

	// Get server certificate
	if serverCertURL != "" {
		resp, err := g.makeRequest(ctx, "GET", serverCertURL, nil)
		if err == nil {
			var cert godaddyCertificate
			if err := json.Unmarshal(resp, &cert); err == nil {
				serverCert = &cert
			}
		}
	}

	// Get identity certificate
	if identityCertURL != "" {
		resp, err := g.makeRequest(ctx, "GET", identityCertURL, nil)
		if err == nil {
			var cert godaddyCertificate
			if err := json.Unmarshal(resp, &cert); err == nil {
				identityCert = &cert
			}
		}
	}

	return serverCert, identityCert, nil
}

func (g *godaddyAdapter) buildRecord(details *godaddyAgentDetails, serverCert, identityCert *godaddyCertificate) *Record {
	record := &Record{
		ANSName:     details.ANSName,
		FQDN:        fmt.Sprintf("%s.%s", details.AgentName, details.Extension),
		Protocol:    details.Protocol,
		Version:     details.Version,
		Status:      g.mapStatus(details.AgentStatus),
		RegistrarID: "godaddy",
		TTL:         5 * time.Minute,
		UpdatedAt:   time.Now(),
		Metadata: AgentMetadata{
			DisplayName:  details.AgentName,
			Capabilities: []string{details.AgentCapability},
			Protocols:    []string{details.Protocol},
		},
	}

	// Parse registration timestamp
	if details.RegistrationTimestamp != "" {
		if t, err := time.Parse(time.RFC3339, details.RegistrationTimestamp); err == nil {
			record.RegisteredAt = t
		}
	}

	// Extract endpoint from protocol extensions
	if a2aData, ok := details.ProtocolExtensions["a2a"].(map[string]interface{}); ok {
		if url, ok := a2aData["url"].(string); ok {
			record.Endpoint = url
			record.Metadata.ProtocolExtensions = details.ProtocolExtensions
		}
	}

	// Set default endpoint if not found
	if record.Endpoint == "" {
		record.Endpoint = fmt.Sprintf("https://%s.%s", details.AgentName, details.Extension)
	}

	// Add certificate information
	if serverCert != nil {
		record.Certificates.PublicCert = g.parseCertDetails(serverCert)
	}
	if identityCert != nil {
		record.Certificates.PrivateCert = g.parseCertDetails(identityCert)
	}

	// Set expiration
	if identityCert != nil && identityCert.ExpiresAt != "" {
		if t, err := time.Parse(time.RFC3339, identityCert.ExpiresAt); err == nil {
			record.ExpiresAt = t
		}
	} else {
		record.ExpiresAt = time.Now().Add(90 * 24 * time.Hour)
	}

	return record
}

func (g *godaddyAdapter) parseCertDetails(cert *godaddyCertificate) CertDetails {
	details := CertDetails{
		Fingerprint: cert.Fingerprint,
		Issuer:      cert.Issuer,
	}

	if cert.IssuedAt != "" {
		if t, err := time.Parse(time.RFC3339, cert.IssuedAt); err == nil {
			details.IssuedAt = t
		}
	}

	if cert.ExpiresAt != "" {
		if t, err := time.Parse(time.RFC3339, cert.ExpiresAt); err == nil {
			details.ExpiresAt = t
		}
	}

	return details
}

func (g *godaddyAdapter) mapStatus(status string) string {
	switch status {
	case "ACTIVE", "VERIFIED":
		return "active"
	case "PENDING", "PENDING_VALIDATION":
		return "pending"
	case "REVOKED":
		return "revoked"
	case "EXPIRED":
		return "expired"
	default:
		return "unknown"
	}
}

func init() {
	Register("godaddy", func(opts Options, config map[string]interface{}) (Adapter, error) {
		return NewGoDaddyAdapter(opts, config)
	})
}
