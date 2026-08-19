package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"
)

const apiBase = "https://api.tailscale.com/api/v2"

// Matches any connector tag, not just today's tag:hbo-connector/tag:netflix-connector,
// so a future third app connector doesn't require a code change here. See ADR 0006.
var connectorTagPattern = regexp.MustCompile(`^tag:.*-connector$`)

var httpClient = &http.Client{Timeout: 30 * time.Second}

type device struct {
	ID   string   `json:"id"`
	Name string   `json:"name"`
	Tags []string `json:"tags"`
}

type devicesResponse struct {
	Devices []device `json:"devices"`
}

type routesResponse struct {
	AdvertisedRoutes []string `json:"advertisedRoutes"`
	EnabledRoutes    []string `json:"enabledRoutes"`
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
}

func main() {
	dryRun := flag.Bool("dry-run", false, "compute and log the intended change without disabling any routes")
	flag.Parse()

	clientID := requireEnv("TS_OAUTH_CLIENT_ID")
	clientSecret := requireEnv("TS_OAUTH_CLIENT_SECRET")
	tailnet := envOrDefault("TS_TAILNET", "-")

	token, err := fetchAccessToken(clientID, clientSecret)
	if err != nil {
		log.Fatalf("fetching OAuth access token: %v", err)
	}

	devices, err := listDevices(token, tailnet)
	if err != nil {
		log.Fatalf("listing tailnet devices: %v", err)
	}

	targets := filterConnectorDevices(devices)
	if len(targets) == 0 {
		log.Printf("no tag:*-connector devices found, nothing to prune")
		return
	}
	log.Printf("found %d connector device(s) to prune", len(targets))

	// Best-effort only: audit-log timestamps are logged purely so real
	// route-churn data accumulates for a future revisit of the selection
	// algorithm (ADR 0006). A fetch failure here must never block pruning.
	auditLog, err := fetchAuditLog(token, tailnet)
	if err != nil {
		log.Printf("warning: fetching audit log (non-fatal, continuing without approval timestamps): %v", err)
		auditLog = ""
	}

	failed := false
	for _, d := range targets {
		if err := pruneDevice(token, d, auditLog, *dryRun); err != nil {
			log.Printf("error pruning device %s (%s): %v", d.Name, d.ID, err)
			failed = true
			continue
		}
	}

	if failed {
		os.Exit(1)
	}
}

func pruneDevice(token string, d device, auditLog string, dryRun bool) error {
	routes, err := getDeviceRoutes(token, d.ID)
	if err != nil {
		return fmt.Errorf("getting current routes: %w", err)
	}

	enabled := append([]string(nil), routes.EnabledRoutes...)
	sort.Strings(enabled)

	disableCount := len(enabled) / 2
	toDisable := enabled[:disableCount]
	toKeep := enabled[disableCount:]

	log.Printf("device %s (%s): %d routes enabled, disabling %d (capped at 50%%)", d.Name, d.ID, len(enabled), disableCount)
	for _, r := range toDisable {
		if ts, ok := lastApprovalTime(auditLog, r); ok {
			log.Printf("  disabling %s (last approved %s)", r, ts)
		} else {
			log.Printf("  disabling %s (no approval event in trailing 90 days)", r)
		}
	}

	if disableCount == 0 {
		log.Printf("device %s (%s): nothing to disable this run", d.Name, d.ID)
		return nil
	}

	if dryRun {
		log.Printf("device %s (%s): dry-run, not calling the API", d.Name, d.ID)
		return nil
	}

	if err := setDeviceRoutes(token, d.ID, toKeep); err != nil {
		return fmt.Errorf("disabling routes: %w", err)
	}
	log.Printf("device %s (%s): now %d routes enabled", d.Name, d.ID, len(toKeep))
	return nil
}

func fetchAccessToken(clientID, clientSecret string) (string, error) {
	form := url.Values{
		"client_id":     {clientID},
		"client_secret": {clientSecret},
	}
	req, err := http.NewRequest(http.MethodPost, apiBase+"/oauth/token", strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	body, err := do(req)
	if err != nil {
		return "", err
	}
	var tok tokenResponse
	if err := json.Unmarshal(body, &tok); err != nil {
		return "", fmt.Errorf("decoding token response: %w", err)
	}
	if tok.AccessToken == "" {
		return "", fmt.Errorf("token response had no access_token")
	}
	return tok.AccessToken, nil
}

func listDevices(token, tailnet string) ([]device, error) {
	req, err := http.NewRequest(http.MethodGet, apiBase+"/tailnet/"+tailnet+"/devices", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	body, err := do(req)
	if err != nil {
		return nil, err
	}
	var resp devicesResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decoding devices response: %w", err)
	}
	return resp.Devices, nil
}

func filterConnectorDevices(devices []device) []device {
	var out []device
	for _, d := range devices {
		for _, t := range d.Tags {
			if connectorTagPattern.MatchString(t) {
				out = append(out, d)
				break
			}
		}
	}
	return out
}

func getDeviceRoutes(token, deviceID string) (*routesResponse, error) {
	req, err := http.NewRequest(http.MethodGet, apiBase+"/device/"+deviceID+"/routes", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	body, err := do(req)
	if err != nil {
		return nil, err
	}
	var resp routesResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decoding routes response: %w", err)
	}
	return &resp, nil
}

func setDeviceRoutes(token, deviceID string, routes []string) error {
	if routes == nil {
		routes = []string{}
	}
	payload, err := json.Marshal(map[string][]string{"routes": routes})
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, apiBase+"/device/"+deviceID+"/routes", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	_, err = do(req)
	return err
}

// fetchAuditLog returns the raw JSON body of the trailing-90-day Configuration
// Audit Log. The exact response schema is not pinned to typed structs here —
// this is a best-effort, log-only signal (see ADR 0006), so entries are
// matched by substring below rather than parsed strictly.
func fetchAuditLog(token, tailnet string) (string, error) {
	end := time.Now().UTC()
	start := end.AddDate(0, 0, -90)
	q := url.Values{
		"start": {start.Format(time.RFC3339)},
		"end":   {end.Format(time.RFC3339)},
	}
	req, err := http.NewRequest(http.MethodGet, apiBase+"/tailnet/"+tailnet+"/logging/configuration?"+q.Encode(), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	body, err := do(req)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

// lastApprovalTime does a best-effort scan of the raw audit log body for an
// entry mentioning the given CIDR and returns a human-readable marker. This
// is intentionally crude (see fetchAuditLog) — it's a visibility aid, not a
// decision input.
func lastApprovalTime(auditLog, cidr string) (string, bool) {
	if auditLog == "" || !strings.Contains(auditLog, cidr) {
		return "", false
	}
	return "within trailing 90 days (see audit log for exact time)", true
}

func do(req *http.Request) ([]byte, error) {
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%s %s: unexpected status %d: %s", req.Method, req.URL.Path, resp.StatusCode, string(body))
	}
	return body, nil
}

func requireEnv(name string) string {
	v := os.Getenv(name)
	if v == "" {
		log.Fatalf("required environment variable %s is not set", name)
	}
	return v
}

func envOrDefault(name, def string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return def
}
