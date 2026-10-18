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
        class ALife_fnc_requestPhoneData { allowedTargets = 2; };
        class ALife_fnc_shopBuy { allowedTargets = 2; };
        class ALife_fnc_giveItem { allowedTargets = 2; };
        class ALife_fnc_bankTransfer { allowedTargets = 2; };
        class ALife_fnc_sendMessage { allowedTargets = 2; };
        class ALife_fnc_requestRevive { allowedTargets = 2; };
        class ALife_fnc_completeRevive { allowedTargets = 2; };
        class ALife_fnc_containerOpen { allowedTargets = 2; };
        class ALife_fnc_containerTransfer { allowedTargets = 2; };
        class ALife_fnc_atmAction { allowedTargets = 2; };
        class ALife_fnc_shopBuyClothing { allowedTargets = 2; };
        class ALife_fnc_shopBuyVehicle { allowedTargets = 2; };
        class ALife_fnc_vehicleLock { allowedTargets = 2; };

        // Server -> client
        class ALife_fnc_sessionReceived { allowedTargets = 1; };
        class ALife_fnc_spawnApproved { allowedTargets = 1; };
        class ALife_fnc_notify { allowedTargets = 1; };
        class ALife_fnc_phoneData { allowedTargets = 1; };
        class ALife_fnc_messageReceived { allowedTargets = 1; };
        class ALife_fnc_revived { allowedTargets = 1; };
        class ALife_fnc_reviveApplied { allowedTargets = 1; };
        class ALife_fnc_containerData { allowedTargets = 1; };
        class ALife_fnc_clothingShop { allowedTargets = 1; };
        class ALife_fnc_vehicleShop { allowedTargets = 1; };
        class ALife_fnc_vehicleLockApply { allowedTargets = 1; };
    };

    class Commands
    {
        mode = 1;
    };
};
