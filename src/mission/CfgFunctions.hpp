// TasDyn-ALife — function library registration.
// Registers ALife_fnc_load / ALife_fnc_save from functions/. Add new
// categories/functions here as the SQF side grows — every function must be
// registered here before it can be referenced in CfgRemoteExec.hpp.

class CfgFunctions
{
    class ALife
    {
        class Persistence
        {
            file = "functions";
            class load {};   // -> ALife_fnc_load, functions\fn_load.sqf
            class save {};   // -> ALife_fnc_save, functions\fn_save.sqf
        };
    };
};
