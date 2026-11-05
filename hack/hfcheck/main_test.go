package main

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hermeto/aibox/internal/models"
)

const pinned = "0123456789abcdef0123456789abcdef01234567"

func model(id, repo, rev string, gated bool) models.Model {
	return models.Model{ID: id, License: "apache-2.0", Gated: gated, Source: models.Source{Type: models.SourceHuggingFace, Repo: repo, Revision: rev}}
}

func TestCheck(t *testing.T) {
	c := &models.Catalog{Models: []models.Model{
		model("placeholder", "org/a", models.PlaceholderRevision, false),
		model("good", "org/b", pinned, false),
		model("gone", "org/c", pinned, false),
		model("gated", "org/d", pinned, true),
		{ID: "sim", Source: models.Source{Type: models.SourceSimulator}},
	}}
	get := func(_ context.Context, repo, rev string) (*revisionInfo, int, error) {
		switch repo {
		case "org/a":
			return &revisionInfo{SHA: "fedcba9876543210fedcba9876543210fedcba98"}, 200, nil
		case "org/b":
			info := &revisionInfo{SHA: rev}
			info.CardData.License = "mit"
			return info, 200, nil
		case "org/d":
			return nil, http.StatusUnauthorized, errors.New("401")
		default:
			return nil, http.StatusNotFound, errors.New("404")
		}
	}
	var out bytes.Buffer
	if check(context.Background(), c, get, &out) {
		t.Fatal("placeholder and missing revisions must fail")
	}
	for _, want := range []string{
		`::error::placeholder: revision not pinned → pin revision: "fedcba9876543210fedcba9876543210fedcba98"`,
		"ok      good org/b@" + pinned + ` (model card license "mit", catalog says "apache-2.0")`,
		"::error::gone: revision " + pinned + " of org/c not found",
		"::warning::gated: gated repository org/d",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("missing %q in:\n%s", want, out.String())
		}
	}
	if strings.Contains(out.String(), "sim") {
		t.Error("simulator models have nothing to check")
	}
}

func TestHubLookup(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tok" || r.URL.Path != "/api/models/org/m/revision/main" {
			http.Error(w, "nope", http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(`{"sha": "` + pinned + `", "cardData": {"license": "apache-2.0"}}`))
	}))
	defer srv.Close()
	get := hubLookup(srv.Client(), srv.URL, "tok")
	info, status, err := get(context.Background(), "org/m", "main")
	if err != nil || status != 200 || info.SHA != pinned {
		t.Fatalf("%+v %d %v", info, status, err)
	}
	if _, status, err := get(context.Background(), "org/x", "main"); err == nil || status != http.StatusNotFound {
		t.Fatalf("missing repo: %d %v", status, err)
	}
}
