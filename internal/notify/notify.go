// Package notify delivers scan alerts to user-supplied webhooks. Because the
// destination URL is attacker-influenced, every request is SSRF-guarded the same
// way outbound scans are. Slack incoming-webhooks are detected and sent a
// Slack-shaped payload; everything else receives a generic JSON body.
package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/zvdy/parsero-go/internal/safety"
)

type Alert struct {
	Event             string   `json:"event"`
	Target            string   `json:"target"`
	ScanID            string   `json:"scan_id"`
	ScheduleID        string   `json:"schedule_id,omitempty"`
	NewlyReachable    []string `json:"newly_reachable,omitempty"`
	NoLongerReachable []string `json:"no_longer_reachable,omitempty"`
}

// Notifier posts alerts to webhooks. Guard is the per-host SSRF check, injectable
// for tests; it defaults to safety.ResolveAndCheck.
type Notifier struct {
	Client *http.Client
	Guard  func(ctx context.Context, host string) error
}

func New() *Notifier {
	return &Notifier{
		Client: &http.Client{Timeout: 10 * time.Second},
		Guard:  safety.ResolveAndCheck,
	}
}

// Send validates the URL and its host before connecting, so a webhook can't be
// used to reach internal services.
func (n *Notifier) Send(ctx context.Context, webhookURL string, alert Alert) error {
	if webhookURL == "" {
		return nil
	}
	client := n.Client
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	guard := n.Guard
	if guard == nil {
		guard = safety.ResolveAndCheck
	}

	u, err := url.Parse(webhookURL)
	if err != nil {
		return fmt.Errorf("invalid webhook url: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("webhook url must be http(s)")
	}
	if err := guard(ctx, u.Hostname()); err != nil {
		return fmt.Errorf("webhook host rejected: %w", err)
	}

	body, err := json.Marshal(payloadFor(u, alert))
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "parsero/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}
	return nil
}

func payloadFor(u *url.URL, alert Alert) any {
	if strings.Contains(u.Host, "hooks.slack.com") {
		return map[string]string{"text": slackText(alert)}
	}
	return alert
}

func slackText(a Alert) string {
	var b strings.Builder
	fmt.Fprintf(&b, ":rotating_light: *parsero* — `%s`\n", a.Target)
	if len(a.NewlyReachable) > 0 {
		fmt.Fprintf(&b, "*%d newly reachable* Disallow path(s):\n", len(a.NewlyReachable))
		for _, u := range a.NewlyReachable {
			fmt.Fprintf(&b, "• %s\n", u)
		}
	}
	if len(a.NoLongerReachable) > 0 {
		fmt.Fprintf(&b, "_%d no longer reachable_\n", len(a.NoLongerReachable))
	}
	return b.String()
}
