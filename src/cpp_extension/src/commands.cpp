#include "commands.h"

#include "db.h"

namespace {

std::string HandlePing(Database& db) {
    std::string error;
    if (db.Ping(error)) {
        return "OK";
    }
    return "ERROR:" + error;
}

}  // namespace

std::string DispatchCommand(Database& db, const std::string& command,
                             const std::vector<std::string>& /*args*/) {
    if (command == "ping") {
        return HandlePing(db);
    }

    return "ERROR:unknown command";
}
