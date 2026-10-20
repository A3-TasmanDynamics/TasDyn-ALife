// The police wanted list and jail.
//
// Police add crimes to a player (Windows on them -> Wanted); each adds its
// bounty to the player's wanted total (wanted_crimes, players.civ_bounty).
// Arresting a restrained, wanted civilian at the jail (Windows -> Arrest)
// clears their crimes, gives the officer a share of the bounty in cash, and
// jails them for the crimes' minutes (min..max), with bail = bounty x
// bailRate (paid from the bank to get out early). Jail time is saved: you
// can't log out of it.

class CfgCrimes
{
    //   name   -- shown to police and the player
    //   bounty -- dollars added to their wanted total
    //   jail   -- minutes in jail it adds
    class speeding         { name = "Speeding";                      bounty = 1500;  jail = 2; };
    class reckless_driving { name = "Reckless driving";              bounty = 2500;  jail = 4; };
    class hit_and_run      { name = "Hit and run";                   bounty = 5000;  jail = 8; };
    class evading          { name = "Evading police";                bounty = 4000;  jail = 6; };
    class resisting        { name = "Resisting arrest";              bounty = 3000;  jail = 5; };
    class trespassing      { name = "Trespassing";                   bounty = 2000;  jail = 3; };
    class theft            { name = "Theft";                         bounty = 3000;  jail = 5; };
    class vehicle_theft    { name = "Vehicle theft";                 bounty = 8000;  jail = 10; };
    class assault          { name = "Assault";                       bounty = 5000;  jail = 8; };
    class illegal_weapon   { name = "Illegal weapon";                bounty = 6000;  jail = 8; };
    class drug_possession  { name = "Drug possession";               bounty = 4000;  jail = 6; };
    class drug_trafficking { name = "Drug trafficking";              bounty = 12000; jail = 12; };
    class robbery          { name = "Robbery";                       bounty = 20000; jail = 15; };
    class kidnapping       { name = "Kidnapping";                    bounty = 15000; jail = 15; };
    class attempted_murder { name = "Attempted murder";              bounty = 15000; jail = 15; };
    class murder           { name = "Murder";                        bounty = 30000; jail = 25; };
};

class CfgJail
{
    markers[] = { "jail" };          // the jail: an AREA marker (prisoners are kept inside it)
    release = "jail_release";        // where they're let out (a point marker)
    radius = 30;                     // metres: police must be this close to arrest (point marker)
    minMinutes = 2;                  // sentence = the crimes' minutes, at least this...
    maxMinutes = 60;                 // ...and at most this
    bailRate = 1;                    // bail = their bounty x this
    copReward = 0.5;                 // the arresting officer gets this share of the bounty, in cash
};
