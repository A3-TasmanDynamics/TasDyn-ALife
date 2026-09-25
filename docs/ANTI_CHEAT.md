# Anti-Cheat: Threat Model & Design

This doc exists because "heuristic anti-cheat" in the README was two bullet points and a
sentence. Before Phase 3 (Anti-Cheat & Hardening) starts, this is the actual threat model: what
a cheater can do against a **vanilla-client** Arma 3 server, what each layer of defense catches,
and — just as importantly — what none of this can catch, so nobody is surprised later.

## 1. Constraints that shape everything here

- **No client-side mod is required to play.** We can't ship a kernel-mode driver, an EAC/VAC
  integration, or a Screentime/DUI overlay scanner — anything that needs the player to install
  something is off the table by the project's own premise.
- **Server can't read client memory.** Full stop. Anything that requires inspecting the client
  process (detecting a Cheat Engine table, an aimbot DLL, an ESP overlay) is not something this
  server can do on its own. See [§5](#5-explicitly-out-of-scope) for what fills that gap.
- **The server can, however, fully control what it trusts.** Every stateful action — money,
  inventory, licenses, jail time — already flows through the "Client Requests, Server Decides"
  pattern ([README §1](../README.md#1-concept)). Anti-cheat here is mostly about making sure that
  pattern has no gaps, not about detecting cheats after the fact.

## 2. Threat vectors, ranked by what they'd actually do to the server

| # | Vector | What it lets a cheater do | Prototype precedent? |
|---|---|---|---|
| 1 | **Economy exploitation** (dupe, double-submit, direct balance edit) | Infinite money/items — breaks the server's economy for everyone, fastest way to kill a Life server | Yes — the old prototype's save/load field-mapping bug corrupted rank/admin data; a deliberate dupe exploit is the same class of bug, done on purpose |
| 2 | **Arbitrary server-side code execution** via an unrestricted `remoteExec`/`publicVariableServer` surface | Worst case: full server compromise — spawn anything, grant any rank, read/write any DB row the extension can reach | Not seen in the prototype (extDB3 was the write path, not raw remoteExec), but this is the single highest-severity vector in *any* custom Arma framework |
| 3 | **Movement/teleport hacks** | Skip jail, escape encounters, reach unreachable areas, instant-travel for deliveries | — |
| 4 | **Generic cheat-menu variable manipulation** (god mode, infinite ammo, no-recoil toggles from off-the-shelf mission-agnostic cheat menus) | Combat advantage, invulnerability | — |
| 5 | **Information leakage via broadcast state** (`publicVariable` sending sensitive data — wanted status, undercover flags, stash locations — to every client) | Passive cheating: a modified client just *reads* what the server already sent everyone | — |
| 6 | **Aimbot / ESP / wallhack via external memory-reading tools** | Combat advantage | — |

Vectors 1–5 are things the server-authoritative design can directly prevent or detect. Vector 6 is
the one this architecture fundamentally cannot solve alone — addressed head-on in §5, not hidden.

## 3. Defense layers

### Layer 0 — Platform: BattlEye

Not previously called out in the README, and it should be: **BattlEye ships as part of the Arma 3
client and server** — enabling it server-side requires zero client-side install, so it doesn't
violate the "vanilla client" premise. It catches exactly the class of generic
Cheat-Engine/DLL-injection tooling that a heuristic layer built for *this* server's logic wasn't
designed to catch, and it's a single server-config toggle plus a filters file to maintain. This is
free coverage — there's no reason not to run it alongside everything below.

- Enable BattlEye on the dedicated server (`-BEpath`, `BattlEye` folder present).
- Start from BattlEye's published community filter set; tune false positives during closed alpha.
- BattlEye and the heuristic layer below are complementary, not redundant — BE catches
  generic/known cheat signatures, the heuristic layer catches Life-server-specific abuse that no
  generic filter would recognize (a legitimate-looking `remoteExec` call requesting an
  impossible sell price, say).

### Layer 1 — API surface: `remoteExec` allowlist

Every network-callable function the mission exposes is a potential Vector 2. The framework must
enumerate them, not leave them implicit:

- A `CfgRemoteExec` allowlist (`server.jip` mode, `class = "TargetAny"` disabled) so only
  named, reviewed functions are callable across the network at all — no ad-hoc
  `[params] remoteExec ["someFunction"]` is reachable unless it's in this list.
  Ties directly into the [SQF request/response framework skeleton](../CONTRIBUTING.md) work in
  Phase 1 — this allowlist *is* the "Server Decides" boundary, made concrete and enforced by the
  engine rather than by convention.
- Every allowlisted function validates argument **type, range, and ownership** before touching
  state — e.g. a "sell item" request checks the calling client actually owns that inventory slot,
  that the quantity is a positive integer within the item's known bounds, and that the shop exists
  at the player's current position, before it ever reaches the DB layer.
- No SQF function ever trusts a client-supplied price, balance, or ID as authoritative — those are
  always re-derived server-side from the DB record, never taken from the request payload as fact.

### Layer 2 — Economic integrity

Directly addresses Vector 1, and is where the prototype actually broke:

- **Transaction locking** on every DB write that touches money/inventory (already tracked — issue
  covering `cmd_save`/`cmd_load` and locking in Phase 1).
- **Idempotent requests**: every economy-affecting request (buy, sell, bank transfer) carries a
  request token; a duplicate token within the debounce window is rejected server-side rather than
  applied twice. This is the concrete fix for double-submit duplication (spam-clicking "sell", or
  a scripted double `remoteExec`) — a plain DB transaction lock alone only protects against
  *concurrent* writes, not a client re-sending the same logical request before the first response
  lands.
- Client never holds an authoritative balance — the displayed cash/bank figure is always a
  server push, never a value the client can locally mutate and have honored.
- **Specific dupe patterns to test against** (the "toolless" methods that don't need an external
  tool, just bad server-side assumptions — these are the concrete cases the layer above needs to
  actually survive, not just a general promise of "transaction locking"):
  - **Death-race dupe**: item transferred to a container in the same tick a player dies, so both
    the death-drop logic and the transfer logic think they own the item.
  - **Vehicle-exit dupe**: item moved into a vehicle cargo the same tick a player exits/the vehicle
    despawns, similarly double-counted by two code paths that both think they're authoritative.
  - **Trade-race dupe**: two players trade the same item simultaneously from two different client
    requests before the first trade's DB write completes.
  - **Container desync dupe**: client-side container UI state drifts from the server's actual
    inventory record (e.g. after a reconnect mid-transaction), and the next action is built against
    the stale client view instead of a fresh server-authoritative read.
  - Idempotency tokens (above) catch the request-replay shape of these; the container/inventory
    code additionally needs to re-read current state from the DB immediately before every mutation
    rather than trusting whatever state the request implies — replay protection alone doesn't fix
    a genuine race between two different legitimate-looking requests.

### Layer 3 — Movement & behavior heuristics

Directly addresses Vectors 3 and 4:

- **Distance-per-tick validation**: server tracks each player's position on a fixed interval;
  a delta exceeding the fastest legitimate movement mode (sprint speed, or vehicle speed while
  in a vehicle) by a margin flags the player. Margin needs real tuning against actual network
  jitter/lag during closed alpha — too tight and legitimate laggy connections get flagged.
- **Honeypot variables**: the framework defines a small set of decoy variables named like the
  variables off-the-shelf, mission-agnostic cheat menus commonly target (e.g. generic
  `"godMode"`/`"infiniteAmmo"`-style names) that have **no actual gameplay effect** in this
  framework — real state lives under obscure, framework-specific variable names instead. A
  server-side write-watch on the decoy (`addPublicVariableEventHandler`-style, or equivalent
  server-owned check) means: nothing in this codebase should ever set that variable, so any write
  to it is a near-zero-false-positive signal that a generic cheat menu is loaded and active.
- Rate-limiting of suspicious action bursts (e.g. dozens of "sell" requests inside one second)
  feeds the same alerting path as the two checks above, even before a hard threshold trips.
- **Continuous re-checks, not just on-join**: all of the above run on a recurring interval for the
  whole session, not only at connect time — a client that cheats clean at join and goes hot later
  is exactly the case a join-time-only check misses.
- **Repeat-offense tracking**: flags accumulate per-UID across a session (and across sessions,
  since they're DB-backed) rather than resetting on each individual check — a player who trips
  three medium-confidence signals in an hour is a materially different case than one who trips one,
  even though no single signal alone crossed the high-confidence bar.
- **First-time player screening**: a UID's first session runs with a tighter movement-threshold
  margin and a shorter idempotency debounce window than the tuned defaults — new accounts are
  watched more closely by default, relaxing to normal thresholds after one clean session. Surfaced
  to staff via [ADMIN_TOOLS.md §8](ADMIN_TOOLS.md#8-access-control-banlist-and-reporting).

### Layer 4 — Response & ops

Detection without a response plan is just logging. Graduated response, because false positives are
real (the movement check especially, under real network conditions):

| Confidence | Signal | Response |
|---|---|---|
| High | Honeypot variable write | Auto-flag + immediate admin alert; ban is a manual admin action after a quick log review, not fully automatic, at least through the alpha |
| Medium | Movement/teleport threshold exceeded | Flag + log; admin-reviewable, not auto-kick, until the margin is proven tuned against real latency |
| Medium | Rejected idempotency token / economy validation failure | Log only at first — mostly expected noise from client-server race conditions, not necessarily malicious |
| Ops | BattlEye filter match | BattlEye's own action (kick/ban per filter config) |

All flags route through the audit-logging pattern already established for
[TasDyn-AI's `Syslog`](https://github.com/A3-TasmanDynamics/TasDyn-AI) — same "one category, one
severity, one routed channel" shape, so admin alerting doesn't need a second bespoke system
invented just for this server.

Admin tooling is what turns a flag into an actual action — [ADMIN_TOOLS.md §6's anti-cheat flag
review panel](ADMIN_TOOLS.md#6-menu-sections) (Admin tier) is the concrete in-game surface for
this table, not just a design intention.

### Layer 5 — Post-launch maturity

Explicitly deferred past launch (see
[ROADMAP.md](ROADMAP.md#post-launch--fast-follow-explicitly-out-of-scope-for-launch)):
tuning thresholds against real player data, expanding the honeypot variable set, reviewing
flagged-event logs for patterns, and revisiting whatever Layer 3 margins turned out too
tight/loose in practice.

## 4. What ships for launch

Everything in Layers 0–4 above is launch-scope — none of it was in the original two-bullet
version of this plan except the movement check and the honeypot variables. Layer 5 is the only
part explicitly pushed post-launch, and that's a deliberate call, not an oversight: better to ship
four solid layers than five half-tuned ones.

The launch *date* itself is a separate question from this doc's scope. It already moved once —
see [ROADMAP.md](ROADMAP.md) and [ADMIN_TOOLS.md §11](ADMIN_TOOLS.md#11-timeline-impact--the-honest-part)
for why: the admin-tooling side of this project grew from "basic tooling" to feature parity with
two commercial products, launch-critical, and that revised estimate is itself flagged as a
proposal rather than a settled fact.

## 5. Explicitly out of scope

Said plainly, because pretending otherwise would be worse than admitting it:

- **Aimbot, ESP, and wallhacks via external memory-reading tools are not something this
  architecture can detect.** No purely server-side, no-required-client-mod design can. BattlEye
  (Layer 0) catches the known/signature-matched tools in this category; anything novel or
  undetected by BattlEye's filters gets through. This is a real, permanent limitation of the
  vanilla-client premise, not a gap this doc's design closes. Worth noting: this isn't us falling
  short of a bar commercial competitors clear either — [Fini Anti-Hack & Admin Tools' own product
  page](https://bytex.market/products/item/7iclegb5zmytw3d22q3l/Fini%20Anti-Hack%20%26%20Admin%20Tools)
  explicitly states it doesn't detect memory-based cheats like Aurora, and can't fix
  framework-level exploits (e.g. jailing exploits) either — that second one is on this project's
  own SQF framework correctness, covered by the data-contract discipline elsewhere in the roadmap,
  not by an anti-cheat layer at all.
- **Mitigation, not prevention**: the one thing actually in the framework's control is *what
  data the server sends in the first place*. Vector 5 (information leakage) is where this
  matters — prefer targeted `remoteExec` to the specific clients who need a piece of state over
  global `publicVariable` broadcasts, so there's simply less for a reading-only cheat to read.
  This reduces the value of ESP-style tools without ever detecting them.
- Kernel-level anti-cheat (BattlEye's own kernel driver, if a player has it from Arma 3 itself)
  is Bohemia's software, not this project's — we configure and rely on it, we don't build it.

## 6. Open questions to settle during Phase 1/3

- Exact distance-per-tick thresholds per movement mode (walk/sprint/vehicle-by-type) — needs
  real numbers from actual playtesting, not a guess baked in ahead of time.
- Idempotency token TTL/debounce window — long enough to catch a genuine double-submit, short
  enough not to block a legitimate fast re-sell.
- Whether flagged-event alerts route through TasDyn-AI directly (Discord bot integration is
  post-launch per the roadmap) or through a simpler interim channel (RPT log + BEC admin chat)
  until the Discord bot exists.
