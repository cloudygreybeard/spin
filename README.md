# spin

A command-line tool for selecting an item at random from a list, with a
spinning text display that decelerates to rest.

> [!NOTE]
> *For Craig Robinson, on the occasion of his retirement from Red Hat.
> An engineer across more than one domain, and proof that the most
> useful perspective sometimes comes from working at the boundaries
> between them. His warmth, straight talking, and dependability made
> the work better and the people around him better for it.*

## 1 Background

A common requirement in both recreational and practical settings is to
choose a single item from a finite set in a manner that is fair and
unpredictable. Prize wheels, raffles, and team allocation exercises all
reduce to this problem. The approach taken here is to model the physical
behaviour of a spinning wheel, and to implement that model as a
composable Unix command-line program.

The physics draws on the treatment of rotational dynamics and friction
found in standard texts at GCSE and A-Level[^1][^2][^3], where a
uniform disc subject to a constant friction torque undergoes uniform
angular deceleration. The computer science draws on the modular
arithmetic and pseudorandom number theory covered in introductory
algorithm courses[^5][^6], together with the Unix pipeline conventions
established by the C programming tradition[^4].

## 2 Theory

### 2.1 Rotational kinematics under constant friction

Consider a uniform disc of mass $m$ and radius $R$, free to rotate about
a central axle of radius $r$. The disc is given an initial angular
velocity and then left to decelerate under kinetic friction alone.

The moment of inertia of a uniform disc about its central axis
is[^1][^2] $I = \frac{1}{2} m R^2$ ...(1). The normal reaction at the
axle bearing equals the weight of the disc ($mg$), so the friction force
is $F = \mu m g$, where $\mu$ is the coefficient of kinetic friction.
This force acts at the axle radius $r$, producing a retarding torque
$\tau = \mu m g r$ ...(2). By Newton's second law for rotation, the
angular deceleration is

```math
\alpha = \frac{\tau}{I} = \frac{2 \mu g r}{R^2} \qquad \ldots\,(3)
```

Note that $m$ cancels. The deceleration depends only on the friction
coefficient and the geometry of the wheel, not on its mass. This result
is often surprising on first encounter[^2], but it follows directly from
the fact that both the moment of inertia and the friction torque are
proportional to $m$.

Mass does, however, affect the initial angular velocity for a given
applied impulse. If a tangential force $F_{\text{push}}$ is applied at
the rim for a brief interval $\delta t$, the angular impulse is
$F_{\text{push}} \, \delta t \, R$, and the resulting initial angular
velocity is

```math
\omega_0 = \frac{F_{\text{push}} \, \delta t \, R}{I} = \frac{2 \, F_{\text{push}} \, \delta t}{m R} \qquad \ldots\,(4)
```

A heavier wheel starts more slowly for the same push, but once spinning,
it decelerates at the same rate under Coulomb friction alone. It
therefore covers fewer total revolutions before coming to rest.

### 2.2 Aerodynamic drag

In practice, a spinning wheel is also subject to aerodynamic drag, which
produces a retarding torque proportional to the angular velocity[^2][^3].
Writing $\gamma$ for the drag coefficient, the drag torque is
$\tau_{\text{drag}} = \gamma \omega$, and the resulting deceleration is
$\gamma \omega / I$. Since $I \propto m$, the drag deceleration is
inversely proportional to mass: a lighter wheel at the same angular
velocity experiences the same drag torque but has less inertia to resist
it.

Combining Coulomb friction and drag, the equation of motion becomes

```math
\frac{dv}{dt} = -\alpha_c - \beta v \qquad \ldots\,(5)
```

where $\alpha_c$ is the Coulomb deceleration from equation (3) and
$\beta = \gamma / m$. This is a first-order linear ODE with solution

```math
v(t) = \left(v_0 + \frac{\alpha_c}{\beta}\right) e^{-\beta t} - \frac{\alpha_c}{\beta} \qquad \ldots\,(6)
```

The velocity decays exponentially (governed by drag) towards a negative
offset (governed by Coulomb friction). The wheel stops when $v = 0$, at
time $t_{\text{stop}} = (1/\beta) \ln(1 + \beta v_0 / \alpha_c)$ ...(7).

The inclusion of drag resolves an otherwise counter-intuitive consequence
of the pure Coulomb model, in which a lighter wheel, pushed with the
same force, spins for arbitrarily longer because mass cancels from the
deceleration. With drag, the lighter wheel's higher initial velocity is
met with proportionally greater aerodynamic braking, producing behaviour
consistent with everyday experience: heavy flywheels coast, and light
wheels stop. When $\beta = 0$, the model reduces to pure Coulomb
friction and equations (5) and (6) simplify to constant deceleration.

### 2.3 The discrete delay schedule

