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
	"fmt"
	"io"
	"slices"
	"strings"
)

// SortOrder controls the ordering of items in histogram output.
type SortOrder int

const (
	SortOriginal  SortOrder = iota // preserve input order
	SortCountDesc                  // most frequent first
	SortCountAsc                   // least frequent first
	SortNameAsc                    // alphabetical
	SortNameDesc                   // reverse alphabetical
)

// ParseSortOrder converts a flag value to a SortOrder. Valid values are
// "original", "count", "count-desc", "count-asc", "name", "name-asc",
// and "name-desc". The bare forms "count" and "name" default to
// descending and ascending respectively.
func ParseSortOrder(s string) (SortOrder, error) {
	switch s {
	case "original", "":
		return SortOriginal, nil
	case "count", "count-desc":
		return SortCountDesc, nil
	case "count-asc":
		return SortCountAsc, nil
	case "name", "name-asc":
		return SortNameAsc, nil
	case "name-desc":
		return SortNameDesc, nil
	default:
		return SortOriginal, fmt.Errorf("invalid sort order %q: must be original, count[-desc|-asc], or name[-asc|-desc]", s)
	}
}

// Histogram accumulates selection counts and renders a horizontal
// bar chart suitable for displaying Monte Carlo trial results.
type Histogram struct {
	items  []string
	counts map[string]int
	total  int
	sort   SortOrder
}

// NewHistogram creates a histogram for the given items, preserving
// their original order by default.
func NewHistogram(items []string) *Histogram {
	return &Histogram{
		items:  items,
		counts: make(map[string]int, len(items)),
	}
}

// SetSort sets the display ordering for Render and WriteTSV.
func (h *Histogram) SetSort(order SortOrder) {
	h.sort = order
}

// Record increments the count for the given item.
func (h *Histogram) Record(item string) {
	h.counts[item]++
	h.total++
}

// Total returns the number of trials recorded.
func (h *Histogram) Total() int {
	return h.total
}

// merge adds the counts from a worker's local frequency map into the
// histogram. This is the fan-in step of the parallel Monte Carlo.
func (h *Histogram) merge(counts map[string]int) {
	for item, c := range counts {
		h.counts[item] += c
		h.total += c
	}
}

// orderedItems returns the items in the current sort order.
func (h *Histogram) orderedItems() []string {
	ordered := make([]string, len(h.items))
	copy(ordered, h.items)

	switch h.sort {
	case SortCountDesc:
		slices.SortStableFunc(ordered, func(a, b string) int {
			return h.counts[b] - h.counts[a]
		})
	case SortCountAsc:
		slices.SortStableFunc(ordered, func(a, b string) int {
			return h.counts[a] - h.counts[b]
		})
	case SortNameAsc:
		slices.Sort(ordered)
	case SortNameDesc:
		slices.SortFunc(ordered, func(a, b string) int {
			return strings.Compare(b, a)
		})
	}

	return ordered
}

// Render draws the histogram as a horizontal bar chart to w. The width
// parameter controls the total line width (set to 60 if unknown). The
// chart is preceded by enough ANSI cursor-up sequences to overwrite a
// previous rendering of the same histogram, enabling live updates.
func (h *Histogram) Render(w io.Writer, width int, overwrite bool) error {
	if width < 30 {
		width = 60
	}

	items := h.orderedItems()
	labelWidth := maxItemLen(items)
	barBudget := width - labelWidth - 21
	if barBudget < 10 {
		barBudget = 10
	}

	maxCount := 0
	for _, item := range items {
		if c := h.counts[item]; c > maxCount {
			maxCount = c
		}
	}

	lines := len(items) + 1
	if overwrite {
		if _, err := fmt.Fprintf(w, "\033[%dA", lines); err != nil {
			return err
		}
	}

	for _, item := range items {
		c := h.counts[item]
		barLen := 0
		if maxCount > 0 {
			barLen = c * barBudget / maxCount
		}
		pct := 0.0
		if h.total > 0 {
			pct = 100.0 * float64(c) / float64(h.total)
		}
		bar := strings.Repeat("\u2588", barLen)
		if _, err := fmt.Fprintf(w, "  %-*s |%-*s %5d (%5.1f%%)\n",
			labelWidth, item, barBudget, bar, c, pct); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintf(w, "  trials: %d\n", h.total)
	return err
}

// WriteTSV writes the final frequency table to w in tab-separated
// format: item, count, percentage. The output respects the current
// sort order. Suitable for piping to other tools.
func (h *Histogram) WriteTSV(w io.Writer) error {
	for _, item := range h.orderedItems() {
		c := h.counts[item]
		pct := 0.0
		if h.total > 0 {
			pct = 100.0 * float64(c) / float64(h.total)
		}
		if _, err := fmt.Fprintf(w, "%s\t%d\t%.2f%%\n", item, c, pct); err != nil {
			return err
		}
	}
	return nil
}
