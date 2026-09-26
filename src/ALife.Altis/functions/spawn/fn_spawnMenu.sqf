/*
    File: fn_spawnMenu.sqf
    Author: Tasman Dynamics

    Description:
        Opens the spawn selection dialog. Client-side -- this is the target
        of a remoteExec from the server (initPlayerServer.sqf) once this
        player's own record has loaded successfully. Never called for
        anyone else's client. Everything else (populating the faction
        buttons/spawn list, defaulting to civilian) happens in the dialog's
        own onLoad handler -- see fn_spawnMenuOpen.sqf.

    Parameter(s):
        None

    Returns:
        Nothing
*/

closeDialog 0;
createDialog "ALife_SpawnMenu";
