// Package api is the HTTP client the ruust CLI uses to talk to the Ruust
// control plane. It decodes the shared JSON contract into Go structs and turns
// non-2xx responses (which carry an { "error": ... } body) into Go errors.
//
// Customer-facing vocabulary is used throughout: the unit a customer deploys is
// an Egg (never a Blob), and its lifecycle states are incubating, hatching,
// hatched, cold, and cracked.
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/RuustRun/cli/internal/config"
)

// Egg is a deployed unit a customer owns and pays for.
type Egg struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Region      string  `json:"region"`      // hyphenated slug, e.g. "eu-west"
	RegionLabel string  `json:"regionLabel"` // human label, e.g. "London"
	Tier        string  `json:"tier"`
	TierLabel   string  `json:"tierLabel"`
	PriceGbp    float64 `json:"priceGbp"`
	State       string  `json:"state"` // incubating|hatching|hatched|cold|cracked
	URL         string  `json:"url"`   // host only, no scheme
	Repo        string  `json:"repo"`  // git url
	RAMMb       float64 `json:"ramMb"`
	CreatedAt   string  `json:"createdAt"` // ISO 8601, UTC
}

// Domain is a hostname attached to an Egg.
type Domain struct {
	Hostname   string `json:"hostname"`
	IsCustom   bool   `json:"isCustom"`
	CertStatus string `json:"certStatus"`
}

// Deployment describes the most recent deploy attempt for an Egg.
type Deployment struct {
	GitSha     string  `json:"gitSha"`
	Status     string  `json:"status"`
	ImageRef   *string `json:"imageRef"`
	FinishedAt *string `json:"finishedAt"`
}

// EggDetail is an Egg plus its domains, env keys, and latest deployment.
type EggDetail struct {
	Egg
	Domains    []Domain    `json:"domains"`
	EnvKeys    []string    `json:"envKeys"`
	Deployment *Deployment `json:"deployment"`
}

// Region is a deployment region and its current availability.
type Region struct {
	Slug         string `json:"slug"` // hyphenated, e.g. "eu-west"
	DisplayName  string `json:"displayName"`
	Availability string `json:"availability"` // "live" | "soon"
}

// Me is the currently signed-in account.
type Me struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

// LoginResponse is returned by the login endpoint and carries a session token.
type LoginResponse struct {
	Token     string `json:"token"`
	ExpiresAt string `json:"expiresAt"` // ISO 8601, UTC
	Email     string `json:"email"`
}

// LogLine is a single structured log line for an Egg.
type LogLine struct {
	TS    string `json:"ts"`
	Level string `json:"level"`
	Text  string `json:"text"`
}

// LogsResponse is a batch of log lines.
type LogsResponse struct {
	Lines []LogLine `json:"lines"`
}

// errorBody is the shape of a non-2xx response.
type errorBody struct {
	Error string `json:"error"`
}

// Client talks to the Ruust control plane over HTTP.
type Client struct {
	Host  string // base URL, e.g. "http://localhost:3939"
	Token string // Ruust session id, sent as a Bearer token
	HTTP  *http.Client
}

// New builds a client for the given host and token.
func New(host, token string) *Client {
	return &Client{
		Host:  strings.TrimRight(host, "/"),
		Token: token,
		HTTP:  &http.Client{Timeout: 30 * time.Second},
	}
}

// NewFromConfig builds a client from a loaded config, applying the RUUST_HOST
// and RUUST_TOKEN environment overrides.
func NewFromConfig(c *config.Config) *Client {
	return New(config.Host(c), config.Token(c))
}

// do performs a request against path, encoding body as JSON when non-nil and
// decoding the response into out. A non-2xx status becomes a Go error carrying
// the server's { "error": ... } message when present.
func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	var reader io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encoding request: %w", err)
		}
		reader = bytes.NewReader(buf)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.Host+path, reader)
	if err != nil {
		return fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return apiError(resp.StatusCode, data)
	}

	if out != nil && len(data) > 0 {
		if err := json.Unmarshal(data, out); err != nil {
			return fmt.Errorf("decoding response: %w", err)
		}
	}
	return nil
}

// apiError turns a non-2xx response into a Go error, preferring the server's
// { "error": ... } message and falling back to the status text.
func apiError(status int, data []byte) error {
	var eb errorBody
	if err := json.Unmarshal(data, &eb); err == nil && eb.Error != "" {
		if status == http.StatusUnauthorized {
			return fmt.Errorf("%s (run 'ruust login')", eb.Error)
		}
		return fmt.Errorf("%s", eb.Error)
	}
	if status == http.StatusUnauthorized {
		return fmt.Errorf("not signed in (run 'ruust login')")
	}
	return fmt.Errorf("request failed: %s", http.StatusText(status))
}

// loginRequest is the body posted to the login endpoint.
type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// Login exchanges an email and password for a session token. It does not
// require an existing token.
func (c *Client) Login(email, password string) (LoginResponse, error) {
	var out LoginResponse
	err := c.do(context.Background(), http.MethodPost, "/api/v1/auth/login",
		loginRequest{Email: email, Password: password}, &out)
	return out, err
}

// Me returns the currently signed-in account.
func (c *Client) Me() (Me, error) {
	var out Me
	err := c.do(context.Background(), http.MethodGet, "/api/v1/me", nil, &out)
	return out, err
}

// ListEggs returns all Eggs the signed-in account owns.
func (c *Client) ListEggs() ([]Egg, error) {
	var out struct {
		Eggs []Egg `json:"eggs"`
	}
	err := c.do(context.Background(), http.MethodGet, "/api/v1/eggs", nil, &out)
	return out.Eggs, err
}

// GetEgg returns the full detail for a single Egg by id.
func (c *Client) GetEgg(id string) (EggDetail, error) {
	var out EggDetail
	err := c.do(context.Background(), http.MethodGet, "/api/v1/eggs/"+id, nil, &out)
	return out, err
}

// createEggRequest is the body posted to create an Egg.
type createEggRequest struct {
	Repo   string `json:"repo"`
	Branch string `json:"branch"`
	Region string `json:"region"`
	Tier   string `json:"tier"`
}

// CreateEgg provisions a new Egg from a git repo, branch, region, and tier.
func (c *Client) CreateEgg(repo, branch, region, tier string) (Egg, error) {
	var out Egg
	err := c.do(context.Background(), http.MethodPost, "/api/v1/eggs",
		createEggRequest{Repo: repo, Branch: branch, Region: region, Tier: tier}, &out)
	return out, err
}

// Logs returns a batch of log lines for an Egg.
func (c *Client) Logs(id string) (LogsResponse, error) {
	var out LogsResponse
	err := c.do(context.Background(), http.MethodGet, "/api/v1/eggs/"+id+"/logs", nil, &out)
	return out, err
}

// Regions returns the deployment regions and their availability.
func (c *Client) Regions() ([]Region, error) {
	var out struct {
		Regions []Region `json:"regions"`
	}
	err := c.do(context.Background(), http.MethodGet, "/api/v1/regions", nil, &out)
	return out.Regions, err
}
