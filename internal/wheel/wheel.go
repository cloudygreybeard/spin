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

// Package wheel implements a spinning wheel random-selection display
// grounded in rotational kinematics.
//
// # Physics model
//
// The wheel is a uniform disc subject to two retarding forces:
//
//   - Coulomb friction at the axle bearing, producing a constant
//     deceleration alpha_c = 2 * mu * g * r_axle / R^2. Mass cancels
//     from this term.
//
//   - Aerodynamic drag, producing a velocity-dependent deceleration
//     (gamma / m) * v. Mass appears in the denominator: a lighter wheel
//     at the same velocity experiences greater deceleration from drag.
//
// The combined equation of motion is:
//
//	dv/dt = -alpha_c - (gamma / m) * v
//
// With the substitution beta = gamma / m, the solution is:
//
//	v(t) = (v0 + alpha_c/beta) * exp(-beta*t) - alpha_c/beta
//
// The inter-item delay is computed by numerical bisection, since the
// position-time integral has no closed-form inverse. When beta = 0
// (no drag), the model reduces to pure Coulomb friction and the
// closed-form expression is used.
//
// The winner is selected uniformly at random using crypto/rand before
// the display begins; the visual spin reveals a predetermined result.
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
	// Friction=0.2, Drag=0.1) produce ~27 ticks with a natural-feeling
	// deceleration curve.
	baseVelocity     = 35.0  // items/s at Force=1, Mass=1
	baseDeceleration = 100.0 // items/s^2 at Friction=1
)

var whitespaceRE = regexp.MustCompile(`\s+`)

// Config controls the wheel's physics and starting position.
type Config struct {
	// Force is the magnitude of the initial push (default 1.0).
	// Higher values produce a faster initial spin and more total
	// rotations. Proportional to the tangential impulse applied
	// at the rim of the wheel.
	Force float64

	// Mass is the wheel's mass (default 1.0). Higher values increase
	// the moment of inertia, reducing the initial angular velocity
	// for a given force. Mass also appears in the drag term: a
	// heavier wheel experiences less drag deceleration per unit
	// velocity, so it coasts longer.
	Mass float64

	// Friction is the coefficient of kinetic friction at the axle
	// bearing (default 0.2). Higher values increase the constant
	// (Coulomb) deceleration, causing the wheel to stop sooner.
	Friction float64

	// Drag is the aerodynamic drag coefficient (default 0.1). It
	// contributes a velocity-dependent deceleration of (Drag/Mass)*v,
	// which dominates at high angular velocities and diminishes as
	// the wheel slows. Set to 0 for pure Coulomb friction.
	Drag float64

	// MaxDelay is the inter-item delay at which the wheel is
	// considered stopped (default 500 ms).
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
// wheel on w that decelerates to a stop on the selected item.
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
	if cfg.Drag < 0 {
		return "", fmt.Errorf("drag must be non-negative, got %v", cfg.Drag)
	}
	if cfg.MaxDelay <= 0 {
		return "", fmt.Errorf("max delay must be positive, got %v", cfg.MaxDelay)
	}

	v0 := baseVelocity * cfg.Force / cfg.Mass
	alphac := baseDeceleration * cfg.Friction
	beta := cfg.Drag / cfg.Mass

	winnerIdx, err := secureRandomInt(n)
	if err != nil {
		return "", fmt.Errorf("selecting winner: %w", err)
	}

	delays := delaySchedule(v0, alphac, beta, cfg.MaxDelay.Seconds())
	totalTicks := len(delays)

	if totalTicks == 0 {
		padWidth := maxItemLen(items)
		fmt.Fprintf(w, "  * %-*s\n", padWidth, items[winnerIdx])
		return items[winnerIdx], nil
	}

	startIdx := alignStart(cfg.Start, winnerIdx, totalTicks, n)

	if cfg.Start >= 0 {
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
		parts = whitespaceRE.Split(raw, -1)
	}

	items := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			items = append(items, t)
		}
	}
	return items, nil
}

// delaySchedule dispatches to the appropriate delay computation based
// on whether aerodynamic drag is present (beta > 0) or absent.
func delaySchedule(v0, alphac, beta, maxDelaySec float64) []time.Duration {
	if beta < 1e-12 {
		return delayScheduleCoulomb(v0, alphac, maxDelaySec)
	}
	return delayScheduleDrag(v0, alphac, beta, maxDelaySec)
}

// delayScheduleCoulomb computes delays under pure Coulomb friction
// (constant deceleration). Each delay is the exact traversal time
// between consecutive positions:
//
//	dt[k] = ( sqrt(v0^2 - 2*ac*k) - sqrt(v0^2 - 2*ac*(k+1)) ) / ac
func delayScheduleCoulomb(v0, ac, maxDelaySec float64) []time.Duration {
	v0sq := v0 * v0
	var delays []time.Duration
	for k := 0; ; k++ {
		disc := v0sq - 2*ac*float64(k)
		discNext := v0sq - 2*ac*float64(k+1)
		if disc <= 0 || discNext < 0 {
			break
		}
		dt := (math.Sqrt(disc) - math.Sqrt(discNext)) / ac
		if dt > maxDelaySec {
			break
		}
		delays = append(delays, time.Duration(dt*float64(time.Second)))
	}
	return delays
}

// delayScheduleDrag computes delays under combined Coulomb friction and
// aerodynamic drag. The velocity during each tick evolves as:
//
//	v(tau) = (v_k + ac/beta) * exp(-beta*tau) - ac/beta
//
// and the position traversed is:
//
//	theta(tau) = (v_k + ac/beta)/beta * (1 - exp(-beta*tau)) - (ac/beta)*tau
//
// Each delay is found by bisection on theta(dt) = 1.
func delayScheduleDrag(v0, ac, beta, maxDelaySec float64) []time.Duration {
	var delays []time.Duration
	v := v0
	acOverBeta := ac / beta

	for v > 1e-9 {
		a := v + acOverBeta

		// Time for velocity to reach zero from current v.
		tStop := math.Log(1+beta*v/ac) / beta

		// Maximum distance traversable before the wheel stops.
		thetaMax := a/beta*(1-math.Exp(-beta*tStop)) - acOverBeta*tStop
		if thetaMax < 1.0 {
			break
		}

		// Bisect for dt such that theta(dt) = 1.
		lo, hi := 0.0, tStop
		for i := 0; i < 100 && hi-lo > 1e-9; i++ {
			mid := (lo + hi) / 2
			theta := a/beta*(1-math.Exp(-beta*mid)) - acOverBeta*mid
			if theta < 1.0 {
				lo = mid
			} else {
				hi = mid
			}
		}
		dt := (lo + hi) / 2

		if dt > maxDelaySec {
			break
		}

		delays = append(delays, time.Duration(dt*float64(time.Second)))

		v = a*math.Exp(-beta*dt) - acOverBeta
	}

	return delays
}

// Select picks a uniformly random item from items using crypto/rand,
// without any visual display. It is the fast path used by Monte Carlo
// simulation and fast-forward mode.
func Select(items []string) (string, error) {
	if len(items) == 0 {
		return "", fmt.Errorf("no items to select from")
	}
	idx, err := secureRandomInt(len(items))
	if err != nil {
		return "", fmt.Errorf("selecting item: %w", err)
	}
	return items[idx], nil
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
