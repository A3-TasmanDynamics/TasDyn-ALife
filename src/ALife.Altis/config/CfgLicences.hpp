// Licence names for the phone's Player ID app. Licences are stored per
// faction in players.<faction>_licence as a list of these keys (e.g.
// ["driver","boat"]); a key with no entry here is shown as-is.
//
// Licences with a price can be bought:
//   - the everyday ones at a licence office (CfgShops type "licences": a
//     vendor NPC, Windows key -> the licence window), from the bank;
//   - processing licences at their processor (the worker's menu).
// ALife_fnc_buyLicence does both.
//   price    -- dollars
//   payment  -- "bank" or "cash" (drugs: cash only)
// No price = not for sale (staff can still give it from the admin menu).

class CfgLicences
{
    // Civilian
    class driver  { displayName = "Driver Licence";   price = 500;   payment = "bank"; };
    class truck   { displayName = "Truck Licence";    price = 20000; payment = "bank"; };
    class pilot   { displayName = "Pilot Licence";    price = 25000; payment = "bank"; };
    class boat    { displayName = "Boating Licence";  price = 1000;  payment = "bank"; };
    class dive    { displayName = "Diving Licence";   price = 2000;  payment = "bank"; };
    class gun     { displayName = "Firearms Licence"; price = 10000; payment = "bank"; };
    class hunting { displayName = "Hunting Licence";  price = 5000;  payment = "bank"; };
    class home    { displayName = "Home Owner";       price = 75000; payment = "bank"; };
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
