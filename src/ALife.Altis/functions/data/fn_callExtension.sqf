/*
    File: fn_callExtension.sqf
    Author: Tasman Dynamics

    Description:
        Thin wrapper around "tasdyn_alife" callExtension. The array-args
        form (`ext callExtension [command, args]`) returns an ARRAY
        `[resultString, returnCode]` in this Arma version -- NOT a bare
        string the way the single-string form does.

        Found the hard way: every direct call site (fn_load.sqf,
        fn_save.sqf, fn_sync.sqf) compared/parsed that whole array as if it
        were the response string, which produced a live "Error ==: Type
        Array, expected ... String" runtime error the moment a real
        dedicated server actually ran this code with -autoInit. This had
        been completely invisible until then -- every C++-side test this
        project ran used test_harness.exe's direct LoadLibrary/
        GetProcAddress calls, which never go through callExtension at all,
        so the SQF-side return shape was never actually exercised.

        Also found the hard way, same session: STRING arguments going the
        other direction (SQF -> extension) arrive at the C++ side wrapped
        in an extra literal pair of double quotes -- a real player's uid
        of 76561198127262076 showed up as a *second*, bogus players row
        with uid literally "76561198127262076" (quotes included as part of
        the text) the first time this ever ran through a genuine
        callExtension call from real SQF, rather than test_harness.exe's
        direct argv, which passes command-line args raw with no such
        wrapping. This is now the one place that strips that wrapping (by
        Unicode code point, not a literal `"` in this file's own source,
        to avoid any quote-escaping confusion), so no new call site can
        get bitten by it, the same way this file already centralizes
        unwrapping the [string, code] array return shape above.

    Parameter(s):
        0: STRING - command ("ping" / "load" / "save")
        1: ARRAY - args for that command

    Returns:
        STRING - the extension's actual response string
*/

// Strips one layer of literal-quote wrapping ("x" -> x) from a string arg
// before it crosses into the extension -- see the file header above.
// Non-strings pass through untouched (callExtension args can be numbers/
// booleans too, e.g. the empty [] for "ping").
private _fnc_unwrapQuotes = {
    if (!(_this isEqualType "")) exitWith { _this };
    private _chars = toArray _this;
    if (count _chars < 2) exitWith { _this };
    if ((_chars select 0 == 34) && ((_chars select (count _chars - 1)) == 34)) exitWith {
        toString (_chars select [1, (count _chars) - 2])
    };
    _this
};

params ["_command", "_args"];
private _cleanArgs = _args apply { _x call _fnc_unwrapQuotes };

("tasdyn_alife" callExtension [_command, _cleanArgs]) select 0
