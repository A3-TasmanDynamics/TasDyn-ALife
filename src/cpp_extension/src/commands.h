#pragma once

#include <string>
#include <vector>

class Database;

// Dispatches one callExtension command by name. This is the one place new
// commands get wired in -- "ping"/"load"/"save" per docs/DATA_CONTRACT.md
// are implemented in commands.cpp.
std::string DispatchCommand(Database& db, const std::string& command,
                             const std::vector<std::string>& args);

// Undoes Arma's argument conversion for one callExtension array argument.
// Arma passes every element to RVExtensionArgs as its `str` form, so a SQF
// string "abc" arrives as the 5 characters "abc" (quotes included) and any
// inner quote is doubled (a"b -> "a""b"). Non-string args (numbers,
// booleans) arrive unquoted and are returned unchanged.
std::string UnwrapArmaString(const std::string& arg);

// True for a 17-digit Steam64 ID -- the only valid players.uid.
bool IsSteam64(const std::string& uid);
