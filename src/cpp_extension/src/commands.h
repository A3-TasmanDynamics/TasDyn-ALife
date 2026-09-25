#pragma once

#include <string>
#include <vector>

class Database;

// Dispatches one callExtension command by name. This is the one place new
// commands get wired in -- cmd_save/cmd_load land here once
// docs/DATA_CONTRACT.md's "save"/"load" commands are implemented.
std::string DispatchCommand(Database& db, const std::string& command,
                             const std::vector<std::string>& args);
