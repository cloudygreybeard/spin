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
	"math"
	"strings"
	"testing"
	"time"
)

func noopSleep(time.Duration) {}

// defaultTestCfg returns a Config with default physics but a no-op sleep
// function so tests complete instantly.
func defaultTestCfg() Config {
	return Config{
		Force:    1.0,
		Mass:     1.0,
		Friction: 0.2,
		Drag:     0.1,
		MaxDelay: 500 * time.Millisecond,
		Start:    -1,
		sleepFn:  noopSleep,
	}
}

// ----- ParseItems -----------------------------------------------------------

func TestParseItems(t *testing.T) {
	tests := []struct {
		name      string
		raw       string
		separator string
		want      []string
		wantErr   bool
	}{
		{
			name: "whitespace separated",
			raw:  "apple  banana cherry",
			want: []string{"apple", "banana", "cherry"},
		},
		{
			name: "newline separated",
			raw:  "apple\nbanana\ncherry",
			want: []string{"apple", "banana", "cherry"},
		},
		{
			name: "mixed whitespace",
			raw:  "  apple\t banana \n cherry  ",
			want: []string{"apple", "banana", "cherry"},
		},
		{
			name:      "comma separator",
			raw:       "apple,banana,,cherry",
			separator: ",",
			want:      []string{"apple", "banana", "cherry"},
		},
		{
			name:      "regex separator",
			raw:       "apple|banana||cherry",
			separator: `\|+`,
			want:      []string{"apple", "banana", "cherry"},
		},
		{
			name: "empty input",
			raw:  "",
			want: []string{},
		},
		{
			name: "whitespace only",
			raw:  "   \t\n  ",
			want: []string{},
		},
		{
			name:      "invalid regex",
			raw:       "a,b",
			separator: "[invalid",
			wantErr:   true,
		},
		{
			name: "single item",
			raw:  "only",
			want: []string{"only"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseItems(tt.raw, tt.separator)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseItems() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if len(got) != len(tt.want) {
				t.Fatalf("ParseItems() returned %d items, want %d: got %v", len(got), len(tt.want), got)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("ParseItems()[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

// ----- delaySchedule (Coulomb-only, drag=0) ---------------------------------

func TestDelayScheduleCoulombMonotonicallyIncreases(t *testing.T) {
	configs := []struct {
		name  string
		v0    float64
		alpha float64
	}{
		{"default", 35.0, 20.0},
		{"high friction", 35.0, 120.0},
		{"low friction", 35.0, 5.0},
		{"fast spin", 100.0, 20.0},
	}

	for _, c := range configs {
		t.Run(c.name, func(t *testing.T) {
			delays := delaySchedule(c.v0, c.alpha, 0, 1.0)
			if len(delays) == 0 {
				t.Skip("no delays produced")
			}
			for i := 1; i < len(delays); i++ {
				if delays[i] < delays[i-1] {
					t.Errorf("delay[%d] = %v < delay[%d] = %v; must be monotonically increasing",
						i, delays[i], i-1, delays[i-1])
				}
			}
		})
	}
}

func TestDelayScheduleCoulombTickCount(t *testing.T) {
	delays := delaySchedule(35.0, 20.0, 0, 0.5)
	if len(delays) != 30 {
		t.Errorf("Coulomb delaySchedule(35,20,0,0.5) produced %d ticks, want 30", len(delays))
	}
}

func TestDelayScheduleCoulombFirstDelayMatchesV0(t *testing.T) {
	v0 := 35.0
	delays := delaySchedule(v0, 20.0, 0, 0.5)
	if len(delays) == 0 {
		t.Fatal("no delays")
	}
	expected := 1.0 / v0
	got := delays[0].Seconds()
	if math.Abs(got-expected)/expected > 0.05 {
		t.Errorf("first delay = %v s, want ~%v s (1/v0)", got, expected)
	}
}

func TestDelayScheduleCoulombPhysicsFormula(t *testing.T) {
	v0, alpha := 35.0, 20.0
	delays := delaySchedule(v0, alpha, 0, 1.0)
	v0sq := v0 * v0

	for k, d := range delays {
		disc := v0sq - 2*alpha*float64(k)
		discNext := v0sq - 2*alpha*float64(k+1)
		expected := (math.Sqrt(disc) - math.Sqrt(discNext)) / alpha
		got := d.Seconds()
		if math.Abs(got-expected) > 1e-9 {
			t.Errorf("delay[%d] = %v s, want %v s", k, got, expected)
		}
	}
}

// ----- delaySchedule (with drag) --------------------------------------------

func TestDelayScheduleDragMonotonicallyIncreases(t *testing.T) {
	configs := []struct {
		name string
		v0   float64
		ac   float64
		beta float64
	}{
		{"default params", 35.0, 20.0, 0.1},
		{"high drag", 35.0, 20.0, 1.0},
		{"low mass high v0", 350.0, 1.0, 1.0},
		{"heavy wheel", 7.0, 20.0, 0.02},
	}

	for _, c := range configs {
		t.Run(c.name, func(t *testing.T) {
			delays := delaySchedule(c.v0, c.ac, c.beta, 1.0)
			if len(delays) == 0 {
				t.Skip("no delays produced")
			}
			for i := 1; i < len(delays); i++ {
				if delays[i] < delays[i-1] {
					t.Errorf("delay[%d] = %v < delay[%d] = %v; must be monotonically increasing",
						i, delays[i], i-1, delays[i-1])
				}
			}
		})
	}
}

func TestDelayScheduleDragReducesTicks(t *testing.T) {
	coulomb := delaySchedule(35.0, 20.0, 0, 0.5)
	withDrag := delaySchedule(35.0, 20.0, 0.1, 0.5)
	if len(withDrag) >= len(coulomb) {
		t.Errorf("drag produced %d ticks >= Coulomb %d; drag should reduce tick count",
			len(withDrag), len(coulomb))
	}
}

func TestDelayScheduleDragLightWheelStopsFaster(t *testing.T) {
	// The key behavioural fix: a light wheel (high beta = drag/mass)
	// should produce fewer ticks than a heavy wheel (low beta),
	// given the same initial velocity.
	light := delaySchedule(35.0, 20.0, 1.0, 1.0)  // beta=1.0 (mass=0.1, drag=0.1)
	heavy := delaySchedule(35.0, 20.0, 0.02, 1.0)  // beta=0.02 (mass=5, drag=0.1)
	if len(light) >= len(heavy) {
		t.Errorf("light wheel (beta=1) produced %d ticks >= heavy wheel (beta=0.02) %d ticks",
			len(light), len(heavy))
	}
}

func TestDelayScheduleDragNarrowsMassGap(t *testing.T) {
	// With the same force, a lighter wheel still spins longer (higher v0
	// dominates), but drag should narrow the gap compared to Coulomb-only.
	lightV0 := baseVelocity * 1.0 / 0.5 // mass=0.5 → v0=70
	heavyV0 := baseVelocity * 1.0 / 2.0 // mass=2 → v0=17.5
	ac := baseDeceleration * 0.2         // alpha_c=20

	coulombLight := delaySchedule(lightV0, ac, 0, 1.0)
	coulombHeavy := delaySchedule(heavyV0, ac, 0, 1.0)
	coulombRatio := float64(len(coulombLight)) / float64(len(coulombHeavy))

	dragLight := delaySchedule(lightV0, ac, 0.1/0.5, 1.0)
	dragHeavy := delaySchedule(heavyV0, ac, 0.1/2.0, 1.0)
	dragRatio := float64(len(dragLight)) / float64(len(dragHeavy))

	if dragRatio >= coulombRatio {
		t.Errorf("drag should narrow the light/heavy gap: Coulomb ratio=%.1f, drag ratio=%.1f",
			coulombRatio, dragRatio)
	}
}

func TestDelayScheduleDragPreventsRunaway(t *testing.T) {
	// The original problem case: mass=0.1, friction=0.01 with Coulomb
	// only would produce ~61000 ticks. With drag, it should be bounded.
	v0 := baseVelocity * 1.0 / 0.1 // 350
	ac := baseDeceleration * 0.01   // 1
	drag, mass := 0.1, 0.1
	beta := drag / mass // 1.0

	delays := delaySchedule(v0, ac, beta, 0.5)
	if len(delays) > 500 {
		t.Errorf("light wheel with drag produced %d ticks; expected drag to cap this", len(delays))
	}
	if len(delays) == 0 {
		t.Error("expected some ticks")
	}
}

func TestDelayScheduleDragZeroFallsBackToCoulomb(t *testing.T) {
	coulomb := delaySchedule(35.0, 20.0, 0, 0.5)
	noDrag := delaySchedule(35.0, 20.0, 1e-15, 0.5)
	if len(coulomb) != len(noDrag) {
		t.Errorf("zero-drag produced %d ticks, Coulomb produced %d; should match",
			len(noDrag), len(coulomb))
	}
}

func TestDelayScheduleDragFirstDelayApproxV0(t *testing.T) {
	v0 := 35.0
	delays := delaySchedule(v0, 20.0, 0.1, 1.0)
	if len(delays) == 0 {
		t.Fatal("no delays")
	}
	expected := 1.0 / v0
	got := delays[0].Seconds()
	if math.Abs(got-expected)/expected > 0.10 {
		t.Errorf("first delay = %v s, want ~%v s (1/v0, within 10%%)", got, expected)
	}
}

// ----- alignStart -----------------------------------------------------------

func TestAlignStart(t *testing.T) {
	tests := []struct {
		name       string
		userStart  int
		winnerIdx  int
		totalTicks int
		n          int
	}{
		{name: "auto basic", userStart: -1, winnerIdx: 2, totalTicks: 30, n: 5},
		{name: "auto winner at zero", userStart: -1, winnerIdx: 0, totalTicks: 30, n: 5},
		{name: "auto single item", userStart: -1, winnerIdx: 0, totalTicks: 30, n: 1},
		{name: "explicit zero", userStart: 0, winnerIdx: 3, totalTicks: 30, n: 5},
		{name: "explicit wraps", userStart: 12, winnerIdx: 1, totalTicks: 30, n: 5},
		{name: "explicit exact n", userStart: 5, winnerIdx: 4, totalTicks: 30, n: 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start := alignStart(tt.userStart, tt.winnerIdx, tt.totalTicks, tt.n)
			if start < 0 || start >= tt.n {
				t.Fatalf("alignStart() = %d, out of range [0, %d)", start, tt.n)
			}
			if tt.userStart < 0 {
				landed := (start + tt.totalTicks) % tt.n
				if landed != tt.winnerIdx {
					t.Errorf("auto start %d + %d ticks lands on %d, want %d",
						start, tt.totalTicks, landed, tt.winnerIdx)
				}
			} else {
				if start != tt.userStart%tt.n {
					t.Errorf("explicit start = %d, want %d %% %d = %d",
						start, tt.userStart, tt.n, tt.userStart%tt.n)
				}
			}
		})
	}
}

// ----- Spin -----------------------------------------------------------------

func TestSpinReturnsItemFromList(t *testing.T) {
	items := []string{"alpha", "bravo", "charlie", "delta", "echo"}
	cfg := defaultTestCfg()

	var buf bytes.Buffer
	winner, err := Spin(&buf, items, cfg)
	if err != nil {
		t.Fatalf("Spin() error: %v", err)
	}

	found := false
	for _, item := range items {
		if winner == item {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Spin() returned %q, which is not in the item list", winner)
	}
}

func TestSpinOutputContainsWinner(t *testing.T) {
	items := []string{"alpha", "bravo", "charlie"}
	cfg := defaultTestCfg()

	var buf bytes.Buffer
	winner, err := Spin(&buf, items, cfg)
	if err != nil {
		t.Fatalf("Spin() error: %v", err)
	}

	output := buf.String()
	finalLine := "* " + winner
	if !strings.Contains(output, finalLine) {
		t.Errorf("output does not contain final reveal %q:\n%s", finalLine, output)
	}
}

func TestSpinSingleItem(t *testing.T) {
	cfg := defaultTestCfg()
	var buf bytes.Buffer
	winner, err := Spin(&buf, []string{"only"}, cfg)
	if err != nil {
		t.Fatalf("Spin() error: %v", err)
	}
	if winner != "only" {
		t.Errorf("Spin() = %q, want %q", winner, "only")
	}
}

func TestSpinExplicitStart(t *testing.T) {
	items := []string{"a", "b", "c", "d", "e"}
	cfg := defaultTestCfg()
	cfg.Start = 2

	var buf bytes.Buffer
	winner, err := Spin(&buf, items, cfg)
	if err != nil {
		t.Fatalf("Spin() error: %v", err)
	}

	output := buf.String()
	firstItem := items[cfg.Start%len(items)]
	idx := strings.Index(output, "> ")
	if idx < 0 {
		t.Fatal("output contains no animation frames")
	}
	frameText := strings.TrimSpace(output[idx+2:])
	if !strings.HasPrefix(frameText, firstItem) {
		t.Errorf("animation starts with %q, want prefix %q", frameText, firstItem)
	}

	found := false
	for _, item := range items {
		if winner == item {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Spin() returned %q, not in item list", winner)
	}
}

func TestSpinStartModuloWraps(t *testing.T) {
	items := []string{"a", "b", "c", "d", "e"}
	cfg := defaultTestCfg()
	cfg.Start = 11

	var buf bytes.Buffer
	winner, err := Spin(&buf, items, cfg)
	if err != nil {
		t.Fatalf("Spin() error: %v", err)
	}

	output := buf.String()
	idx := strings.Index(output, "> ")
	if idx < 0 {
		t.Fatal("output contains no animation frames")
	}
	frameText := strings.TrimSpace(output[idx+2:])
	if !strings.HasPrefix(frameText, "b") {
		t.Errorf("start=11 with 5 items: animation starts with %q, want prefix %q", frameText, "b")
	}

	found := false
	for _, item := range items {
		if winner == item {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Spin() returned %q, not in item list", winner)
	}
}

func TestSpinValidation(t *testing.T) {
	tests := []struct {
		name    string
		items   []string
		cfg     Config
		wantErr string
	}{
		{
			name:    "empty items",
			items:   nil,
			cfg:     Config{Force: 1, Mass: 1, Friction: 0.2, Drag: 0.1, MaxDelay: time.Second},
			wantErr: "no items",
		},
		{
			name:    "zero force",
			items:   []string{"a"},
			cfg:     Config{Force: 0, Mass: 1, Friction: 0.2, Drag: 0.1, MaxDelay: time.Second},
			wantErr: "force must be positive",
		},
		{
			name:    "negative mass",
			items:   []string{"a"},
			cfg:     Config{Force: 1, Mass: -1, Friction: 0.2, Drag: 0.1, MaxDelay: time.Second},
			wantErr: "mass must be positive",
		},
		{
			name:    "zero friction",
			items:   []string{"a"},
			cfg:     Config{Force: 1, Mass: 1, Friction: 0, Drag: 0.1, MaxDelay: time.Second},
			wantErr: "friction must be positive",
		},
		{
			name:    "negative drag",
			items:   []string{"a"},
			cfg:     Config{Force: 1, Mass: 1, Friction: 0.2, Drag: -0.1, MaxDelay: time.Second},
			wantErr: "drag must be non-negative",
		},
		{
			name:    "zero max delay",
			items:   []string{"a"},
			cfg:     Config{Force: 1, Mass: 1, Friction: 0.2, Drag: 0.1, MaxDelay: 0},
			wantErr: "max delay must be positive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			_, err := Spin(&buf, tt.items, tt.cfg)
			if err == nil {
				t.Fatal("Spin() expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("Spin() error = %q, want substring %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestSpinPhysicsEffects(t *testing.T) {
	items := []string{"a", "b", "c", "d", "e"}
	baseCfg := defaultTestCfg()

	countFrames := func(cfg Config) int {
		var buf bytes.Buffer
		if _, err := Spin(&buf, items, cfg); err != nil {
			t.Fatalf("Spin() error: %v", err)
		}
		return strings.Count(buf.String(), "> ")
	}

	base := countFrames(baseCfg)
	if base == 0 {
		t.Fatal("base config produced 0 frames")
	}

	highForce := baseCfg
	highForce.Force = 2.0
	if f := countFrames(highForce); f <= base {
		t.Errorf("force=2 produced %d frames <= base %d", f, base)
	}

	heavyMass := baseCfg
	heavyMass.Mass = 3.0
	if f := countFrames(heavyMass); f >= base {
		t.Errorf("mass=3 produced %d frames >= base %d", f, base)
	}

	highFriction := baseCfg
	highFriction.Friction = 0.5
	if f := countFrames(highFriction); f >= base {
		t.Errorf("friction=0.5 produced %d frames >= base %d", f, base)
	}

	highDrag := baseCfg
	highDrag.Drag = 0.5
	if f := countFrames(highDrag); f >= base {
		t.Errorf("drag=0.5 produced %d frames >= base %d", f, base)
	}
}

// ----- secureRandomInt ------------------------------------------------------

func TestSecureRandomIntDistribution(t *testing.T) {
	const (
		max    = 10
		trials = 1000
	)
	counts := make([]int, max)
	for range trials {
		v, err := secureRandomInt(max)
		if err != nil {
			t.Fatalf("secureRandomInt() error: %v", err)
		}
		if v < 0 || v >= max {
			t.Fatalf("secureRandomInt(%d) = %d, out of range", max, v)
		}
		counts[v]++
	}

	for i, c := range counts {
		if c < 40 || c > 160 {
			t.Errorf("bucket %d has %d hits in %d trials (expected ~%d); may indicate bias", i, c, trials, trials/max)
		}
	}
}

// ----- maxItemLen -----------------------------------------------------------

func TestMaxItemLen(t *testing.T) {
	tests := []struct {
		items []string
		want  int
	}{
		{[]string{"a", "bb", "ccc"}, 3},
		{[]string{"hello"}, 5},
		{[]string{""}, 0},
	}
	for _, tt := range tests {
		got := maxItemLen(tt.items)
		if got != tt.want {
			t.Errorf("maxItemLen(%v) = %d, want %d", tt.items, got, tt.want)
		}
	}
}
