// Licence names for the phone's Player ID app. Licences are stored per
// faction in players.<faction>_licence as a list of these keys (e.g.
// ["driver","boat"]); a key with no entry here is shown as-is.

class CfgLicences
{
    // Civilian
    class driver  { displayName = "Driver Licence"; };
    class truck   { displayName = "Truck Licence"; };
    class pilot   { displayName = "Pilot Licence"; };
    class boat    { displayName = "Boating Licence"; };
    class dive    { displayName = "Diving Licence"; };
    class gun     { displayName = "Firearms Licence"; };
    class hunting { displayName = "Hunting Licence"; };
    class home    { displayName = "Home Owner"; };
    class rebel   { displayName = "Rebel Training"; };
    // Processing (CfgResources.hpp): free with the licence, a fee per batch without
    class oil     { displayName = "Oil Processing"; };
    class copper  { displayName = "Copper Processing"; };
    class iron    { displayName = "Iron Processing"; };
    class salt    { displayName = "Salt Processing"; };
    class sand    { displayName = "Sand Processing"; };
    class diamond { displayName = "Diamond Processing"; };
    class cement  { displayName = "Cement Mixing"; };

    // Police
    class cAir    { displayName = "Police Air Unit"; };
    class cg      { displayName = "Coast Guard"; };

    // EMS
    class mAir    { displayName = "Air Ambulance"; };
};
