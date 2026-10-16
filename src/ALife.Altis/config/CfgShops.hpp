// Shops. A shop opens with a scroll-wheel action when you're within
// `radius` metres of one of its markers and playing its faction. Prices are
// checked by the server, never taken from the client.
//
//   faction  -- "civ" / "cop" / "medic" (who can use it)
//   markers  -- where it is (mission.sqm markers)
//   Items    -- class = item from CfgItems.hpp, price per item

class CfgShops
{
    class medic_supplies
    {
        displayName = "Medical Supplies";
        faction = "medic";
        markers[] = { "spawn_medic_kavala", "spawn_medic_athira", "spawn_medic_pyrgos" };
        radius = 30;
        class Items
        {
            class medkit { price = 250; };
        };
    };
};
