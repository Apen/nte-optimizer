# Optimization model

This document explains how NTE Optimizer evaluates equipment and chooses a
build. The model is a planning aid, not a guarantee of in-game damage.

## Statistic sources

For each character, the application combines:

1. level- and breakthrough-specific panel data from
   `data/game/characters/base_stats.json`;
2. the base critical rate and critical damage rules used by the model;
3. supported panel stats and permanent effects from the equipped Arc;
4. module main and secondary stats;
5. cartridge stats and activated two-piece or four-piece set bonuses;
6. the character console trait for the relevant module area;
7. optional values from `account_overrides.json`.

Effects requiring a rotation, target state, stack count, or other combat
condition remain separate when the data supports that distinction. The normal
stat column contains guaranteed values; maximum effects contain conditional
projections.

## Recommendation weights

A strategy supplies an ordered stat priority. For example:

```text
Critical Rate > Critical Damage = ATK % > Flat ATK
```

The default rank weights are:

| Rank | Weight |
| --- | ---: |
| 1 | 1.00 |
| 2 | 0.85 |
| 3 | 0.70 |
| 4 | 0.55 |
| 5 | 0.40 |
| 6 | 0.25 |
| 7 and later | minimum 0.10 |

Stats joined by `=` receive the same weight. Cartridge main-stat preferences
are stored separately: they filter eligible cartridges but do not create an
additional score weight.

## Weighted equipment relevance

NTE does not use random substat roll ranges for modules: for a given module
size, a particular substat always contributes the same amount. The application
therefore does not present a fictional roll-quality percentage. It measures how
relevant the effective stats of a piece are to the selected character strategy.

The amount still matters because two-, three-, and four-cell modules contribute
different fixed amounts while consuming different areas of the console. The
solver evaluates that stat budget together with geometry instead of treating a
four-cell module as an intrinsically better roll than a two-cell module.

Each property has a reference value in `data/optimizer/references.json` so
different units can be compared.

```text
normalized value = effective value / reference value
contribution     = normalized value × strategy weight
equipment relevance = sum of contributions
```

For a critical-rate reference of `0.064`, a roll of `0.032`, and a weight of
`1.0`, the contribution is `0.50`.

If a profile defines a cap:

- value above the hard cap contributes nothing;
- value above the soft cap is multiplied by `after_soft_scale`;
- when that scale is absent, the default post-cap efficiency is 25%.

## Goals, constraints, and importance

For target `C` and actual value `V`:

```text
ratio = V / C
```

Fast uses the original target utility:

```text
below target: utility = ratio⁴
at or above target: utility = 1 + min(ratio - 1, 0.25) × 0.2
```

Its search contribution is:

```text
utility × importance × 10
```

Beta is an experimental alternative. It uses the same goals, weights,
inventory, and approximate candidate search, but a bounded, diminishing-return
utility. The target is only a saturation threshold; it does not set the slope.
For a positive scale `S` resolved before the search:

```text
M = min(max(V, 0), C)
contribution = 10 × importance × M / (S + M)
S = 10 × reference
```

References come from `data/optimizer/references.json` for every profile, not
from character recommendations. Final ATK, HP, and DEF use the larger of the
flat reference and base stat multiplied by the percent reference. A missing
reference is an error, not a fallback to the target. The global factor `10`
is experimental calibration; it may be adjusted after comparing Fast and Beta
on different characters. At the same value below both targets,
raising the target leaves the score and marginal gain unchanged. Each goal
contributes less than `10 × importance`, and values above target add no points.

The interface lets users set a nonnegative weight. A zero-weight goal makes
no ranking contribution, even when a strict constraint is attached. Surplus
value under the Fast curve stops improving
utility after 125% of the target, at a maximum utility of `1.05`. This prevents
one heavily overcapped stat from overwhelming every other goal.

Strict minimums and maximums are hard constraints. A result that violates one
is rejected rather than merely receiving a lower score. A new strict minimum
is stored as an absolute value, independently of the soft target; existing
saved settings using a target-relative tolerance are still read. Strictness
never changes the ranking score of an otherwise identical build. This
keeps scores comparable when constraints are added or removed and ensures that
an unconstrained search cannot rank the same build differently.

