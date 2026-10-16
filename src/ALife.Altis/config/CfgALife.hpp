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

    // Revive: a medic holds the Revive action this long (seconds), using one
    // medkit; the player gets up where they fell, with their gear, at
    // reviveHealth (0-1).
    reviveTime = 8;
    reviveHealth = 0.5;

    // Phone: opened with Y (DIK code 21). Items and money can be given to
    // players within giveDistance metres; messages are limited to one every
    // messageCooldown seconds and messageMaxLength characters.
    phoneKey = 21;
    giveDistance = 5;
    messageCooldown = 3;
    messageMaxLength = 300;
};
