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
	crand "crypto/rand"
	"fmt"
	"math/rand/v2"
	"runtime"
	"sync"
	"time"
)

const batchSize = 10000

// MonteCarlo runs n independent random selections from items in parallel
// across available CPU cores, where n is given by the trials parameter.
//
// Each worker uses a ChaCha8 PRNG seeded from crypto/rand, providing
// statistical randomness without the kernel CSPRNG contention that would
// otherwise serialise the goroutines. This is the standard approach for
// Monte Carlo simulation: cryptographic seeding for unpredictability,
// fast PRNG for throughput.
//
// Workers accumulate results in local frequency maps and send them in
// batches to a collector, eliminating per-item channel overhead. If
// progress is non-nil, it is called approximately every 50 ms with the
// current histogram state.
func MonteCarlo(items []string, trials int, progress func(*Histogram)) (*Histogram, error) {
	n := len(items)
	if n == 0 {
		return nil, fmt.Errorf("no items to select from")
	}
	if trials <= 0 {
		return nil, fmt.Errorf("trials must be positive, got %d", trials)
	}

	hist := NewHistogram(items)
	batches := make(chan map[string]int, runtime.NumCPU()*2)
	errc := make(chan error, 1)

	var wg sync.WaitGroup
	workers := runtime.NumCPU()

	for w := range workers {
		count := trials / workers
		if w < trials%workers {
			count++
		}
		wg.Add(1)
		go func(count int) {
			defer wg.Done()

			// Seed a per-worker PRNG from the OS CSPRNG.
			var seed [32]byte
			if _, err := crand.Read(seed[:]); err != nil {
				select {
				case errc <- fmt.Errorf("seeding PRNG: %w", err):
				default:
				}
				return
			}
			rng := rand.New(rand.NewChaCha8(seed))

			local := make(map[string]int, n)
			pending := 0
			for range count {
				local[items[rng.IntN(n)]]++
				pending++
				if pending >= batchSize {
					batches <- local
					local = make(map[string]int, n)
					pending = 0
				}
			}
			if pending > 0 {
				batches <- local
			}
		}(count)
	}

	go func() {
		wg.Wait()
		close(batches)
	}()

	var ticker *time.Ticker
	var tickC <-chan time.Time
	if progress != nil {
		ticker = time.NewTicker(50 * time.Millisecond)
		defer ticker.Stop()
		tickC = ticker.C
	}

	for batch := range batches {
		hist.merge(batch)

		select {
		case <-tickC:
			progress(hist)
		case err := <-errc:
			return nil, err
		default:
		}
	}

	select {
	case err := <-errc:
		return nil, err
	default:
	}

	return hist, nil
}
