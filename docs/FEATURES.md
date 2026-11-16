# Features

What ALife does today, what's being worked on, and what's on the idea list. Snapshot of
**16 November 2026**, taken from the code itself (mission, C++ extension, website, Discord bot,
Server Manager), not from plans.

The Admin panel's **Development → Project board** mirrors this list: *Finished* is the **Done**
column, *In progress* is **In progress**, *Ideas* is **To do**. Update both when something moves.
[ROADMAP.md](ROADMAP.md) is the older phase plan and is behind this list.

- [Finished](#finished)
  - [In game](#in-game)
  - [Website](#website)
  - [Discord and TeamSpeak](#discord-and-teamspeak)
  - [Server Manager](#server-manager)
- [In progress](#in-progress)
- [Ideas](#ideas)

---

## Finished

### In game

**Core and saving**
- Custom C++ extension talking to PostgreSQL: prepared statements only, idempotent request tokens
  so a repeated save or purchase can never apply twice, and server-side checks on everything a
  client asks for (`CfgRemoteExec` allowlist, server-authoritative actions).
- Join → load your character → spawn menu → play. Saved on spawn, every 60 seconds, on death and
  on disconnect. Each faction keeps its own alive state and position, so logging off to dodge
  death or arrest doesn't work.
- 48 slots across Civilian, Police (BLUFOR) and EMS (Independent). Spawn points for Civilian,
  Rebel, Cartel, Police and EMS, plus *Last position* when you logged off alive.
- Loading screen with a clear message for every failure (including a missing database extension).

**Player basics**
- Food and water with a HUD; you lose health when either runs out.
- Interaction and radial menus, player tags, group markers, notifications, jump, holster,
  surrender and synced animations.
- Virtual inventory with weight, giving items, dropping gear, bags and containers.

**Phone** (Y)
- Messages, Bank, Inventory, Player ID, Settings (wallpapers), Market (live prices), and Wanted
  for police.

**Civilian work**
- Gathering and processing: oil, copper, iron, salt, sand into glass, diamonds, rock into cement,
  sold at the resource trader.
- Licences: driver, truck, pilot, boat, dive, gun, hunting, home, rebel, and one per processing
  route.

**Economy**
- Cash and bank are separate; ATMs for deposits, withdrawals and transfers.
- Dynamic market: buying pushes prices up, selling pushes them down, prices recover over time and
  survive restarts.
- Shops: general store and market, clothing, gun store, licence office, and car, truck, air and boat
  dealers, each limited to the right faction.
- Gas stations: fuel from any pump, paid by cash or bank.
- Every economy action is logged per player (shown in the website's player history).

**Vehicles**
- Buy, store and retrieve from garages, impound, lock and unlock with keys, trunk storage,
  faction skins, tyres and fuel.

**Crime**
- Gas station robberies (hold up the cashier with a weapon out).
- Police evidence vault robbery: bolt cutters, keypad decoder, blasting charge, then cracking the
  vault; police are alerted with a map marker.
- Wanted list with 17 crimes, from speeding to murder.
- Rebel market, armoury, clothing and vehicles.

**Police**
- Restraints (zip ties), search, seize items into the evidence vault, fines, jail and bail.
- Placeables: spike strips, cones, barriers, road lights and arrow signs.
- Lights, sirens and a pull-over whoop on every police vehicle; taser.
- Personal lockers; police market, armoury, clothing, cars, helicopters and boats.

**EMS**
- Revive with a defibrillator, and a death screen while you wait.
- Lights and sirens; personal lockers; medical market, equipment, clothing, vehicles and air.

**Staff tools in game**
- Admin menu: bring, heal, freeze, warn, kick, ban, compensate, give item or cash, give or take a
  licence, arsenal, god mode, invisibility, announcements, spawn and delete vehicles. Every action
  checks the staff member's website permissions and is logged to the staff log.
- Anti-cheat logging: unusual activity (like actions fired faster than a person could) is flagged
  for staff to review. Nothing kicks or bans automatically.

### Website

**Public**
- Landing page: live server status, players online by faction, how to join, faction cards,
  leaderboard teaser, FAQ, latest devlog. Link previews in Discord and search engines.
- Leaderboards (opt-in, wealth shown as a band): richest, most playtime, most houses, most
  vehicles, police service, EMS service, most wanted, richest and largest gangs.
- Public Police and EMS rosters, server status with uptime history, rules, devlog, 404 page.

**Players** (Steam sign-in)
- Dashboard: characters per faction, net worth, playtime chart, bank balance chart, recent
  activity, vehicles, houses, licences, gang, tickets, notifications, milestones, a "get set up"
  checklist, ban appeals. Money moves in game only.
- Profile: display name, Discord and TeamSpeak linking.
- Faction, division and staff applications; faction documents.
- Support tickets.

**Faction Command** (Police and EMS command)
- Overview with on-duty hours, strength by rank and members who've gone quiet.
- Roster with filters, personnel files, recruitment and training, divisions and qualifications,
  ranks and gear, discipline and blacklist, a document drive with version history, and the command
  log.

**Admin and support**
- Home page, one staff inbox, quick search.
- Live: online players (message, warn, kick, announce) and the live map.
- Moderation: cases (player reports become cases), bans and appeals, punishment points,
  anti-cheat review, watchlist.
- Players and world: player lookup and timeline, vehicles, houses, gangs, wanted board, event log,
  economy charts.
- Support: ticket queue, saved replies, player history on tickets.
- Staff: list and profiles, activity, applications, staff log, roles and permissions, role sync.
- Content: devlog, rules, item prices, notices, scheduled messages, Discord settings.
- Server control (start, stop, scheduled restarts) and development tools (health, logs, database
  browser, this project board).

### Discord and TeamSpeak
- Discord bot: account linking (website or `/link`), player and staff commands (`/players`,
  `/whois`, `/lookup`, `/profile`, `/status`, `/loa`, `/rank`, promote, demote, reinstate, `/sync`),
  onboarding, a live status message, and alerts for staff (tickets, staff log, anti-cheat, watchlist).
- Role sync from website ranks to Discord roles and TeamSpeak server groups.

### Server Manager
- Desktop app that runs the game server, website, Discord bot and TeamSpeak, restarts them if they
  crash, and rebuilds the website and bot from the code on every start.
- Deploys the mission and extension, edits `server.cfg`, live console and RPT log, performance
  graphs, staff and anti-cheat logs, and a local control API.

---

## In progress

- **Houses**: in the database, on the dashboard, leaderboards and admin pages, but buying and
  owning a house in game isn't built yet.
- **Gangs**: managed on the website (invite, ranks, remove), with gang leaderboards and an admin
  board, but creating, leaving and disbanding a gang in game isn't built yet.
- **Phone apps**: Perks, Gang, Keys and Admin show as "coming soon".
- **Live map terrain**: the export script is ready (`feat/map-export`); waiting on an export from
  the game to draw the map tiles.
- **Website speed**: compressing pages and caching files properly (started on `feat/speed`).
- **Website accessibility and security**: keyboard focus, contrast, reduced motion, security
  headers. Approved, not started.
- **Player extras on the website**: opt-in public player profiles, gang pages, a restart
  countdown. Approved, not started.
- **Going public**: set `SITE_BASE_URL` to the real address, HTTPS hosting, a backup and restore
  drill, rate limits on sign-in.
- **BattlEye**: not enabled yet.
- **Real-player testing**: a full session with real players across all three factions, then a
  closed alpha.

---

## Ideas

**Gameplay**
- Drugs: growing or processing, selling and trafficking (the crimes already exist).
- Hunting and fishing (the hunting licence already exists).
- Uranium mining and other late-game resources.
- Speed cameras (speeding is already a crime).
- Rebel and Cartel gameplay beyond spawns and shops, like territory or hideouts.
- A second vehicle tier for Police and EMS.

**Staff tools**
- In-game spectate, teleport, staff-only live map, object tools.
- Server lockdown, restart countdown warnings in game, time and weather control.
- Arsenal editor: item pools per faction and saved loadouts.
- Debug console for the top rank only.

**Website and community**
- Two-way Discord ticket sync (reply from a Discord thread).
- Staff meetings and an internal wiki ([OPERATIONS.md](OPERATIONS.md)).
- Faction duty logs for Police, EMS and gangs.

**Engineering**
- Load test of the extension with many players saving at once.
