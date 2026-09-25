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

// docs/DATA_CONTRACT.md "load": args = [uid]
std::string HandleLoad(Database& db, const std::vector<std::string>& args) {
    if (args.empty()) return "ERROR:missing uid";

    std::string response;
    std::string error;
    if (!db.LoadPlayer(args[0], response, error)) {
        return "ERROR:" + error;
    }
    return response;
}

// docs/DATA_CONTRACT.md "save": args = [uid, field, value, requestToken]
std::string HandleSave(Database& db, const std::vector<std::string>& args) {
    if (args.size() < 4) return "ERROR:expected [uid, field, value, requestToken]";

    std::string status;
    std::string error;
    if (!db.SaveField(args[0], args[1], args[2], args[3], status, error)) {
        return "ERROR:" + error;
    }
    return status;  // already "OK" / "ERROR" / "DUPLICATE" per the contract
}

}  // namespace

std::string DispatchCommand(Database& db, const std::string& command,
                             const std::vector<std::string>& args) {
    if (command == "ping") return HandlePing(db);
    if (command == "load") return HandleLoad(db, args);
    if (command == "save") return HandleSave(db, args);

    return "ERROR:unknown command";
}
