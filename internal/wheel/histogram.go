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
	"strings"
)

// Histogram accumulates selection counts and renders a horizontal
// bar chart suitable for displaying Monte Carlo trial results.
type Histogram struct {
	items  []string
	counts map[string]int
	total  int
}

// NewHistogram creates a histogram for the given items, preserving
// their original order.
func NewHistogram(items []string) *Histogram {
	return &Histogram{
		items:  items,
		counts: make(map[string]int, len(items)),
	}
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

// Render draws the histogram as a horizontal bar chart to w. The width
// parameter controls the total line width (set to 60 if unknown). The
// chart is preceded by enough ANSI "cursor up" sequences to overwrite
// a previous rendering of the same histogram, enabling live updates.
func (h *Histogram) Render(w io.Writer, width int, overwrite bool) {
	if width < 30 {
		width = 60
	}

	labelWidth := maxItemLen(h.items)
	// Layout: "  label  |bars  count (pct%)"
	// Reserve space: 2 + labelWidth + 3 ("|") + count/pct (~16 chars)
	barBudget := width - labelWidth - 21
	if barBudget < 10 {
		barBudget = 10
	}

	maxCount := 0
	for _, item := range h.items {
		if c := h.counts[item]; c > maxCount {
			maxCount = c
		}
	}

	lines := len(h.items) + 1 // items + summary line
	if overwrite {
		// Move cursor up to overwrite the previous rendering.
		fmt.Fprintf(w, "\033[%dA", lines)
	}

	for _, item := range h.items {
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
		fmt.Fprintf(w, "  %-*s |%-*s %5d (%5.1f%%)\n",
			labelWidth, item, barBudget, bar, c, pct)
	}
	fmt.Fprintf(w, "  trials: %d\n", h.total)
}

// WriteTSV writes the final frequency table to w in tab-separated
// format: item, count, percentage. Suitable for piping to other tools.
func (h *Histogram) WriteTSV(w io.Writer) {
	for _, item := range h.items {
		c := h.counts[item]
		pct := 0.0
		if h.total > 0 {
			pct = 100.0 * float64(c) / float64(h.total)
		}
		fmt.Fprintf(w, "%s\t%d\t%.2f%%\n", item, c, pct)
	}
}
