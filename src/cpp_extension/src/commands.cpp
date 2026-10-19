#include "commands.h"

#include "db.h"

#include <cctype>

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
    if (!IsSteam64(args[0])) return "ERROR:invalid uid";

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
    if (!IsSteam64(args[0])) return "ERROR:invalid uid";

    std::string status;
    std::string error;
    if (!db.SaveField(args[0], args[1], args[2], args[3], status, error)) {
        return "ERROR:" + error;
    }
    return status;  // already "OK" / "ERROR" / "DUPLICATE" per the contract
}

// args = [uid, faction, killer, weapon, distance, secondsIncapacitated]
std::string HandleCombatLog(Database& db, const std::vector<std::string>& args) {
    if (args.size() < 6) return "ERROR:expected [uid, faction, killer, weapon, distance, seconds]";
    if (!IsSteam64(args[0])) return "ERROR:invalid uid";
    if (args[1] != "civ" && args[1] != "cop" && args[1] != "medic") return "ERROR:invalid faction";
    auto isWholeNumber = [](const std::string& s) {
        if (s.empty()) return true;  // "" = not applicable
        size_t i = (s[0] == '-') ? 1 : 0;
        if (i == s.size() || s.size() > 10) return false;
        for (; i < s.size(); ++i) {
            if (s[i] < '0' || s[i] > '9') return false;
        }
        return true;
    };
    if (!isWholeNumber(args[4]) || !isWholeNumber(args[5])) return "ERROR:distance/seconds must be whole numbers";

    std::string error;
    if (!db.RecordCombatLog(args[0], args[1], args[2], args[3], args[4], args[5], error)) {
        return "ERROR:" + error;
    }
    return "OK";
}

// args = [op, uid, faction, amount, requestToken, otherUid, otherFaction, memo]
std::string HandleBank(Database& db, const std::vector<std::string>& args) {
    if (args.size() < 8) return "ERROR:expected [op, uid, faction, amount, token, otherUid, otherFaction, memo]";
    const std::string& op = args[0];
    if (op != "purchase" && op != "transfer" && op != "deposit" && op != "withdraw") return "ERROR:unknown bank op";
    if (!IsSteam64(args[1])) return "ERROR:invalid uid";
    if (op == "transfer" && !IsSteam64(args[5])) return "ERROR:invalid recipient uid";
    const std::string& amount = args[3];
    if (amount.empty() || amount.size() > 12 || amount[0] == '0') return "ERROR:invalid amount";
    for (char c : amount) {
        if (c < '0' || c > '9') return "ERROR:invalid amount";
    }
    if (args[4].empty() || args[4].size() > 128) return "ERROR:missing request token";
    if (args[7].size() > 200) return "ERROR:memo too long";

    std::string status;
    std::string error;
    bool ok = (op == "deposit" || op == "withdraw")
                  ? db.BankCash(op, args[1], args[2], amount, args[4], status, error)
                  : db.BankTx(op, args[1], args[2], amount, args[4], args[5], args[6], args[7], status, error);
    if (!ok) {
        return "ERROR:" + error;
    }
    return status;
}

// args = [op, uid]  (op: start / end / reset; reset takes no uid)
std::string HandleSession(Database& db, const std::vector<std::string>& args) {
    if (args.empty()) return "ERROR:expected [op, uid]";
    const std::string& op = args[0];
    std::string uid = args.size() > 1 ? args[1] : "";
    if (op != "reset" && !IsSteam64(uid)) return "ERROR:invalid uid";
    std::string error;
    if (!db.RecordSession(op, uid, error)) return "ERROR:" + error;
    return "OK";
}

// A config class name: what market items are keyed by.
bool IsItemName(const std::string& s) {
    if (s.empty() || s.size() > 64) return false;
    for (char c : s) {
        if (!(std::isalnum(static_cast<unsigned char>(c)) || c == '_')) return false;
    }
    return true;
}

// A positive price with at most two decimals, e.g. "12" or "12.40".
bool IsPrice(const std::string& s) {
    if (s.empty() || s.size() > 14) return false;
    size_t dot = s.find('.');
    std::string whole = s.substr(0, dot);
    std::string frac = dot == std::string::npos ? "" : s.substr(dot + 1);
    if (whole.empty() || frac.size() > 2 || (dot != std::string::npos && frac.empty())) return false;
    for (char c : whole + frac) {
        if (!std::isdigit(static_cast<unsigned char>(c))) return false;
    }
    return std::stod(s) > 0;
}

// args = [op, item, price]: "load" (no more args) / "set" item price / "reset"
std::string HandleMarket(Database& db, const std::vector<std::string>& args) {
    if (args.empty()) return "ERROR:expected [op, ...]";
    const std::string& op = args[0];
    std::string error;
    if (op == "load") {
        std::string response;
        if (!db.MarketLoad(response, error)) return "ERROR:" + error;
        return response;
    }
    if (op == "set") {
        if (args.size() < 3) return "ERROR:expected [set, item, price]";
        if (!IsItemName(args[1])) return "ERROR:invalid item";
        if (!IsPrice(args[2])) return "ERROR:invalid price";
        if (!db.MarketSet(args[1], args[2], error)) return "ERROR:" + error;
        return "OK";
    }
    if (op == "reset") {
        if (!db.MarketReset(error)) return "ERROR:" + error;
        return "OK";
    }
    return "ERROR:unknown market op";
}

