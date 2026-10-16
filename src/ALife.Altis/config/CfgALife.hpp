// Gameplay settings. Read with
//   getNumber (missionConfigFile >> "CfgALife" >> "<setting>")

class CfgALife
{
    // Death screen: seconds before the Respawn button unlocks. After that
    // you can respawn, or keep waiting for a medic.
    respawnTime = 120;

    // Police slots (mission.sqm police_1 .. police_10): the first
    // copPublicSlots are open to anyone; the rest need players.cop_level of
    // at least copWhitelistLevel, checked by the server when the character
    // loads.
    copPublicSlots = 4;
    copWhitelistLevel = 1;
};
