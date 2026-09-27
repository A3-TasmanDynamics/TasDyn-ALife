#pragma once

#include <string>
#include <vector>

class Database;

// Dispatches one callExtension command by name. This is the one place new
// commands get wired in -- "ping"/"load"/"save" per docs/DATA_CONTRACT.md
// are implemented in commands.cpp.
std::string DispatchCommand(Database& db, const std::string& command,
                             const std::vector<std::string>& args);