// args = ["add", uid, faction, classname, price, skin?] -> "OK:<id>:<plate>"
//        ["status", id, status]                 -> "OK"
//        ["list", uid, faction]                 -> [[id, class, plate, status], ...]
//        ["get", id]                            -> [id, ownerUid, scope, class, plate, status, fuel, damage, inventory, skin]
//        ["store", id, status, fuel, damageJson, inventoryJson] -> "OK"   (status garaged / impounded)
//        ["inventory", id, inventoryJson]       -> "OK"
//        ["reset"]                              -> "OK:<count>"  (spawned -> garaged, at server start)
//        ["log", id, uid, action, amount]       -> "OK"   (action impound / return)
std::string HandleVehicle(Database& db, const std::vector<std::string>& args) {
    if (args.empty()) return "ERROR:expected [op, ...]";
    const std::string& op = args[0];
    std::string error;
    if (op == "add") {
        if (args.size() < 5) return "ERROR:expected [add, uid, faction, classname, price]";
        if (!IsSteam64(args[1])) return "ERROR:invalid uid";
        if (args[2] != "civ" && args[2] != "cop" && args[2] != "medic") return "ERROR:invalid faction";
        if (!IsItemName(args[3])) return "ERROR:invalid classname";
        if (!IsPrice(args[4]) || args[4].find('.') != std::string::npos) return "ERROR:invalid price";
        std::string skin = args.size() > 5 ? args[5] : "";
        if (skin.size() > 64) return "ERROR:invalid skin";
        for (char c : skin) {
            if (!std::isalnum(static_cast<unsigned char>(c)) && c != '_' && c != ':') return "ERROR:invalid skin";
        }
        std::string id, plate;
        if (!db.VehicleAdd(args[1], args[2], args[3], args[4], skin, id, plate, error)) return "ERROR:" + error;
        return "OK:" + id + ":" + plate;
    }
    if (op == "status") {
        if (args.size() < 3) return "ERROR:expected [status, id, status]";
        if (args[1].empty() || args[1].size() > 18) return "ERROR:invalid id";
        for (char c : args[1]) {
            if (!std::isdigit(static_cast<unsigned char>(c))) return "ERROR:invalid id";
        }
        const std::string& st = args[2];
        if (st != "spawned" && st != "garaged" && st != "impounded" && st != "destroyed") return "ERROR:invalid status";
        if (!db.VehicleStatus(args[1], st, error)) return "ERROR:" + error;
        return "OK";
    }
    auto validId = [](const std::string& s) {
        if (s.empty() || s.size() > 18) return false;
        for (char c : s) {
            if (!std::isdigit(static_cast<unsigned char>(c))) return false;
        }
        return true;
    };
    auto validJson = [](const std::string& s) { return !s.empty() && s.size() <= 8000 && (s[0] == '[' || s[0] == '{'); };
    if (op == "list") {
        if (args.size() < 3 || !IsSteam64(args[1])) return "ERROR:expected [list, uid, faction]";
        if (args[2] != "civ" && args[2] != "cop" && args[2] != "medic") return "ERROR:invalid faction";
        std::string out;
        if (!db.VehicleList(args[1], args[2], out, error)) return "ERROR:" + error;
        return out;
    }
    if (op == "get") {
        if (args.size() < 2 || !validId(args[1])) return "ERROR:invalid id";
        std::string out;
        if (!db.VehicleGet(args[1], out, error)) return "ERROR:" + error;
        return out;
    }
    if (op == "store") {
        if (args.size() < 6 || !validId(args[1])) return "ERROR:expected [store, id, status, fuel, damage, inventory]";
        if (args[2] != "garaged" && args[2] != "impounded") return "ERROR:invalid status";
        if (!IsPrice(args[3])) return "ERROR:invalid fuel";
        if (!validJson(args[4]) || !validJson(args[5])) return "ERROR:invalid json";
        if (!db.VehicleStore(args[1], args[2], args[3], args[4], args[5], error)) return "ERROR:" + error;
        return "OK";
    }
    if (op == "inventory") {
        if (args.size() < 3 || !validId(args[1]) || !validJson(args[2])) return "ERROR:expected [inventory, id, json]";
        if (!db.VehicleInventory(args[1], args[2], error)) return "ERROR:" + error;
        return "OK";
    }
    if (op == "reset") {
        std::string count;
        if (!db.VehicleReset(count, error)) return "ERROR:" + error;
        return "OK:" + count;
    }
    if (op == "log") {
        if (args.size() < 5 || !validId(args[1])) return "ERROR:expected [log, id, uid, action, amount]";
        if (!args[2].empty() && !IsSteam64(args[2])) return "ERROR:invalid uid";
        if (args[3] != "impound" && args[3] != "return") return "ERROR:invalid action";
        if (!args[4].empty() && !validId(args[4])) return "ERROR:invalid amount";
        if (!db.VehicleLog(args[1], args[2], args[3], args[4], error)) return "ERROR:" + error;
        return "OK";
    }
    return "ERROR:unknown vehicle op";
}


