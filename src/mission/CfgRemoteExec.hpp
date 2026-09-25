// TasDyn-ALife — remoteExec allowlist.
// See docs/ANTI_CHEAT.md Layer 1 ("API surface: remoteExec allowlist") for
// why this exists: only functions named here are network-callable at all.
// mode = 1 is whitelist-only (mode = 2 would ignore this list entirely —
// never use it). allowedTargets = 2 is server-only; nothing here is meant
// to be called directly by a client. Add every new server function here in
// the same PR that registers it in CfgFunctions.hpp and implements it —
// none of the three is optional.

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
    };

    class Commands
    {
        mode = 1;
    };
};
