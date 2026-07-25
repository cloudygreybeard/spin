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

// Package wheel implements a spinning wheel random-selection animation
// grounded in rotational kinematics.
//
// # Physics model
//
// The animation models a uniform disc (prize wheel) subject to Coulomb
// friction at a central axle bearing. Under constant friction torque the
// angular deceleration is:
//
//	alpha = 2 * mu * g * r_axle / R^2
//
// Mass cancels from the deceleration (I = 1/2 m R^2), so friction alone
// governs how quickly the wheel slows. Mass does affect the initial
// angular velocity for a given applied impulse:
//
//	v0 = impulse / I  ∝  Force / Mass
//
// The inter-item delay follows directly from the position-time relation
// theta(t) = v0*t - 1/2*alpha*t^2:
//
//	dt[k] = ( sqrt(v0^2 - 2*alpha*k) - sqrt(v0^2 - 2*alpha*(k+1)) ) / alpha
//
// This yields a physically accurate deceleration curve: rapid at first,
// with progressively longer pauses as the wheel approaches rest.
//
// The winner is selected uniformly at random using crypto/rand before the
// animation begins; the visual spin reveals a predetermined result.
package wheel

import (
	"crypto/rand"
	"fmt"
	"io"
	"math"
	"math/big"
	"regexp"
	"strings"
	"time"
)

const (
	// Scaling factors calibrated so default parameters (Force=1, Mass=1,
	// Friction=0.2) produce ~30 ticks, a ~29 ms initial delay, and a
	// ~153 ms final delay for a natural-feeling animation.
	baseVelocity     = 35.0  // items/s at Force=1, Mass=1
	baseDeceleration = 100.0 // items/s^2 at Friction=1
)

// Config controls the wheel's physics and starting position.
type Config struct {
	// Force is the magnitude of the initial push (default 1.0).
	// Higher values produce a faster initial spin and more total
	// rotations. Proportional to the tangential impulse applied
	// at the rim of the wheel.
	Force float64

	// Mass is the wheel's mass (default 1.0). Higher values increase
	// the moment of inertia, reducing the initial angular velocity for
	// a given force. In the Coulomb friction model mass cancels from
	// the deceleration rate, so it affects only how far the wheel
	// spins, not how quickly it slows.
	Mass float64

	// Friction is the coefficient of kinetic friction at the axle
	// bearing (default 0.2). Higher values increase the constant
	// deceleration, causing the wheel to stop sooner. Typical real
	// values range from 0.05 (well-oiled bearing) to 0.5 (dry axle).
	Friction float64

	// MaxDelay is the inter-item delay at which the wheel is
	// considered stopped (default 500 ms). It maps to a minimum
	// velocity threshold: v_min = 1 / MaxDelay.
	MaxDelay time.Duration

	// Start is the 0-indexed starting position in the item list.
	// A negative value (the default) lets the wheel choose a starting
	// position automatically from the physics. Values >= 0 are taken
	// modulo the item count, so a value larger than the list wraps
	// around the wheel.
	Start int

	// sleepFn overrides time.Sleep for testing. Nil uses the default.
	sleepFn func(time.Duration)
}

