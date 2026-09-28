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

        String ARGUMENTS going the other way (SQF -> extension) are handled
        on the C++ side, not here: Arma itself converts each array element
        to its `str` form while calling RVExtensionArgs, so "7656..." arrives
        with the quotes added *after* this script runs (and inner quotes
        doubled). Nothing in SQF can undo that; an earlier attempt to strip
        quotes here was a no-op for real strings and let a quoted uid create
        a duplicate players row. The extension's UnwrapArmaString
        (src/cpp_extension/src/commands.cpp) reverses it for every argument.

    Parameter(s):
        0: STRING - command ("ping" / "load" / "save")
        1: ARRAY - args for that command

    Returns:
        STRING - the extension's actual response string
*/

params ["_command", "_args"];

("tasdyn_alife" callExtension [_command, _args]) select 0
