package main

import (
	"strings"
	"testing"
)

func TestSelectWork(t *testing.T) {
	tests := []struct {
		name string
		date string
		due  string
		want bool
	}{
		{"overdue reminder", "2026-09-20", "2026-09-19", true},
		{"due today", "2026-09-20", "2026-09-20", true},
		{"future reminder", "2026-09-20", "2026-09-21", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			items, err := selectWork(DigestRequest{Date: tc.date, Records: []Record{{ID: "shift-42", Kind: "volunteer_reminder", Text: "Call the morning crew", DueDate: tc.due}}})
			if err != nil {
				t.Fatal(err)
			}
			if (len(items) == 1) != tc.want {
				t.Fatalf("selected %d records, want selected=%t", len(items), tc.want)
			}
		})
	}
}

func TestRenderDigestEscapesRecordContent(t *testing.T) {
	page, err := renderDigest("2026-09-20", []WorkItem{{ID: "receipt-1"}}, map[string]string{"receipt-1": "Donation <script>"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(page, "<script>") || !strings.Contains(page, "&lt;script&gt;") {
		t.Fatal("unescaped summary")
	}
}
