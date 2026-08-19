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

// The Configuration Audit Log endpoint is observably slower than the rest of
// the API on a tailnet with heavy route churn (a 30s timeout wasn't enough in
// practice). It's a best-effort, log-only call on a monthly job with no time
// pressure, so it gets a much longer budget rather than a guessed-tight one.
var auditLogClient = &http.Client{Timeout: 2 * time.Minute}

type device struct {
	ID   string   `json:"id"`
	Name string   `json:"name"`
	Tags []string `json:"tags"`
	// NodeID is a distinct identifier space from ID: ID is the numeric
	// device ID used by the routes endpoints, NodeID (e.g. "nRJvHL7Yi611CNTRL")
	// is what the audit log's target.id actually contains. Confirmed by
	// inspecting a live device object -- the two are not interchangeable.
	NodeID string `json:"nodeId"`
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

// auditLogResponse mirrors the real shape of GET /tailnet/{tailnet}/logging/configuration,
// which is a list of full route-list snapshots (Old/New), not per-CIDR diffs —
// confirmed by inspecting a live response; ADR 0004's assumption of a
// "specific CIDR diff" per event doesn't hold. See buildRouteHistory.
type auditLogResponse struct {
	Logs []auditEvent `json:"logs"`
}

// Old/New are deliberately json.RawMessage: the audit log mixes many event
// types (LOGIN, APPROVE, CREATE, ...) whose old/new payloads aren't route
// lists at all (e.g. objects, not arrays) — decoding straight into []string
// fails the whole batch on the first mismatch. Only AUTO_APPROVED_ROUTES
// events get decoded further, in buildRouteHistory.
type auditEvent struct {
	EventTime time.Time `json:"eventTime"`
	Target    struct {
		ID       string `json:"id"`
		Property string `json:"property"`
	} `json:"target"`
	Old json.RawMessage `json:"old"`
	New json.RawMessage `json:"new"`
}

func routeList(raw json.RawMessage) []string {
	var routes []string
	if err := json.Unmarshal(raw, &routes); err != nil {
		return nil
	}
	return routes
}

// routeHistory is the per-device result of buildRouteHistory: firstSeen gives
// a real approval timestamp for routes that entered the enabled set within
// the trailing 90-day audit window; predatesWindow marks routes that were
// already enabled at the start of that window (so they're at least that old,
// but no more precise than that — the window doesn't reach far enough back to
// say by how much, especially for a connector with 292+ days of accumulated
// routes).
type routeHistory struct {
	firstSeen      map[string]time.Time
	predatesWindow map[string]bool
}

func buildRouteHistory(events []auditEvent, deviceID string) routeHistory {
	h := routeHistory{firstSeen: map[string]time.Time{}, predatesWindow: map[string]bool{}}

	var relevant []auditEvent
	for _, e := range events {
		if e.Target.ID == deviceID && e.Target.Property == "AUTO_APPROVED_ROUTES" {
			relevant = append(relevant, e)
		}
	}
	sort.Slice(relevant, func(i, j int) bool { return relevant[i].EventTime.Before(relevant[j].EventTime) })

	if len(relevant) == 0 {
		return h
	}
	for _, r := range routeList(relevant[0].Old) {
		h.predatesWindow[r] = true
	}
	for _, e := range relevant {
		for _, r := range routeList(e.New) {
			if h.predatesWindow[r] {
				continue
			}
			if _, seen := h.firstSeen[r]; !seen {
				h.firstSeen[r] = e.EventTime
			}
		}
	}
	return h
}

// rankOldestFirst orders enabled routes for the pruning cap: routes with no
// precise age data (predates the 90-day audit window, or has no matching
// event at all) are treated as the oldest tier and sort first, since there's
// no positive evidence they're recent; routes with a real first-seen
// timestamp follow, oldest timestamp first. Both tiers break ties on the CIDR
// string for a stable, reproducible order run to run.
func rankOldestFirst(routes []string, h routeHistory) []string {
	ranked := append([]string(nil), routes...)
	sort.Slice(ranked, func(i, j int) bool {
		a, b := ranked[i], ranked[j]
		aTime, aKnown := h.firstSeen[a]
		bTime, bKnown := h.firstSeen[b]
		if aKnown != bKnown {
			return !aKnown // unknown age (predates window / no data) sorts first
		}
		if aKnown && !aTime.Equal(bTime) {
			return aTime.Before(bTime)
		}
		return a < b
	})
	return ranked
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
	auditEvents, err := fetchAuditLog(token, tailnet)
	if err != nil {
		log.Printf("warning: fetching audit log (non-fatal, continuing without approval timestamps): %v", err)
		auditEvents = nil
	}

	failed := false
	for _, d := range targets {
		if err := pruneDevice(token, d, auditEvents, *dryRun); err != nil {
			log.Printf("error pruning device %s (%s): %v", d.Name, d.ID, err)
			failed = true
			continue
		}
	}

	if failed {
		os.Exit(1)
	}
}

func pruneDevice(token string, d device, auditEvents []auditEvent, dryRun bool) error {
	routes, err := getDeviceRoutes(token, d.ID)
	if err != nil {
		return fmt.Errorf("getting current routes: %w", err)
	}

	history := buildRouteHistory(auditEvents, d.NodeID)
	enabled := rankOldestFirst(routes.EnabledRoutes, history)

	disableCount := len(enabled) / 2
	toDisable := enabled[:disableCount]
	toKeep := enabled[disableCount:]

	log.Printf("device %s (%s): %d routes enabled, disabling %d oldest-first (capped at 50%%)", d.Name, d.ID, len(enabled), disableCount)
	for _, r := range toDisable {
		switch {
		case history.predatesWindow[r]:
			log.Printf("  disabling %s (already enabled before the 90-day audit window started; older than that)", r)
		case !history.firstSeen[r].IsZero():
			age := time.Since(history.firstSeen[r]).Round(time.Hour)
			log.Printf("  disabling %s (first approved %s, %s ago)", r, history.firstSeen[r].Format(time.RFC3339), age)
		default:
			log.Printf("  disabling %s (no matching AUTO_APPROVED_ROUTES event found)", r)
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

	body, err := do(httpClient, req)
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

	body, err := do(httpClient, req)
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

	body, err := do(httpClient, req)
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

	_, err = do(httpClient, req)
	return err
}

// fetchAuditLog returns the trailing-90-day Configuration Audit Log, used
// purely to log real approval-recency data (see buildRouteHistory) — never to
// gate which routes get disabled (ADR 0006).
func fetchAuditLog(token, tailnet string) ([]auditEvent, error) {
	end := time.Now().UTC()
	start := end.AddDate(0, 0, -90)
	q := url.Values{
		"start": {start.Format(time.RFC3339)},
		"end":   {end.Format(time.RFC3339)},
	}
	req, err := http.NewRequest(http.MethodGet, apiBase+"/tailnet/"+tailnet+"/logging/configuration?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	body, err := do(auditLogClient, req)
	if err != nil {
		return nil, err
	}
	var resp auditLogResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decoding audit log response: %w", err)
	}
	return resp.Logs, nil
}

func do(client *http.Client, req *http.Request) ([]byte, error) {
	resp, err := client.Do(req)
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
