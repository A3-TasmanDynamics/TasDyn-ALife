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
    if (op != "purchase" && op != "transfer") return "ERROR:unknown bank op";
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
    if (!db.BankTx(op, args[1], args[2], amount, args[4], args[5], args[6], args[7], status, error)) {
        return "ERROR:" + error;
    }
    return status;
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

    return "ERROR:unknown command";
}