// The game's Admin app (ALife_fnc_adminAction). The mission has already
// checked the staff member's permissions; this only validates and writes.
//   ["log",        staffUid, targetUid|"", action, reason, afterJson] -> "OK"
//   ["ban",        staffUid, targetUid, minutes (0 = permanent), reason] -> "OK" / "ALREADY"
//   ["kick",       staffUid, targetUid, reason] -> "OK"
//   ["compensate", staffUid, targetUid, faction, amount, reason, "self"?] -> "OK:<balance>" / "NO_ACCOUNT"
//      (staffUid == targetUid only with "self": a server owner, CfgALife >> owners[])
std::string HandleAdmin(Database& db, const std::vector<std::string>& args) {
    if (args.size() < 3) return "ERROR:expected [op, staffUid, targetUid, ...]";
    const std::string& op = args[0];
    if (!IsSteam64(args[1])) return "ERROR:invalid staff uid";
    if (!(args[2].empty() && op == "log") && !IsSteam64(args[2])) return "ERROR:invalid target uid";
    auto digits = [](const std::string& s, size_t max) {
        if (s.empty() || s.size() > max) return false;
        for (char c : s) {
            if (c < '0' || c > '9') return false;
        }
        return true;
    };
    auto reasonOk = [](const std::string& s) { return !s.empty() && s.size() <= 300; };
    std::string error;
    std::string status;

    if (op == "log") {
        if (args.size() < 6) return "ERROR:expected [log, staffUid, targetUid, action, reason, afterJson]";
        const std::string& action = args[3];
        if (action.empty() || action.size() > 40) return "ERROR:invalid action";
        for (char c : action) {
            if (!((c >= 'a' && c <= 'z') || c == '_' || c == '.')) return "ERROR:invalid action";
        }
        if (args[4].size() > 300 || args[5].size() > 2000) return "ERROR:too long";
        if (!db.AdminLog(args[1], args[2], action, args[4], args[5], error)) return "ERROR:" + error;
        return "OK";
    }
    if (op == "ban") {
        if (args.size() < 5 || !digits(args[3], 6) || !reasonOk(args[4])) return "ERROR:expected [ban, staffUid, targetUid, minutes, reason]";
        if (!db.AdminBan(args[1], args[2], std::stoi(args[3]), args[4], status, error)) return "ERROR:" + error;
        return status;
    }
    if (op == "kick") {
        if (args.size() < 4 || !reasonOk(args[3])) return "ERROR:expected [kick, staffUid, targetUid, reason]";
        if (!db.AdminKick(args[1], args[2], args[3], error)) return "ERROR:" + error;
        return "OK";
    }
    if (op == "compensate") {
        if (args.size() < 6 || !digits(args[4], 9) || !reasonOk(args[5])) return "ERROR:expected [compensate, staffUid, targetUid, faction, amount, reason]";
        if (args[3] != "civ" && args[3] != "cop" && args[3] != "medic") return "ERROR:invalid faction";
        long long amount = std::stoll(args[4]);
        if (amount < 1 || amount > 100000000) return "ERROR:amount must be $1 to $100,000,000";
        if (args[1] == args[2] && (args.size() < 7 || args[6] != "self")) return "ERROR:you can't compensate yourself";
        if (!db.AdminCompensate(args[1], args[2], args[3], amount, args[5], status, error)) return "ERROR:" + error;
        return status;
    }
    return "ERROR:unknown admin op";
}

}  // namespace

std::string UnwrapArmaString(const std::string& arg) {
    if (arg.size() < 2 || arg.front() != '"' || arg.back() != '"') {
        return arg;
    }
    std::string out;
    out.reserve(arg.size() - 2);
    for (size_t i = 1; i + 1 < arg.size(); ++i) {
        out.push_back(arg[i]);
        if (arg[i] == '"' && i + 2 < arg.size() && arg[i + 1] == '"') {
            ++i;  // "" -> "
        }
    }
    return out;
}

bool IsSteam64(const std::string& uid) {
    if (uid.size() != 17) return false;
    for (char c : uid) {
        if (c < '0' || c > '9') return false;
    }
    return true;
}

std::string DispatchCommand(Database& db, const std::string& command,
                             const std::vector<std::string>& args) {
    if (command == "ping") return HandlePing(db);
    if (command == "load") return HandleLoad(db, args);
    if (command == "save") return HandleSave(db, args);
    if (command == "combatlog") return HandleCombatLog(db, args);
    if (command == "bank") return HandleBank(db, args);
    if (command == "session") return HandleSession(db, args);
    if (command == "market") return HandleMarket(db, args);
    if (command == "vehicle") return HandleVehicle(db, args);
    if (command == "admin") return HandleAdmin(db, args);

    return "ERROR:unknown command";
}
