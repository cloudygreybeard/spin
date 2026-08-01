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
	h.Render(&buf, 60, false)
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
	h.Render(&buf, 60, false)
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
	h.Render(&buf, 60, true)
	output := buf.String()

	if !strings.Contains(output, "\033[") {
		t.Error("overwrite=true should produce ANSI cursor movement sequences")
	}
}

func TestHistogramRenderNoOverwrite(t *testing.T) {
	h := NewHistogram([]string{"x", "y"})
	h.Record("x")

	var buf bytes.Buffer
	h.Render(&buf, 60, false)
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
	h.WriteTSV(&buf)
	output := buf.String()

	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 3 {
		t.Fatalf("WriteTSV produced %d lines, want 3", len(lines))
	}

	// Line format: "item\tcount\tpct%"
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
	h.WriteTSV(&buf)
	output := buf.String()

	if !strings.Contains(output, "75.00%") {
		t.Errorf("expected 75.00%% in output:\n%s", output)
	}
	if !strings.Contains(output, "25.00%") {
		t.Errorf("expected 25.00%% in output:\n%s", output)
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
