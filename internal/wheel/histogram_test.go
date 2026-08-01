// Copyright 2026
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package wheel

import (
	"bytes"
	"strings"
	"testing"
)

func TestHistogramRecord(t *testing.T) {
	h := NewHistogram([]string{"a", "b", "c"})

	h.Record("a")
	h.Record("b")
	h.Record("a")

	if h.Total() != 3 {
		t.Errorf("Total() = %d, want 3", h.Total())
	}
	if h.counts["a"] != 2 {
		t.Errorf("counts[a] = %d, want 2", h.counts["a"])
	}
	if h.counts["b"] != 1 {
		t.Errorf("counts[b] = %d, want 1", h.counts["b"])
	}
	if h.counts["c"] != 0 {
		t.Errorf("counts[c] = %d, want 0", h.counts["c"])
	}
}

func TestHistogramRenderContainsItems(t *testing.T) {
	h := NewHistogram([]string{"alpha", "bravo", "charlie"})
	h.Record("alpha")
	h.Record("bravo")
	h.Record("bravo")
	h.Record("charlie")
	h.Record("charlie")
	h.Record("charlie")

	var buf bytes.Buffer
	if err := h.Render(&buf, 60, false); err != nil {
		t.Fatalf("Render() error: %v", err)
	}
	output := buf.String()

	for _, item := range []string{"alpha", "bravo", "charlie"} {
		if !strings.Contains(output, item) {
			t.Errorf("Render output missing item %q", item)
		}
	}
	if !strings.Contains(output, "trials: 6") {
		t.Errorf("Render output missing trial count; got:\n%s", output)
	}
}

func TestHistogramRenderBarProportions(t *testing.T) {
	h := NewHistogram([]string{"a", "b"})
	for range 100 {
		h.Record("a")
	}
	for range 50 {
		h.Record("b")
	}

	var buf bytes.Buffer
	if err := h.Render(&buf, 60, false); err != nil {
		t.Fatalf("Render() error: %v", err)
	}
	output := buf.String()

	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 lines, got %d", len(lines))
	}

	aBlocks := strings.Count(lines[0], "\u2588")
	bBlocks := strings.Count(lines[1], "\u2588")

	if aBlocks <= bBlocks {
		t.Errorf("item a (count=100) has %d blocks <= item b (count=50) %d blocks",
			aBlocks, bBlocks)
	}
}

func TestHistogramRenderOverwrite(t *testing.T) {
	h := NewHistogram([]string{"x", "y"})
	h.Record("x")

	var buf bytes.Buffer
	if err := h.Render(&buf, 60, true); err != nil {
		t.Fatalf("Render() error: %v", err)
	}
	output := buf.String()

	if !strings.Contains(output, "\033[") {
		t.Error("overwrite=true should produce ANSI cursor movement sequences")
	}
}

func TestHistogramRenderNoOverwrite(t *testing.T) {
	h := NewHistogram([]string{"x", "y"})
	h.Record("x")

	var buf bytes.Buffer
	if err := h.Render(&buf, 60, false); err != nil {
		t.Fatalf("Render() error: %v", err)
	}
	output := buf.String()

	if strings.Contains(output, "\033[") {
		t.Error("overwrite=false should not produce ANSI sequences")
	}
}

func TestHistogramWriteTSV(t *testing.T) {
	h := NewHistogram([]string{"a", "b", "c"})
	h.Record("a")
	h.Record("a")
	h.Record("b")

	var buf bytes.Buffer
	if err := h.WriteTSV(&buf); err != nil {
		t.Fatalf("WriteTSV() error: %v", err)
	}
	output := buf.String()

	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 3 {
		t.Fatalf("WriteTSV produced %d lines, want 3", len(lines))
	}

	if !strings.HasPrefix(lines[0], "a\t2\t") {
		t.Errorf("line 0 = %q, want prefix \"a\\t2\\t\"", lines[0])
	}
	if !strings.HasPrefix(lines[1], "b\t1\t") {
		t.Errorf("line 1 = %q, want prefix \"b\\t1\\t\"", lines[1])
	}
	if !strings.HasPrefix(lines[2], "c\t0\t") {
		t.Errorf("line 2 = %q, want prefix \"c\\t0\\t\"", lines[2])
	}
}

func TestHistogramWriteTSVPercentages(t *testing.T) {
	h := NewHistogram([]string{"a", "b"})
	for range 75 {
		h.Record("a")
	}
	for range 25 {
		h.Record("b")
	}

	var buf bytes.Buffer
	if err := h.WriteTSV(&buf); err != nil {
		t.Fatalf("WriteTSV() error: %v", err)
	}
	output := buf.String()

	if !strings.Contains(output, "75.00%") {
		t.Errorf("expected 75.00%% in output:\n%s", output)
	}
	if !strings.Contains(output, "25.00%") {
		t.Errorf("expected 25.00%% in output:\n%s", output)
	}
}

