# Live Map export

The admin panel's Live Map draws Altis from data read out of the game itself
(no third-party map images). `exportMap.sqf` reads the terrain in a **local
editor preview** and writes it to your Arma client's RPT log; the website's
map generator then turns that log into the map tiles.

## 1. Export (about 5–10 minutes, on your own PC)

1. Start **Arma 3** (the normal game, no mods needed).
2. **Editor** → open **Altis** (an empty scenario is fine, or the ALife mission).
3. Place any playable unit if the scenario has none, then **Play → Play in Singleplayer**.
4. Press **Esc** → the **Debug Console** opens on the pause menu.
   (If you don't see it: in the editor, *Attributes → General → States → Debug Console: Available in SP*.)
5. Copy the whole of `tools/mapexport/exportMap.sqf` into the big **Execute** box and press **LOCAL EXEC**.
6. Close the pause menu and wait. Progress shows in the top-right
   (heights → trees → roads and buildings). Don't alt-tab away for long;
   the game keeps running it in the background either way.
7. When it says **"Map export finished"**, quit to the main menu.

## 2. Send the log

The log is the newest `Arma3_x64_*.rpt` in:

```
%LOCALAPPDATA%\Arma 3\
```

(Paste that into the Explorer address bar.) It will be large (tens of MB).
Put it somewhere staff can reach — e.g. copy it into this repo's
`tools/mapexport/` folder (it's git-ignored) — and tell staff where it is.

## 3. Generate the tiles (staff)

```
cd src/website
go run ./cmd/mapgen -rpt ../../tools/mapexport/<file>.rpt -out web/static/map/tiles
```

This writes vector map tiles that the Live Map picks up automatically. The
tiles are only ever made from the game's own terrain data.
