// Phone wallpapers (Settings -> Wallpaper). Each texture is the whole phone
// -- rounded body, bezel and screen -- drawn by
// tools/phone_icons/make_frame.py. The first entry is the default; apps
// always use the dark one so text stays readable.

class CfgPhone
{
    defaultWallpaper = "dark";
    appBackground = "ui\phone\phone_dark_ca.paa";

    class Wallpapers
    {
        class dark   { displayName = "Dark";   texture = "ui\phone\phone_dark_ca.paa"; };
        class dusk   { displayName = "Dusk";   texture = "ui\phone\phone_dusk_ca.paa"; };
        class sunset { displayName = "Sunset"; texture = "ui\phone\phone_sunset_ca.paa"; };
        class ocean  { displayName = "Ocean";  texture = "ui\phone\phone_ocean_ca.paa"; };
        class forest { displayName = "Forest"; texture = "ui\phone\phone_forest_ca.paa"; };
        class aurora { displayName = "Aurora"; texture = "ui\phone\phone_aurora_ca.paa"; };
    };
};