In the programme, the continuous rotation of the wheel is represented by
a sequence of discrete item positions. If there are $n$ items arranged
around the wheel, the angular spacing between consecutive items is
$2\pi / n$ radians. We define a linear velocity $v$ in units of items
per second, with $\alpha_c$ and $\beta$ scaled accordingly.

The position traversed during a tick starting at velocity $v_k$ is found
by integrating equation (6):

```math
\theta(\tau) = \frac{1}{\beta}\left(v_k + \frac{\alpha_c}{\beta}\right)\left(1 - e^{-\beta \tau}\right) - \frac{\alpha_c}{\beta}\tau \qquad \ldots\,(8)
```

The inter-item delay $\Delta t_k$ is the value of $\tau$ for which
$\theta(\tau) = 1$. Because equation (8) involves both exponential and
linear terms, the inverse has no closed form; the programme solves it by
bisection to nanosecond precision. The velocity at the end of each tick
is then used as the starting velocity for the next.

The delays increase monotonically with $k$: since $v$ is strictly
decreasing and positive, each successive tick starts from a lower
velocity and therefore takes longer to complete. The wheel stops when it
lacks sufficient kinetic energy to complete the next full item
transition.

When $\beta = 0$ (no drag), the position-time relation reduces to a
quadratic and the delay admits the closed-form expression

```math
\Delta t_k = \frac{\sqrt{v_0^2 - 2\alpha_c k} - \sqrt{v_0^2 - 2\alpha_c(k+1)}}{\alpha_c} \qquad \ldots\,(9)
```

which the programme uses in this case for efficiency.

### 2.4 Selection and fairness

The winning item is selected before the animation begins, using the
cryptographically secure random number generator provided by the
operating system (`crypto/rand` in Go, which reads from `/dev/urandom`
on Unix systems). This source produces uniformly distributed integers
using rejection sampling over the full range of a `math/big.Int`[^5],
ensuring that no item is favoured by modular bias.

The display that follows is a deterministic rendering of this result.

### 2.5 Modular arithmetic and the starting position

The items are arranged in a circular sequence of length $n$, indexed
from $0$ to $n - 1$. After $T$ ticks from starting index $s$, the wheel
displays the item at index $(s + T) \bmod n$. This is a direct
application of modular (or clock) arithmetic[^5][^6], covered in most
introductory treatments of number theory and widely used in computing for
hashing, checksums, and circular buffer addressing.

When the starting position is chosen automatically, it is set so that
the animation lands on the winner:
$s = (\text{winner} - (T \bmod n) + n) \bmod n$ ...(10).
When the user supplies a starting position $p$ (1-indexed), it is
converted to 0-indexed and taken modulo $n$. A small number of extra
ticks at peak velocity are prepended to the delay schedule to satisfy the
alignment condition. Physically, this is equivalent to a marginally
harder push.

### 2.6 Pipeline composability

The programme follows the Unix convention[^7], established by the tools
described in Kernighan and Ritchie's *The C Programming Language*[^4]
and formalised in the POSIX standard, of writing diagnostic output to the
standard error stream and results to the standard output stream. This
allows `spin` to participate in pipelines:

```
spin -f nominees.txt | xargs notify-send
```

The animation appears on the terminal (via stderr) while only the
selected item passes through the pipe (via stdout).

### 2.7 Monte Carlo verification

A Monte Carlo simulation consists of repeated independent random trials
used to estimate a quantity that may be difficult to compute
analytically[^5][^6]. Here, the quantity of interest is the probability
distribution over the item set. Since each trial selects uniformly at
random from $n$ items, the expected frequency of each item after $N$
trials is $N/n$. By the law of large numbers, the empirical frequencies
converge to the true (uniform) probabilities as $N \to \infty$.

The `--monte-carlo` flag runs $N$ independent trials and displays a
live-updating histogram of the accumulated results. This provides a
visual proof of fairness: with sufficient trials, all bars should
converge to equal length.

## 3 Installation

### Homebrew (macOS and Linux)

```bash
brew install cloudygreybeard/tap/spin
```

### Go install

```bash
go install github.com/cloudygreybeard/spin@latest
```

### From source

```bash
git clone https://github.com/cloudygreybeard/spin.git
cd spin
make install
```

## 4 Usage

### 4.1 Basic selection

Items may be provided as positional arguments, piped from standard input,
or read from a file:

```bash
spin apple banana cherry
echo "red green blue" | spin
spin -f items.txt
spin -f items.txt -s ","
```

### 4.2 Worked examples

**Example 1.** A team of five is to be selected to present first. Using
the default parameters:

```bash
spin Alice Bob Carol Dave Eve
```

The wheel displays approximately 27 items over 1.5 seconds before
settling on the result.

**Example 2.** The same selection, but the wheel is to start at the
third position in the list:

```bash
spin --start 3 Alice Bob Carol Dave Eve
```

