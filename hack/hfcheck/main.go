// Command hfcheck verifies that every Hugging Face revision in the model catalog still exists
// and, for placeholder revisions (40 zeros), prints the commit to pin (the current head of
// main). It runs in CI because the build container cannot reach Hugging Face; HF_TOKEN is
// used when set (gated repositories).
//
// Usage: go run ./hack/hfcheck [-catalog models/catalog.yaml]
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/hermeto/aibox/internal/models"
)

type revisionInfo struct {
	SHA      string `json:"sha"`
	CardData struct {
		License any `json:"license"`
	} `json:"cardData"`
}

// lookup returns the commit and license of repo at rev.
type lookup func(ctx context.Context, repo, rev string) (*revisionInfo, int, error)

func main() {
	path := flag.String("catalog", models.DefaultPath(), "model catalog")
	endpoint := flag.String("endpoint", "https://huggingface.co", "Hugging Face endpoint")
	flag.Parse()
	c, err := models.Load(*path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	client := &http.Client{Timeout: 30 * time.Second}
	ok := check(context.Background(), c, hubLookup(client, *endpoint, os.Getenv("HF_TOKEN")), os.Stdout)
	if !ok {
		os.Exit(1)
	}
}

func hubLookup(client *http.Client, endpoint, token string) lookup {
	return func(ctx context.Context, repo, rev string) (*revisionInfo, int, error) {
		u := endpoint + "/api/models/" + repo + "/revision/" + url.PathEscape(rev)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return nil, 0, err
		}
		req.Header.Set("User-Agent", "aibox-hfcheck")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		resp, err := client.Do(req)
		if err != nil {
			return nil, 0, err
		}
		defer func() { _ = resp.Body.Close() }()
		body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
		if err != nil {
			return nil, resp.StatusCode, err
		}
		if resp.StatusCode != http.StatusOK {
			return nil, resp.StatusCode, fmt.Errorf("GET %s: %s", u, resp.Status)
		}
		var info revisionInfo
		if err := json.Unmarshal(body, &info); err != nil {
			return nil, resp.StatusCode, fmt.Errorf("GET %s: %w", u, err)
		}
		return &info, resp.StatusCode, nil
	}
}

// check prints one line per Hugging Face model and reports whether all revisions are pinned
// and resolvable. Errors use the GitHub Actions annotation format.
func check(ctx context.Context, c *models.Catalog, get lookup, w io.Writer) bool {
	ok := true
	for i := range c.Models {
		m := &c.Models[i]
		if !m.Mirrorable() {
			continue
		}
		if !m.Pinned() {
			ok = false
			info, _, err := get(ctx, m.Source.Repo, "main")
			if err != nil {
				_, _ = fmt.Fprintf(w, "::error::%s: revision not pinned and main of %s could not be resolved: %v\n", m.ID, m.Source.Repo, err)
				continue
			}
			_, _ = fmt.Fprintf(w, "::error::%s: revision not pinned → pin revision: \"%s\" (main of %s)\n", m.ID, info.SHA, m.Source.Repo)
			continue
		}
		info, status, err := get(ctx, m.Source.Repo, m.Source.Revision)
		switch {
		case err != nil && (status == http.StatusUnauthorized || status == http.StatusForbidden) && m.Gated:
			_, _ = fmt.Fprintf(w, "::warning::%s: gated repository %s not readable without HF_TOKEN; revision not verified\n", m.ID, m.Source.Repo)
		case err != nil:
			ok = false
			_, _ = fmt.Fprintf(w, "::error::%s: revision %s of %s not found: %v\n", m.ID, m.Source.Revision, m.Source.Repo, err)
		case info.SHA != m.Source.Revision:
			ok = false
			_, _ = fmt.Fprintf(w, "::error::%s: %s resolved to %s, want the commit itself\n", m.ID, m.Source.Revision, info.SHA)
		default:
			_, _ = fmt.Fprintf(w, "ok      %s %s@%s%s\n", m.ID, m.Source.Repo, m.Source.Revision, licenseNote(m, info))
		}
	}
	return ok
}

// licenseNote flags a catalog license that differs from the model card.
func licenseNote(m *models.Model, info *revisionInfo) string {
	card, _ := info.CardData.License.(string)
	if card == "" || strings.EqualFold(card, m.License) {
		return ""
	}
	return fmt.Sprintf(" (model card license %q, catalog says %q)", card, m.License)
}
