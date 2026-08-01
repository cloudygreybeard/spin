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
	"testing"
)

func TestMonteCarloTotalCount(t *testing.T) {
	items := []string{"a", "b", "c"}
	hist, err := MonteCarlo(items, 1000, nil)
	if err != nil {
		t.Fatalf("MonteCarlo() error: %v", err)
	}
	if hist.Total() != 1000 {
		t.Errorf("Total() = %d, want 1000", hist.Total())
	}
}

func TestMonteCarloAllItemsRepresented(t *testing.T) {
	items := []string{"a", "b", "c", "d", "e"}
	hist, err := MonteCarlo(items, 10000, nil)
	if err != nil {
		t.Fatalf("MonteCarlo() error: %v", err)
	}
	for _, item := range items {
		if hist.counts[item] == 0 {
			t.Errorf("item %q has zero count in 10000 trials", item)
		}
	}
}

func TestMonteCarloApproximateUniformity(t *testing.T) {
	items := []string{"a", "b", "c", "d", "e"}
	trials := 50000
	hist, err := MonteCarlo(items, trials, nil)
	if err != nil {
		t.Fatalf("MonteCarlo() error: %v", err)
	}

	expected := float64(trials) / float64(len(items))
	for _, item := range items {
		c := float64(hist.counts[item])
		// Allow 5% deviation from expected.
		if c < expected*0.90 || c > expected*1.10 {
			t.Errorf("item %q: count %.0f deviates >10%% from expected %.0f", item, c, expected)
		}
	}
}

func TestMonteCarloProgressCalled(t *testing.T) {
	items := []string{"a", "b"}
	called := 0
	progress := func(_ *Histogram) {
		called++
	}

	// Use enough trials to exceed the 50ms ticker interval on most hardware.
	_, err := MonteCarlo(items, 5000000, progress)
	if err != nil {
		t.Fatalf("MonteCarlo() error: %v", err)
	}
	if called == 0 {
		t.Error("progress callback was never called for 5M trials")
	}
}

func TestMonteCarloMerge(t *testing.T) {
	h := NewHistogram([]string{"a", "b", "c"})
	h.merge(map[string]int{"a": 10, "b": 20})
	h.merge(map[string]int{"b": 5, "c": 15})

	if h.Total() != 50 {
		t.Errorf("Total() = %d, want 50", h.Total())
	}
	if h.counts["a"] != 10 {
		t.Errorf("counts[a] = %d, want 10", h.counts["a"])
	}
	if h.counts["b"] != 25 {
		t.Errorf("counts[b] = %d, want 25", h.counts["b"])
	}
	if h.counts["c"] != 15 {
		t.Errorf("counts[c] = %d, want 15", h.counts["c"])
	}
}

func TestMonteCarloEmptyItems(t *testing.T) {
	_, err := MonteCarlo(nil, 100, nil)
	if err == nil {
		t.Fatal("MonteCarlo(nil, ...) expected error, got nil")
	}
}

func TestMonteCarloZeroTrials(t *testing.T) {
	_, err := MonteCarlo([]string{"a"}, 0, nil)
	if err == nil {
		t.Fatal("MonteCarlo(..., 0, ...) expected error, got nil")
	}
}

func BenchmarkMonteCarlo100K(b *testing.B) {
	items := []string{"a", "b", "c", "d", "e"}
	for range b.N {
		_, _ = MonteCarlo(items, 100_000, nil)
	}
}

func BenchmarkMonteCarlo10M(b *testing.B) {
	items := []string{"a", "b", "c", "d", "e"}
	for range b.N {
		_, _ = MonteCarlo(items, 10_000_000, nil)
	}
}

func BenchmarkMonteCarlo100M(b *testing.B) {
	items := []string{"a", "b", "c", "d", "e"}
	for range b.N {
		_, _ = MonteCarlo(items, 100_000_000, nil)
	}
}

func BenchmarkSelectSequential100K(b *testing.B) {
	items := []string{"a", "b", "c", "d", "e"}
	for range b.N {
		for range 100_000 {
			_, _ = Select(items)
		}
	}
}
