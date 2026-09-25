// TasDyn-ALife — remoteExec allowlist.
// See docs/ANTI_CHEAT.md Layer 1 ("API surface: remoteExec allowlist") for
// why this exists: only functions named here are network-callable at all.
// mode = 1 is whitelist-only (mode = 2 would ignore this list entirely —
// never use it). Add every new server function here in the same PR that
// registers it in CfgFunctions.hpp and implements it — none of the three
// is optional.
//
// allowedTargets constrains who a function may be remoteExec'd AT, not who
// may call it — 1 = clients only (server calling a specific player's
// screen, e.g. opening a menu), 2 = server only (a client asking the
// server to do something). The real protection for anything that mutates
// state lives in the function itself (e.g. ALife_fnc_spawnPlayer
// re-validates the request from scratch server-side regardless of what
// triggered the call) — allowedTargets narrows the attack surface, it
// isn't the only line of defense.

class CfgRemoteExec
{
    class Functions
    {
        mode = 1;
        jip = 0;

        class ALife_fnc_load
        {
            allowedTargets = 2;
        };
        class ALife_fnc_save
        {
            allowedTargets = 2;
        };
        class ALife_fnc_spawnPlayer
        {
            allowedTargets = 2;
        };
        class ALife_fnc_spawnMenu
        {
            allowedTargets = 1;
        };
    };

    class Commands
    {
        mode = 1;
    };
};
