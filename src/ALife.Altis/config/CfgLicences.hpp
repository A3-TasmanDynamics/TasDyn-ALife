// Licence names for the phone's Player ID app. Licences are stored per
// faction in players.<faction>_licence as a list of these keys (e.g.
// ["driver","boat"]); a key with no entry here is shown as-is.
//
// Processing licences can be bought at their processor (the worker's
// menu, ALife_fnc_buyLicence):
//   price    -- dollars
//   payment  -- "bank" (resources) or "cash" (drugs: cash only)

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
    class oil     { displayName = "Oil Processing";     price = 10000; payment = "bank"; };
    class copper  { displayName = "Copper Processing";  price = 8000;  payment = "bank"; };
    class iron    { displayName = "Iron Processing";    price = 9500;  payment = "bank"; };
    class salt    { displayName = "Salt Processing";    price = 12000; payment = "bank"; };
    class sand    { displayName = "Sand Processing";    price = 14500; payment = "bank"; };
    class diamond { displayName = "Diamond Processing"; price = 35000; payment = "bank"; };
    class cement  { displayName = "Cement Mixing";      price = 6500;  payment = "bank"; };

    // Police
    class cAir    { displayName = "Police Air Unit"; };
    class cg      { displayName = "Coast Guard"; };

    // EMS
    class mAir    { displayName = "Air Ambulance"; };
};
