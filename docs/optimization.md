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

The target utility is:

```text
below target: utility = ratio⁴
at or above target: utility = 1 + min(ratio - 1, 0.25) × 0.2
```

Its search contribution is:

```text
utility × importance × 10
```

The fourth-power curve keeps a meaningful incentive to approach each requested
target instead of treating a nearby floor as almost complete. The interface
importance levels map to `0.5`, `1`, `2`, and `8`. Surplus value stops improving
utility after 125% of the target, at a maximum utility of `1.05`. This prevents
one heavily overcapped stat from overwhelming every other goal.

Strict minimums and maximums are hard constraints. A result that violates one
is rejected rather than merely receiving a lower score. Strictness and
tolerance never change the ranking score of an otherwise identical build. This
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

### Choosing a search method

**Fast is the recommended mode for current versions of the application.** It
reduces the candidate space enough to provide the most practical balance of
result quality and execution time for everyday use.

Balanced retains and evaluates substantially more combinations. Because every
module may introduce different geometry, rotation, position, set, and stat
possibilities, its running time can grow quickly with inventory size and grid
complexity. Balanced is therefore best reserved for deliberate, longer searches
where the user is prepared to wait or stop the calculation and keep the best
result found so far.

The optimizer:

1. excludes equipment reserved by higher-priority characters unless reuse is
   allowed;
2. ranks modules by weighted score and efficiency per occupied cell;
3. keeps strong candidates across geometry and set groups;
4. generates valid rotations and positions;
5. explores combinations without overlap;
6. evaluates modules, cartridge, set, final stats, and goals together;
7. refines same-geometry substitutions in goal-oriented mode.

The exact solver uses an upper bound to discard branches that cannot beat the
current best result. Goal-oriented search avoids that bound during its main
non-linear scoring pass.

![Fast search configuration, ranked candidates, and projected statistics](images/build-search-results.png)

## Result interpretation

The interface deliberately separates three concepts:

1. **Equipment relevance** is the normalized, weighted value of the selected
   module and cartridge stats. It is not a roll-quality percentage.
2. **Build fit** is represented by final panel stats, configured objectives,
   strict bounds, caps, set activation, and valid console placement.
3. **Combat impact** is shown by the damage analysis when structured combat data
   is available for the character.

Fast and Balanced use the same equipment relevance calculation. The difference
between them is the number of candidates and combinations retained and explored,
not the meaning of an item's score.

The displayed ranking is the sum of equipment relevance and weighted objective
utility. Constraints only filter admissible builds: adding a strict minimum
does not change the score of the same build. Fast search always retains the
selected character's currently equipped modules, allowing the current build to
remain a baseline candidate after inventory reduction. Rankings are useful for
comparing candidates under the same character strategy, but they are not a
universal quality percentage and should not be compared across unrelated
profiles.

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
