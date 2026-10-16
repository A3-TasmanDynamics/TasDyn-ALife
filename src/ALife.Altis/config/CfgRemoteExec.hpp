// Only the functions listed here can be called over the network.
// allowedTargets: 1 = server may call it on a client, 2 = a client may
// call it on the server. Server functions also check the caller owns the
// unit they're asking about (remoteExecutedOwner), so one player can't act
// for another.

class CfgRemoteExec
{
    class Functions
    {
        mode = 1;
        jip = 0;

        // Client -> server
        class ALife_fnc_requestSession { allowedTargets = 2; };
        class ALife_fnc_requestSpawn { allowedTargets = 2; };

        // Server -> client
        class ALife_fnc_sessionReceived { allowedTargets = 1; };
        class ALife_fnc_spawnApproved { allowedTargets = 1; };
    };

    class Commands
    {
        mode = 1;
    };
};
