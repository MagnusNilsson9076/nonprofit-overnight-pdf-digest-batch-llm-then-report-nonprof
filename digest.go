package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"net/http"
	"sort"
	"strings"
	"time"
)

type Record struct {
	ID      string `json:"id"`
	Kind    string `json:"kind"`
	Text    string `json:"text"`
	DueDate string `json:"due_date"`
}

type DigestRequest struct {
	Date    string   `json:"date"`
	Records []Record `json:"records"`
}

type WorkItem struct {
	ID     string
	Prompt string
}

// selectWork makes the operational decision before any billable request.
func selectWork(input DigestRequest) ([]WorkItem, error) {
	if _, err := time.Parse("2006-01-02", input.Date); err != nil {
		return nil, errors.New("date must be YYYY-MM-DD")
	}
	seen := make(map[string]bool)
	var work []WorkItem
	for _, r := range input.Records {
		if r.ID == "" || seen[r.ID] {
			return nil, errors.New("record IDs must be nonempty and unique")
		}
		seen[r.ID] = true
		if strings.TrimSpace(r.Text) == "" {
			continue
		}
		switch r.Kind {
		case "donor_receipt", "campaign_report":
		case "volunteer_reminder":
			if r.DueDate == "" || r.DueDate > input.Date {
				continue
			}
		default:
			return nil, fmt.Errorf("unknown record kind %q", r.Kind)
		}
		work = append(work, WorkItem{ID: r.ID, Prompt: "Summarise this " + r.Kind + " for an internal nonprofit digest. Preserve amounts, dates, and action owners. Do not invent details.\n" + r.Text})
	}
	sort.Slice(work, func(i, j int) bool { return work[i].ID < work[j].ID })
	return work, nil
}

func renderDigest(date string, work []WorkItem, summaries map[string]string) (string, error) {
	var b strings.Builder
	b.WriteString("<!doctype html><html><head><meta charset=\"utf-8\"></head><body><h1>Nonprofit digest: " + html.EscapeString(date) + "</h1>")
	for _, item := range work {
		text, ok := summaries[item.ID]
		if !ok || strings.TrimSpace(text) == "" {
			return "", fmt.Errorf("missing summary for %s", item.ID)
		}
		b.WriteString("<section><h2>" + html.EscapeString(item.ID) + "</h2><p>" + html.EscapeString(text) + "</p></section>")
	}
	b.WriteString("</body></html>")
	return b.String(), nil
}

type digestAPI interface {
	Summarise(context.Context, string, []WorkItem) (map[string]string, error)
	PDF(context.Context, string, string) (string, error)
}

func digestHandler(api digestAPI) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			http.Error(w, "POST required", http.StatusMethodNotAllowed)
			return
		}
		var input DigestRequest
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		work, err := selectWork(input)
		if err != nil || len(work) == 0 {
			http.Error(w, "no eligible records or invalid input", http.StatusBadRequest)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 45*time.Minute)
		defer cancel()
		summaries, err := api.Summarise(ctx, input.Date, work)
		if err == nil {
			var page string
			page, err = renderDigest(input.Date, work, summaries)
			if err == nil {
				var url string
				url, err = api.PDF(ctx, input.Date, page)
				if err == nil {
					w.Header().Set("Content-Type", "application/json")
					_ = json.NewEncoder(w).Encode(map[string]string{"pdf_url": url})
					return
				}
			}
		}
		var rejected *APIError
		if errors.As(err, &rejected) && rejected.Status >= 400 && rejected.Status < 500 {
			http.Error(w, rejected.Error(), rejected.Status)
			return
		}
		http.Error(w, "digest processing failed: "+err.Error(), http.StatusBadGateway)
	})
}