## Sets and cartridges

Each recommended set variant appears as a selectable strategy. The solver only
uses cartridges owned by the imported account and checks whether the selected
module geometry activates the best reachable set tier.

```text
set score   = score of the activated tier
build score = module score + cartridge score + set score
```

Set activation is checked as a constraint. Grid occupancy is not awarded an
artificial score: an otherwise weaker build cannot win merely because its
modules cover one additional cell.

## Search process

### Search strategy

The optimizer uses one objective-oriented search strategy. It reduces the
candidate space enough to provide a practical balance of result quality and
execution time. Because every module may introduce different geometry,
rotation, position, set, and stat possibilities, the result is approximate
unless the retained search space is fully explored.

The optimizer:

1. excludes equipment reserved by higher-priority characters unless reuse is
   allowed;
2. ranks modules by weighted score and efficiency per occupied cell;
3. keeps strong candidates across geometry and set groups, including multiple
   specialists per objective and the currently equipped modules;
4. generates valid rotations and positions;
5. explores combinations without overlap;
6. evaluates modules, cartridge, set, final stats, and goals together;
7. refines same-geometry substitutions in goal-oriented mode.

Search bounds account for the maximum of each configured preference curve.
Fast remains approximate because it preselects modules and has a time limit.
Beta additionally retains specialists for an unmet strict minimum,
even when the base value already meets its softer target or its ranking weight
is zero. The number retained per geometry is based on the remaining deficit
and the number of that geometry that could fit by playable area. This is a
feasibility safeguard, not a proof that the full inventory was searched.
An internal `exact-objective` diagnostic can run the same objective function
without Fast preselection or a timeout on small test inventories; it is not a
practical search mode for a full account.

![Fast search configuration, ranked candidates, and projected statistics](images/build-search-results.png)

## Result interpretation

The interface deliberately separates three concepts:

1. **Equipment relevance** is the normalized, weighted value of the selected
   module and cartridge stats. It is not a roll-quality percentage.
2. **Build fit** is represented by final panel stats, configured objectives,
   strict bounds, caps, set activation, and valid console placement.
3. **Combat impact** is shown by the damage analysis when structured combat data
   is available for the character.

The displayed ranking is the weighted objective utility plus equipment
relevance multiplied by `0.000001` as a tie-breaker. The detailed score dialog
shows that small term separately. Final panel values
already include modules, the cartridge, the Arc, set effects, and character
bonuses, so equipment relevance is not added to the ranking a second time. It
remains visible as a diagnostic and is used only as a stable tie-breaker between
otherwise equivalent candidates. Constraints only filter admissible builds:
adding a strict minimum does not change the score of the same build. Fast search
always retains the selected character's currently equipped modules, allowing
the current build to remain a baseline candidate after inventory reduction.
Rankings are useful for comparing candidates under the same character strategy,
but they are not a universal quality percentage and should not be compared
across unrelated profiles.

### Basic damage index

The ranked results also expose a sortable **Basic DMG** column. It represents a
coefficient-1 attack before enemy defense, resistance, skill coefficients, and
skill-specific bonuses:

```text
Basic DMG = ATK × (1 + universal DMG)
            × (1 + CRIT rate × (CRIT DMG - 1))
```

CRIT rate is capped at 100% for this calculation. The index is intentionally
independent of a particular ability, making it useful for quickly comparing
the general offensive potential of builds. The Damage tab remains the source
for skill-specific projections, conditional effects, and enemy mitigation.

`complete: true` means the retained search space was fully explored. If the
user stops a search, the application returns the best candidate found so far
with `complete: false`.

![Selected cartridge, console grid, and module placement for a result](images/application-overview.png)

Damage estimates use the structured combat rules currently available in
`data/game/combat/damage.json`. Missing or conditional rules are reported rather
than silently presented as guaranteed damage.

![Per-action current and projected damage comparison](images/damage-analysis.png)