The animation begins at "Carol" and proceeds through the list
cyclically. The start value wraps via modulo, so `--start 8` in a list
of five items begins at item 3 (since $8 - 1 = 7$, and $7 \bmod 5 = 2$,
which is 0-indexed "Carol").

**Example 3.** Investigating the effect of friction. A well-oiled axle
bearing ($\mu = 0.05$) produces a longer spin, while a dry bearing
($\mu = 0.5$) brings the wheel to rest more quickly:

```bash
spin --friction 0.05 red green blue yellow
spin --friction 0.5  red green blue yellow
```

The reader may verify that doubling the friction coefficient
approximately halves the number of displayed items, in agreement with the
inverse relationship between $\alpha$ and the total traversal count
$N = v_0^2 / (2\alpha)$.

**Example 4.** Disabling drag to use the pure Coulomb friction model, or
increasing it to simulate a wheel in a viscous medium:

```bash
spin --drag 0 red green blue yellow
spin --drag 0.5 red green blue yellow
```

**Example 5.** Selecting without the wheel display, printing only the
result:

```bash
spin --fast-forward Alice Bob Carol Dave Eve
```

**Example 6.** Running a Monte Carlo simulation of 1000 trials to verify
the uniformity of the distribution. The histogram updates live on the
terminal; the final frequency table is written to stdout:

```bash
spin --monte-carlo 1000 --fast-forward a b c d e
```

With five items and 1000 trials, each item should appear approximately
200 times. The `--fast-forward` flag is recommended for large trial
counts; without it, each trial displays the full wheel animation.

**Example 7.** Capturing the frequency table for further analysis:

```bash
spin --monte-carlo 10000 --fast-forward a b c d e > results.tsv
```

The TSV output contains one row per item with the count and percentage,
suitable for piping to `sort`, `awk`, or a plotting tool.

### 4.3 Physics parameters

| Flag         | Default | Physical meaning                                   |
|--------------|---------|-----------------------------------------------------|
| `--force`    | 1.0     | Magnitude of the initial push (impulse)              |
| `--mass`     | 1.0     | Mass of the wheel (affects inertia and drag braking) |
| `--friction` | 0.2     | Coefficient of kinetic friction at the axle          |
| `--drag`     | 0.1     | Aerodynamic drag coefficient (0 = pure Coulomb)      |
| `--max-delay`| 500ms   | Delay threshold at which the wheel stops             |
| `--start`    | random  | Starting position (1-indexed, wraps mod $n$)         |

The relationships between these parameters and the display are
summarised in the following table:

| Increase in parameter | Effect on $v_0$ | Effect on $\alpha_c$ | Effect on $\beta$ | Effect on total items      |
|-----------------------|-----------------|----------------------|-------------------|----------------------------|
| Force                 | Increases       | No change            | No change         | Increases                  |
| Mass                  | Decreases       | No change            | Decreases         | Decreases (but drag helps) |
| Friction              | No change       | Increases            | No change         | Decreases                  |
| Drag                  | No change       | No change            | Increases         | Decreases                  |

## 5 Flags

```
  -f, --file string          path to input file
  -s, --separator string     item separator regex (default: whitespace)
      --start string         starting position (1-indexed, wraps via modulo) or "random" (default "random")
      --force float          spin force (default 1)
      --mass float           wheel mass (default 1)
      --friction float       coefficient of kinetic friction (default 0.2)
      --drag float           aerodynamic drag coefficient (default 0.1)
  -m, --max-delay duration   delay threshold at which the wheel stops (default 500ms)
  -n, --monte-carlo int      run N trials and display a frequency histogram
  -q, --fast-forward         skip the wheel animation (print result only)
  -h, --help                 help for spin
```

## 6 Development

```bash
make build    # Build the binary
make test     # Run tests with race detector
make lint     # Run golangci-lint
make clean    # Remove build artifacts
make snapshot # Build a snapshot release
```

## 7 Licence

Apache 2.0. See [LICENSE](LICENSE).

[^1]: Duncan, T. (1995) *GCSE Physics*, 3rd edn. London: John Murray.
[^2]: Muncaster, R. (1993) *A-Level Physics*, 4th edn. Cheltenham: Stanley Thornes.
[^3]: Nelkon, M. and Parker, P. (1995) *Advanced Level Physics*, 7th edn. Oxford: Heinemann.
[^4]: Kernighan, B.W. and Ritchie, D.M. (1988) *The C Programming Language*, 2nd edn. Englewood Cliffs: Prentice Hall.
[^5]: Aho, A.V., Hopcroft, J.E. and Ullman, J.D. (1983) *Data Structures and Algorithms*. Reading: Addison-Wesley.
[^6]: Brookshear, J.G. (1997) *Computer Science: An Overview*, 5th edn. Reading: Addison-Wesley.
[^7]: Tanenbaum, A.S. (1992) *Modern Operating Systems*. Englewood Cliffs: Prentice Hall.