// Spin selects a uniformly random item from items and displays a spinning
// wheel animation on w that decelerates to a stop on the selected item.
// It returns the selected item.
func Spin(w io.Writer, items []string, cfg Config) (string, error) {
	n := len(items)
	if n == 0 {
		return "", fmt.Errorf("no items to select from")
	}
	if cfg.Force <= 0 {
		return "", fmt.Errorf("force must be positive, got %v", cfg.Force)
	}
	if cfg.Mass <= 0 {
		return "", fmt.Errorf("mass must be positive, got %v", cfg.Mass)
	}
	if cfg.Friction <= 0 {
		return "", fmt.Errorf("friction must be positive, got %v", cfg.Friction)
	}
	if cfg.MaxDelay <= 0 {
		return "", fmt.Errorf("max delay must be positive, got %v", cfg.MaxDelay)
	}

	v0 := baseVelocity * cfg.Force / cfg.Mass
	alpha := baseDeceleration * cfg.Friction

	winnerIdx, err := secureRandomInt(n)
	if err != nil {
		return "", fmt.Errorf("selecting winner: %w", err)
	}

	delays := delaySchedule(v0, alpha, cfg.MaxDelay.Seconds())
	totalTicks := len(delays)

	if totalTicks == 0 {
		padWidth := maxItemLen(items)
		fmt.Fprintf(w, "  * %-*s\n", padWidth, items[winnerIdx])
		return items[winnerIdx], nil
	}

	startIdx := alignStart(cfg.Start, winnerIdx, totalTicks, n)

	if cfg.Start >= 0 {
		// User-specified start: prepend extra ticks at peak velocity
		// so the total advances still land on the winner. Physically
		// equivalent to a marginally harder push.
		extra := ((winnerIdx - startIdx - totalTicks%n) % n + n) % n
		if extra > 0 {
			dt := time.Duration(float64(time.Second) / v0)
			pad := make([]time.Duration, extra)
			for i := range pad {
				pad[i] = dt
			}
			delays = append(pad, delays...)
		}
	}

	sleep := time.Sleep
	if cfg.sleepFn != nil {
		sleep = cfg.sleepFn
	}

	padWidth := maxItemLen(items)
	for k, d := range delays {
		fmt.Fprintf(w, "\r  > %-*s", padWidth, items[(startIdx+k)%n])
		sleep(d)
	}

	// Settle: pad the final pause to MaxDelay for a consistent reveal.
	if settle := cfg.MaxDelay - delays[len(delays)-1]; settle > 0 {
		sleep(settle)
	}

	fmt.Fprintf(w, "\r  * %-*s\n", padWidth, items[winnerIdx])
	return items[winnerIdx], nil
}

// ParseItems splits raw text into non-empty, trimmed items.
// If separator is empty, items are split on whitespace.
// Otherwise, separator is compiled as a regular expression.
func ParseItems(raw, separator string) ([]string, error) {
	var parts []string
	if separator != "" {
		re, err := regexp.Compile(separator)
		if err != nil {
			return nil, fmt.Errorf("invalid separator regex %q: %w", separator, err)
		}
		parts = re.Split(raw, -1)
	} else {
		parts = regexp.MustCompile(`\s+`).Split(raw, -1)
	}

	items := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			items = append(items, t)
		}
	}
	return items, nil
}

// delaySchedule computes the inter-item delay for each tick of a wheel
// decelerating from initial velocity v0 (items/s) with constant angular
// deceleration alpha (items/s^2). The schedule stops when the computed
// delay exceeds maxDelaySec or the wheel lacks sufficient energy to
// complete the next full position traversal.
//
// Each delay is the exact traversal time between consecutive discrete
// positions under constant deceleration:
//
//	dt[k] = ( sqrt(v0^2 - 2*alpha*k) - sqrt(v0^2 - 2*alpha*(k+1)) ) / alpha
//
// Only ticks with enough kinetic energy for a full position change are
// included. This preserves the monotonically increasing delay property
// (provably: the delay function is decreasing in the velocity squared).
func delaySchedule(v0, alpha, maxDelaySec float64) []time.Duration {
	v0sq := v0 * v0
	var delays []time.Duration
	for k := 0; ; k++ {
		disc := v0sq - 2*alpha*float64(k)
		discNext := v0sq - 2*alpha*float64(k+1)
		if disc <= 0 || discNext < 0 {
			break
		}
		dt := (math.Sqrt(disc) - math.Sqrt(discNext)) / alpha
		if dt > maxDelaySec {
			break
		}
		delays = append(delays, time.Duration(dt*float64(time.Second)))
	}
	return delays
}

// alignStart returns the 0-indexed starting position so that after
// totalTicks single-step advances through n items the wheel lands on
// winnerIdx. If userStart >= 0 it is taken modulo n; otherwise the
// position is derived from the winner and tick count.
func alignStart(userStart, winnerIdx, totalTicks, n int) int {
	if userStart >= 0 {
		return userStart % n
	}
	return ((winnerIdx - totalTicks%n) % n + n) % n
}

// secureRandomInt returns a cryptographically secure random integer in [0, max).
func secureRandomInt(max int) (int, error) {
	nBig, err := rand.Int(rand.Reader, big.NewInt(int64(max)))
	if err != nil {
		return 0, fmt.Errorf("crypto/rand: %w", err)
	}
	return int(nBig.Int64()), nil
}

func maxItemLen(items []string) int {
	m := 0
	for _, item := range items {
		if len(item) > m {
			m = len(item)
		}
	}
	return m
}