func TestHistogramSortCountDesc(t *testing.T) {
	h := NewHistogram([]string{"c", "a", "b"})
	for range 10 {
		h.Record("a")
	}
	for range 30 {
		h.Record("b")
	}
	for range 20 {
		h.Record("c")
	}

	h.SetSort(SortCountDesc)
	var buf bytes.Buffer
	if err := h.WriteTSV(&buf); err != nil {
		t.Fatalf("WriteTSV() error: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}
	if !strings.HasPrefix(lines[0], "b\t") {
		t.Errorf("first line should be b (most frequent), got %q", lines[0])
	}
	if !strings.HasPrefix(lines[2], "a\t") {
		t.Errorf("last line should be a (least frequent), got %q", lines[2])
	}
}

func TestHistogramSortCountAsc(t *testing.T) {
	h := NewHistogram([]string{"c", "a", "b"})
	for range 10 {
		h.Record("a")
	}
	for range 30 {
		h.Record("b")
	}
	for range 20 {
		h.Record("c")
	}

	h.SetSort(SortCountAsc)
	var buf bytes.Buffer
	if err := h.WriteTSV(&buf); err != nil {
		t.Fatalf("WriteTSV() error: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if !strings.HasPrefix(lines[0], "a\t") {
		t.Errorf("first line should be a (least frequent), got %q", lines[0])
	}
	if !strings.HasPrefix(lines[2], "b\t") {
		t.Errorf("last line should be b (most frequent), got %q", lines[2])
	}
}

func TestHistogramSortNameAsc(t *testing.T) {
	h := NewHistogram([]string{"cherry", "apple", "banana"})
	h.Record("cherry")
	h.Record("apple")
	h.Record("banana")

	h.SetSort(SortNameAsc)
	var buf bytes.Buffer
	if err := h.WriteTSV(&buf); err != nil {
		t.Fatalf("WriteTSV() error: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if !strings.HasPrefix(lines[0], "apple\t") {
		t.Errorf("first line should be apple, got %q", lines[0])
	}
	if !strings.HasPrefix(lines[2], "cherry\t") {
		t.Errorf("last line should be cherry, got %q", lines[2])
	}
}

func TestHistogramSortNameDesc(t *testing.T) {
	h := NewHistogram([]string{"cherry", "apple", "banana"})
	h.Record("cherry")
	h.Record("apple")
	h.Record("banana")

	h.SetSort(SortNameDesc)
	var buf bytes.Buffer
	if err := h.WriteTSV(&buf); err != nil {
		t.Fatalf("WriteTSV() error: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if !strings.HasPrefix(lines[0], "cherry\t") {
		t.Errorf("first line should be cherry, got %q", lines[0])
	}
	if !strings.HasPrefix(lines[2], "apple\t") {
		t.Errorf("last line should be apple, got %q", lines[2])
	}
}

func TestHistogramSortOriginalPreservesOrder(t *testing.T) {
	h := NewHistogram([]string{"zebra", "apple", "mango"})
	h.Record("apple")
	h.Record("zebra")
	h.Record("mango")

	h.SetSort(SortOriginal)
	var buf bytes.Buffer
	if err := h.WriteTSV(&buf); err != nil {
		t.Fatalf("WriteTSV() error: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if !strings.HasPrefix(lines[0], "zebra\t") {
		t.Errorf("first line should be zebra (original order), got %q", lines[0])
	}
}

func TestParseSortOrder(t *testing.T) {
	tests := []struct {
		input   string
		want    SortOrder
		wantErr bool
	}{
		{"original", SortOriginal, false},
		{"", SortOriginal, false},
		{"count", SortCountDesc, false},
		{"count-desc", SortCountDesc, false},
		{"count-asc", SortCountAsc, false},
		{"name", SortNameAsc, false},
		{"name-asc", SortNameAsc, false},
		{"name-desc", SortNameDesc, false},
		{"invalid", SortOriginal, true},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseSortOrder(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseSortOrder(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("ParseSortOrder(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestSelectReturnsItemFromList(t *testing.T) {
	items := []string{"alpha", "bravo", "charlie"}
	for range 20 {
		winner, err := Select(items)
		if err != nil {
			t.Fatalf("Select() error: %v", err)
		}
		found := false
		for _, item := range items {
			if winner == item {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Select() returned %q, not in item list", winner)
		}
	}
}

func TestSelectEmptyList(t *testing.T) {
	_, err := Select(nil)
	if err == nil {
		t.Fatal("Select(nil) expected error, got nil")
	}
}
