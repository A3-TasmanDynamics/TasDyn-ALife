#include "db.h"

#include <array>
#include <cstring>
#include <sstream>

namespace {

// Escapes a plain-text value for embedding inside a double-quoted SQF
// string literal (only used for genuinely free-text fields -- name, dept,
// staff_rank_key -- never for the JSONB array-of-pairs fields, which are
// embedded verbatim since they're already valid array literals).
//
// SQF (unlike C/JSON) escapes an embedded quote by *doubling* it (`""`),
// not with a backslash -- confirmed against Bohemia's own SQF Syntax page.
// This used to backslash-escape, which parseSimpleArray doesn't
// understand at all: a value that actually contained a `"` produced a
// literal `\"` in the response text, and parseSimpleArray choked on it
// with "parseSimpleArray format error" the first time a real value ever
// exercised this path (a uid that had picked up a stray pair of quotes
// from a separate bug -- see fn_callExtension.sqf -- when re-embedded in
// the response's own uid field). Backslash isn't special in SQF string
// literals at all, so it needs no escaping of its own.
std::string EscapeStringLiteral(const std::string& raw) {
    std::string out;
    out.reserve(raw.size() + 8);
    for (char c : raw) {
        if (c == '"') out.push_back('"');
        out.push_back(c);
    }
    return out;
}

void AppendComma(std::string& out, bool& first) {
    if (!first) out += ",";
    first = false;
}

void AppendStringKv(std::string& out, bool& first, const char* key, const std::string& value) {
    AppendComma(out, first);
    out += "[\"" + std::string(key) + "\",\"" + EscapeStringLiteral(value) + "\"]";
}

// value is text libpq already gives us in a form that's valid embedded as-is
// -- numbers, or a JSONB column's raw text (already a valid array literal
// per the JSONB SHAPE RULE in database/schema.sql).
void AppendRawKv(std::string& out, bool& first, const char* key, const std::string& rawValue) {
    AppendComma(out, first);
    out += "[\"" + std::string(key) + "\"," + rawValue + "]";
}

void AppendBoolKv(std::string& out, bool& first, const char* key, const std::string& pgBoolText) {
    AppendComma(out, first);
    out += "[\"" + std::string(key) + "\"," + (pgBoolText == "t" ? "true" : "false") + "]";
}

bool IsUniqueViolation(PGresult* res) {
    const char* sqlState = PQresultErrorField(res, PG_DIAG_SQLSTATE);
    return sqlState && std::strcmp(sqlState, "23505") == 0;
}

// One entry per absolute-set `save` field (everything except *_cash, which
// is a delta -- see SaveCashDelta). The SQL text is always a fixed literal,
// selected by matching `field` against this table -- never built from it.
struct SaveFieldSpec {
    const char* field;
    const char* sql;  // $1 = uid, $2 = value
};

constexpr std::array<SaveFieldSpec, 20> kSaveFields{{
    {"name", "UPDATE players SET name = $2, updated_at = now() WHERE uid = $1"},
    {"civ_licence", "UPDATE players SET civ_licence = $2::jsonb, updated_at = now() WHERE uid = $1"},
    {"cop_licence", "UPDATE players SET cop_licence = $2::jsonb, updated_at = now() WHERE uid = $1"},
    {"medic_licence", "UPDATE players SET medic_licence = $2::jsonb, updated_at = now() WHERE uid = $1"},
    {"civ_gear", "UPDATE players SET civ_gear = $2::jsonb, updated_at = now() WHERE uid = $1"},
    {"cop_gear", "UPDATE players SET cop_gear = $2::jsonb, updated_at = now() WHERE uid = $1"},
    {"medic_gear", "UPDATE players SET medic_gear = $2::jsonb, updated_at = now() WHERE uid = $1"},
    {"cop_level", "UPDATE players SET cop_level = $2::integer, updated_at = now() WHERE uid = $1"},
    {"medic_level", "UPDATE players SET medic_level = $2::integer, updated_at = now() WHERE uid = $1"},
    {"cop_dept", "UPDATE players SET cop_dept = $2, updated_at = now() WHERE uid = $1"},
    {"medic_dept", "UPDATE players SET medic_dept = $2, updated_at = now() WHERE uid = $1"},
    {"civ_alive", "UPDATE players SET civ_alive = $2::boolean, updated_at = now() WHERE uid = $1"},
    {"cop_alive", "UPDATE players SET cop_alive = $2::boolean, updated_at = now() WHERE uid = $1"},
    {"medic_alive", "UPDATE players SET medic_alive = $2::boolean, updated_at = now() WHERE uid = $1"},
    {"civ_position", "UPDATE players SET civ_position = $2::jsonb, updated_at = now() WHERE uid = $1"},
    {"cop_position", "UPDATE players SET cop_position = $2::jsonb, updated_at = now() WHERE uid = $1"},
    {"medic_position", "UPDATE players SET medic_position = $2::jsonb, updated_at = now() WHERE uid = $1"},
    {"civ_items", "UPDATE players SET civ_items = $2::jsonb, updated_at = now() WHERE uid = $1"},
    {"cop_items", "UPDATE players SET cop_items = $2::jsonb, updated_at = now() WHERE uid = $1"},
    {"medic_items", "UPDATE players SET medic_items = $2::jsonb, updated_at = now() WHERE uid = $1"},
}};

}  // namespace

Database::~Database() {
    Disconnect();
}

bool Database::Connect(const DbConfig& config, std::string& outError) {
    std::lock_guard<std::mutex> lock(mutex_);

    std::string connInfo =
        "host=" + config.host +
        " port=" + config.port +
        " dbname=" + config.dbname +
        " user=" + config.user +
        " password=" + config.password +
        " connect_timeout=5" +
        // Lets database triggers attribute changes made by the game server
        // (e.g. an in-game faction promotion) to source 'game' in
        // rank_changes/staff_log -- see docs/INTEGRATIONS.md §3.1.
        " application_name=tasdyn-extension";

    conn_ = PQconnectdb(connInfo.c_str());

    if (PQstatus(conn_) != CONNECTION_OK) {
        outError = PQerrorMessage(conn_);
        PQfinish(conn_);
        conn_ = nullptr;
        return false;
    }

    return true;
}

void Database::Disconnect() {
    std::lock_guard<std::mutex> lock(mutex_);
    if (conn_) {
        PQfinish(conn_);
        conn_ = nullptr;
    }
}

bool Database::IsConnected() const {
    std::lock_guard<std::mutex> lock(mutex_);
    return conn_ != nullptr && PQstatus(conn_) == CONNECTION_OK;
}

bool Database::EnsureConnected() {
    // Caller holds mutex_.
    if (!conn_) return false;  // Connect() was never called successfully -- nothing to reset

    // PQstatus() only reflects libpq's last-known state -- it does NOT
    // proactively detect a connection the server side already killed
    // (pg_terminate_backend, a restart, a network blip). The client only
    // finds out by actually trying something. Confirmed with a live test:
    // checking PQstatus() alone and trusting CONNECTION_OK let a killed
    // connection straight through, and the real query failed right after
    // with "server closed the connection unexpectedly" -- so this probes
    // for real instead of trusting the cached flag.
    if (PQstatus(conn_) == CONNECTION_OK) {
        PGresult* probe = PQexec(conn_, "SELECT 1");
        bool alive = (PQresultStatus(probe) == PGRES_TUPLES_OK);
        PQclear(probe);
        if (alive) return true;
    }

    PQreset(conn_);
    return PQstatus(conn_) == CONNECTION_OK;
}

bool Database::Ping(std::string& outError) {
    std::lock_guard<std::mutex> lock(mutex_);

    if (!EnsureConnected()) {
        outError = "not connected";
        return false;
    }

    PGresult* res = PQexecParams(conn_, "SELECT 1", 0, nullptr, nullptr, nullptr, nullptr, 0);
    bool ok = (PQresultStatus(res) == PGRES_TUPLES_OK);
    if (!ok) outError = PQerrorMessage(conn_);
    PQclear(res);
    return ok;
}

bool Database::EnsureBlankPlayer(const std::string& uid, std::string& outError) {
    // Caller holds mutex_.
    const char* insertParams[1] = {uid.c_str()};
    PGresult* insertRes = PQexecParams(
        conn_,
        "INSERT INTO players (uid) VALUES ($1) ON CONFLICT (uid) DO NOTHING RETURNING id",
        1, nullptr, insertParams, nullptr, nullptr, 0);

    if (PQresultStatus(insertRes) != PGRES_TUPLES_OK) {
        outError = PQerrorMessage(conn_);
        PQclear(insertRes);
        return false;
    }

    bool wasCreated = (PQntuples(insertRes) > 0);
    std::string newId = wasCreated ? PQgetvalue(insertRes, 0, 0) : "";
    PQclear(insertRes);

    if (!wasCreated) return true;  // already existed, nothing else to do

    // New player: seed one bank_accounts row per faction, same transaction
    // as far as this call is concerned -- not wrapping in an explicit
    // BEGIN/COMMIT since it's a single statement inserting all three rows.
    const char* bankParams[1] = {newId.c_str()};
    PGresult* bankRes = PQexecParams(
        conn_,
        "INSERT INTO bank_accounts (player_id, faction, balance) "
        "VALUES ($1, 'civilian', 0), ($1, 'police', 0), ($1, 'medic', 0)",
        1, nullptr, bankParams, nullptr, nullptr, 0);

    bool ok = (PQresultStatus(bankRes) == PGRES_COMMAND_OK);
    if (!ok) outError = PQerrorMessage(conn_);
    PQclear(bankRes);
    return ok;
}

bool Database::LoadPlayer(const std::string& uid, std::string& outResponse, std::string& outError) {
    std::lock_guard<std::mutex> lock(mutex_);

    if (!EnsureConnected()) {
        outError = "not connected";
        return false;
    }

    // Bans are checked before anything else (and before a blank record is
    // created): an active banlist row -- not expired; lifting a ban sets
    // expires_at to the lift time -- answers BANNED with the reason and end
    // time instead of the player's record, so the server never spawns them.
    {
        const char* banParams[1] = {uid.c_str()};
        PGresult* banRes = PQexecParams(
            conn_,
            "SELECT reason, COALESCE(to_char(expires_at AT TIME ZONE 'UTC', 'DD Mon YYYY HH24:MI \"UTC\"'), '') "
            "FROM banlist WHERE uid = $1 AND (expires_at IS NULL OR expires_at > now()) "
            "ORDER BY expires_at DESC NULLS FIRST LIMIT 1",
            1, nullptr, banParams, nullptr, nullptr, 0);
        if (PQresultStatus(banRes) != PGRES_TUPLES_OK) {
            outError = PQerrorMessage(conn_);
            PQclear(banRes);
            return false;
        }
        if (PQntuples(banRes) > 0) {
            std::string ban = "[[\"status\",\"BANNED\"]";
            bool first = false;
            AppendStringKv(ban, first, "reason", PQgetvalue(banRes, 0, 0));
            AppendStringKv(ban, first, "until", PQgetvalue(banRes, 0, 1));
            ban += "]";
            PQclear(banRes);
            outResponse = ban;
            return true;
        }
        PQclear(banRes);
    }

    if (!EnsureBlankPlayer(uid, outError)) return false;

    const char* selectParams[1] = {uid.c_str()};
    PGresult* res = PQexecParams(
        conn_,
        "SELECT p.uid, p.name, COALESCE(sr.key, ''), COALESCE(sr.level, 0), "
        "       p.civ_cash, p.cop_cash, p.medic_cash, "
        "       p.civ_bank, p.cop_bank, p.medic_bank, "
        "       p.cop_level, p.medic_level, "
        "       COALESCE(p.cop_dept, ''), COALESCE(p.medic_dept, ''), "
        "       p.civ_licence::text, p.cop_licence::text, p.medic_licence::text, "
        "       p.civ_gear::text, p.cop_gear::text, p.medic_gear::text, "
        "       p.civ_alive, p.cop_alive, p.medic_alive, "
        "       p.civ_position::text, p.cop_position::text, p.medic_position::text, "
        "       p.civ_bounty, "
        "       p.civ_items::text, p.cop_items::text, p.medic_items::text, "
        "       COALESCE(g.tag, ''), COALESCE(g.name, ''), "
        "       COALESCE((SELECT name FROM faction_rank_names WHERE faction = 'police' AND level = p.cop_level), ''), "
        "       COALESCE((SELECT name FROM faction_rank_names WHERE faction = 'ems' AND level = p.medic_level), '') "
        "FROM players p LEFT JOIN staff_ranks sr ON sr.id = p.staff_rank_id "
        "LEFT JOIN gang_members gm ON gm.player_id = p.id "
        "LEFT JOIN gangs g ON g.id = gm.gang_id "
        "WHERE p.uid = $1",
        1, nullptr, selectParams, nullptr, nullptr, 0);

    if (PQresultStatus(res) != PGRES_TUPLES_OK || PQntuples(res) == 0) {
        outError = (PQntuples(res) == 0) ? "player not found after ensure-created" : PQerrorMessage(conn_);
        PQclear(res);
        return false;
    }

    auto col = [&](int i) { return std::string(PQgetvalue(res, 0, i)); };

    std::string out = "[";
    bool first = true;
    AppendStringKv(out, first, "uid", col(0));
    AppendStringKv(out, first, "name", col(1));
    AppendStringKv(out, first, "staff_rank_key", col(2));
    AppendRawKv(out, first, "staff_level", col(3));
    AppendRawKv(out, first, "civ_cash", col(4));
    AppendRawKv(out, first, "cop_cash", col(5));
    AppendRawKv(out, first, "medic_cash", col(6));
    AppendRawKv(out, first, "civ_bank", col(7));
    AppendRawKv(out, first, "cop_bank", col(8));
    AppendRawKv(out, first, "medic_bank", col(9));
    AppendRawKv(out, first, "cop_level", col(10));
    AppendRawKv(out, first, "medic_level", col(11));
    AppendStringKv(out, first, "cop_dept", col(12));
    AppendStringKv(out, first, "medic_dept", col(13));
    AppendRawKv(out, first, "civ_licence", col(14));
    AppendRawKv(out, first, "cop_licence", col(15));
    AppendRawKv(out, first, "medic_licence", col(16));
    AppendRawKv(out, first, "civ_gear", col(17));
    AppendRawKv(out, first, "cop_gear", col(18));
    AppendRawKv(out, first, "medic_gear", col(19));
    AppendBoolKv(out, first, "civ_alive", col(20));
    AppendBoolKv(out, first, "cop_alive", col(21));
    AppendBoolKv(out, first, "medic_alive", col(22));
    AppendRawKv(out, first, "civ_position", col(23));
    AppendRawKv(out, first, "cop_position", col(24));
    AppendRawKv(out, first, "medic_position", col(25));
    AppendRawKv(out, first, "civ_bounty", col(26));
    AppendRawKv(out, first, "civ_items", col(27));
    AppendRawKv(out, first, "cop_items", col(28));
    AppendRawKv(out, first, "medic_items", col(29));
    // The player's gang (website-managed gangs / gang_members), "" if none:
    // in-game it picks gang uniform textures (CfgClothing UniformTextures).
    AppendStringKv(out, first, "gang_tag", col(30));
    AppendStringKv(out, first, "gang_name", col(31));
    // Their police / EMS rank names (faction_rank_names, edited on the
    // website), "" if their level has no name: shown on player tags.
    AppendStringKv(out, first, "cop_rank", col(32));
    AppendStringKv(out, first, "medic_rank", col(33));
    PQclear(res);

    // Their effective staff permissions, resolved exactly like the website's
    // auth.Effective: none unless they have a rank and staff_status is
    // 'active'; the rank's rank_permissions, then per-player overrides
    // (allow adds, deny removes) -- except the NoOverride keys, which only a
    // rank can give (keep this list in step with permissions.go). A JSON
    // array of strings is also a valid SQF array.
    {
        const char* permParams[1] = {uid.c_str()};
        PGresult* permRes = PQexecParams(
            conn_,
            "WITH p AS (SELECT id, staff_rank_id FROM players "
            "           WHERE uid = $1 AND staff_rank_id IS NOT NULL AND staff_status = 'active'), "
            "no_override(k) AS (VALUES ('staff.edit'), ('staff.suspend'), ('staff.remove'), ('roles.manage'), "
            "    ('players.edit_police'), ('players.edit_medic'), ('server.control'), ('database.query')), "
            "o AS (SELECT o.command_key, o.allow FROM staff_permission_overrides o JOIN p ON p.id = o.player_id "
            "      WHERE o.command_key NOT IN (SELECT k FROM no_override)) "
            "SELECT COALESCE(json_agg(k ORDER BY k)::text, '[]') FROM ("
            "  SELECT rp.command_key AS k FROM rank_permissions rp JOIN p ON rp.rank_id = p.staff_rank_id "
            "  WHERE NOT EXISTS (SELECT 1 FROM o WHERE o.command_key = rp.command_key AND NOT o.allow) "
            "  UNION SELECT command_key FROM o WHERE allow) s",
            1, nullptr, permParams, nullptr, nullptr, 0);
        std::string perms = "[]";
        if (PQresultStatus(permRes) == PGRES_TUPLES_OK && PQntuples(permRes) == 1) perms = PQgetvalue(permRes, 0, 0);
        PQclear(permRes);
        AppendRawKv(out, first, "staff_perms", perms);
    }
    out += "]";

    outResponse = "[[\"status\",\"OK\"]," + out.substr(1);  // splice status in as the first pair
    return true;
}

bool Database::SaveCashDelta(const std::string& uid, const std::string& faction, const std::string& value,
                              const std::string& token, std::string& outStatus, std::string& outError) {
    // Caller holds mutex_.
    PQexec(conn_, "BEGIN");

    const char* idParams[1] = {uid.c_str()};
    PGresult* idRes = PQexecParams(conn_, "SELECT id FROM players WHERE uid = $1", 1, nullptr, idParams,
                                    nullptr, nullptr, 0);
    if (PQresultStatus(idRes) != PGRES_TUPLES_OK || PQntuples(idRes) == 0) {
        outError = (PQntuples(idRes) == 0) ? "unknown uid" : PQerrorMessage(conn_);
        PQclear(idRes);
        PQexec(conn_, "ROLLBACK");
        outStatus = "ERROR";
        return true;
    }
    std::string playerId = PQgetvalue(idRes, 0, 0);
    PQclear(idRes);

    const char* tokenParams[2] = {playerId.c_str(), token.c_str()};
    PGresult* tokenRes = PQexecParams(
        conn_, "INSERT INTO applied_request_tokens (player_id, token) VALUES ($1, $2)",
        2, nullptr, tokenParams, nullptr, nullptr, 0);

    if (PQresultStatus(tokenRes) != PGRES_COMMAND_OK) {
        bool duplicate = IsUniqueViolation(tokenRes);
        if (!duplicate) outError = PQerrorMessage(conn_);
        PQclear(tokenRes);
        PQexec(conn_, "ROLLBACK");
        outStatus = duplicate ? "DUPLICATE" : "ERROR";
        return true;
    }
    PQclear(tokenRes);

    // faction is one of "civ"/"cop"/"medic", matched in SaveField below --
    // never taken from `field` and used verbatim, even though the values
    // happen to be the same trusted set.
    std::string sql = "UPDATE players SET " + faction + "_cash = " + faction + "_cash + $2::bigint, " +
                       "updated_at = now() WHERE uid = $1 AND " + faction + "_cash + $2::bigint >= 0";
    const char* updateParams[2] = {uid.c_str(), value.c_str()};
    PGresult* updateRes = PQexecParams(conn_, sql.c_str(), 2, nullptr, updateParams, nullptr, nullptr, 0);

    bool updated = (PQresultStatus(updateRes) == PGRES_COMMAND_OK) &&
                    std::strcmp(PQcmdTuples(updateRes), "0") != 0;
    if (!updated && PQresultStatus(updateRes) != PGRES_COMMAND_OK) outError = PQerrorMessage(conn_);
    PQclear(updateRes);

    PQexec(conn_, updated ? "COMMIT" : "ROLLBACK");
    outStatus = updated ? "OK" : "ERROR";
    return true;
}

bool Database::SaveField(const std::string& uid, const std::string& field, const std::string& value,
                          const std::string& token, std::string& outStatus, std::string& outError) {
    std::lock_guard<std::mutex> lock(mutex_);

    if (!EnsureConnected()) {
        outError = "not connected";
        return false;
    }

    if (field == "civ_cash") return SaveCashDelta(uid, "civ", value, token, outStatus, outError);
    if (field == "cop_cash") return SaveCashDelta(uid, "cop", value, token, outStatus, outError);
    if (field == "medic_cash") return SaveCashDelta(uid, "medic", value, token, outStatus, outError);

    for (const auto& spec : kSaveFields) {
        if (field != spec.field) continue;

        const char* params[2] = {uid.c_str(), value.c_str()};
        PGresult* res = PQexecParams(conn_, spec.sql, 2, nullptr, params, nullptr, nullptr, 0);

        bool ok = (PQresultStatus(res) == PGRES_COMMAND_OK);
        if (!ok) outError = PQerrorMessage(conn_);
        PQclear(res);

        outStatus = ok ? "OK" : "ERROR";
        return true;
    }

    // field isn't in the allowlist at all.
    outStatus = "ERROR";
    outError = "field not allowlisted: " + field;
    return true;
}

bool Database::RecordCombatLog(const std::string& uid, const std::string& faction, const std::string& killer,
                                const std::string& weapon, const std::string& distance,
                                const std::string& secondsIncapacitated, std::string& outError) {
    std::lock_guard<std::mutex> lock(mutex_);

    if (!EnsureConnected()) {
        outError = "not connected";
        return false;
    }

    const char* params[6] = {uid.c_str(), faction.c_str(), killer.c_str(),
                             weapon.c_str(), distance.c_str(), secondsIncapacitated.c_str()};
    PGresult* res = PQexecParams(
        conn_,
        "WITH p AS (SELECT id FROM players WHERE uid = $1), "
        "d AS (SELECT jsonb_strip_nulls(jsonb_build_object("
        "        'faction', $2::text, "
        "        'killed_by', NULLIF($3::text, ''), "
        "        'weapon', NULLIF($4::text, ''), "
        "        'distance_m', NULLIF($5::text, '')::int, "
        "        'seconds_incapacitated', NULLIF($6::text, '')::int)) AS details), "
        "f AS (INSERT INTO anti_cheat_flags (player_id, flag_type, confidence, details) "
        "      SELECT p.id, 'combat_log', 'high', d.details FROM p, d RETURNING player_id) "
        "INSERT INTO staff_log (target_player_id, action, reason, after_value, source) "
        "SELECT f.player_id, 'player.combat_log', 'Disconnected while incapacitated', d.details, 'game' "
        "FROM f, d",
        6, nullptr, params, nullptr, nullptr, 0);

    bool ok = (PQresultStatus(res) == PGRES_COMMAND_OK);
    if (!ok) {
        outError = PQerrorMessage(conn_);
    } else if (std::strcmp(PQcmdTuples(res), "0") == 0) {
        ok = false;
        outError = "unknown uid";
    }
    PQclear(res);
    return ok;
}

namespace {

// "civ"/"cop"/"medic" (the mission's faction keys) -> bank_accounts.faction.
const char* BankFaction(const std::string& faction) {
    if (faction == "civ") return "civilian";
    if (faction == "cop") return "police";
    if (faction == "medic") return "medic";
    return nullptr;
}

}  // namespace

bool Database::BankTx(const std::string& op, const std::string& uid, const std::string& faction,
                      const std::string& amount, const std::string& token, const std::string& otherUid,
                      const std::string& otherFaction, const std::string& memo, std::string& outStatus,
                      std::string& outError) {
    std::lock_guard<std::mutex> lock(mutex_);

    if (!EnsureConnected()) {
        outError = "not connected";
        return false;
    }

    const bool transfer = (op == "transfer");
    const char* bankFaction = BankFaction(faction);
    const char* otherBankFaction = transfer ? BankFaction(otherFaction) : nullptr;
    if ((op != "purchase" && !transfer) || !bankFaction || (transfer && !otherBankFaction)) {
        outStatus = "ERROR";
        outError = "bad bank operation";
        return true;
    }

    auto exec = [&](const char* sql, int n, const char* const* params) {
        return PQexecParams(conn_, sql, n, nullptr, params, nullptr, nullptr, 0);
    };
    auto fail = [&](PGresult* res, const char* status) {
        if (res) {
            outError = PQerrorMessage(conn_);
            PQclear(res);
        }
        PQclear(PQexec(conn_, "ROLLBACK"));
        outStatus = status;
        return true;
    };

    PQclear(PQexec(conn_, "BEGIN"));

    // Accounts are seeded when a player is created; make sure older
    // records have them too, then lock both in a fixed (id) order so two
    // opposite transfers can't deadlock -- same as the website's moveMoney.
    const char* ensureSql =
        "INSERT INTO bank_accounts (player_id, faction, balance) "
        "SELECT id, $2, 0 FROM players WHERE uid = $1 ON CONFLICT (player_id, faction) DO NOTHING";
    const char* lookupSql =
        "SELECT b.id, b.balance, p.id FROM bank_accounts b JOIN players p ON p.id = b.player_id "
        "WHERE p.uid = $1 AND b.faction = $2";

    const char* fromParams[2] = {uid.c_str(), bankFaction};
    PQclear(exec(ensureSql, 2, fromParams));
    const char* toParams[2] = {otherUid.c_str(), otherBankFaction};
    if (transfer) PQclear(exec(ensureSql, 2, toParams));

    PGresult* fromRes = exec(lookupSql, 2, fromParams);
    if (PQresultStatus(fromRes) != PGRES_TUPLES_OK) return fail(fromRes, "ERROR");
    if (PQntuples(fromRes) == 0) {
        PQclear(fromRes);
        return fail(nullptr, "NO_ACCOUNT");
    }
    std::string fromId = PQgetvalue(fromRes, 0, 0);
    std::string playerId = PQgetvalue(fromRes, 0, 2);
    PQclear(fromRes);

    std::string toId;
    if (transfer) {
        PGresult* toRes = exec(lookupSql, 2, toParams);
        if (PQresultStatus(toRes) != PGRES_TUPLES_OK) return fail(toRes, "ERROR");
        if (PQntuples(toRes) == 0) {
            PQclear(toRes);
            return fail(nullptr, "NO_ACCOUNT");
        }
        toId = PQgetvalue(toRes, 0, 0);
        PQclear(toRes);
        if (toId == fromId) return fail(nullptr, "ERROR");
    }

    const char* lockParams[2] = {fromId.c_str(), toId.empty() ? fromId.c_str() : toId.c_str()};
    PGresult* lockRes = exec(
        "SELECT id FROM bank_accounts WHERE id IN ($1::bigint, $2::bigint) ORDER BY id FOR UPDATE", 2, lockParams);
    if (PQresultStatus(lockRes) != PGRES_TUPLES_OK) return fail(lockRes, "ERROR");
    PQclear(lockRes);

    // The debit is only inserted if the balance (re-read under the lock)
    // covers it. The bank_transactions trigger then updates the balance.
    const char* debitParams[6] = {fromId.c_str(), amount.c_str(), token.c_str(),
                                  toId.empty() ? nullptr : toId.c_str(), memo.c_str(), playerId.c_str()};
    PGresult* debitRes = exec(
        transfer
            ? "INSERT INTO bank_transactions (account_id, type, amount, balance_after, related_account_id, memo, request_token, created_by) "
              "SELECT id, 'transfer_out', -$2::bigint, balance - $2::bigint, $4::bigint, NULLIF($5, ''), $3, $6::bigint "
              "FROM bank_accounts WHERE id = $1::bigint AND balance >= $2::bigint RETURNING id"
            : "INSERT INTO bank_transactions (account_id, type, amount, balance_after, related_account_id, memo, request_token, created_by) "
              "SELECT id, 'purchase', -$2::bigint, balance - $2::bigint, $4::bigint, NULLIF($5, ''), $3, $6::bigint "
              "FROM bank_accounts WHERE id = $1::bigint AND balance >= $2::bigint RETURNING id",
        6, debitParams);
    if (PQresultStatus(debitRes) != PGRES_TUPLES_OK) {
        bool duplicate = IsUniqueViolation(debitRes);
        return fail(debitRes, duplicate ? "DUPLICATE" : "ERROR");
    }
    bool covered = PQntuples(debitRes) > 0;
    PQclear(debitRes);
    if (!covered) return fail(nullptr, "INSUFFICIENT");

    if (transfer) {
        const char* creditParams[6] = {toId.c_str(), amount.c_str(), token.c_str(),
                                       fromId.c_str(), memo.c_str(), playerId.c_str()};
        PGresult* creditRes = exec(
            "INSERT INTO bank_transactions (account_id, type, amount, balance_after, related_account_id, memo, request_token, created_by) "
            "SELECT id, 'transfer_in', $2::bigint, balance + $2::bigint, $4::bigint, NULLIF($5, ''), $3, $6::bigint "
            "FROM bank_accounts WHERE id = $1::bigint",
            6, creditParams);
        if (PQresultStatus(creditRes) != PGRES_COMMAND_OK) {
            bool duplicate = IsUniqueViolation(creditRes);
            return fail(creditRes, duplicate ? "DUPLICATE" : "ERROR");
        }
        PQclear(creditRes);
    }

    PGresult* commitRes = PQexec(conn_, "COMMIT");
    bool committed = PQresultStatus(commitRes) == PGRES_COMMAND_OK;
    if (!committed) outError = PQerrorMessage(conn_);
    PQclear(commitRes);
    outStatus = committed ? "OK" : "ERROR";
    return true;
}

bool Database::BankCash(const std::string& op, const std::string& uid, const std::string& faction,
                        const std::string& amount, const std::string& token, std::string& outStatus,
                        std::string& outError) {
    std::lock_guard<std::mutex> lock(mutex_);

    if (!EnsureConnected()) {
        outError = "not connected";
        return false;
    }

    const bool deposit = (op == "deposit");
    const char* bankFaction = BankFaction(faction);
    // Fixed column names, picked by the validated faction -- never built from input.
    const char* cashColumn = faction == "civ" ? "civ_cash" : faction == "cop" ? "cop_cash" : faction == "medic" ? "medic_cash" : nullptr;
    if ((!deposit && op != "withdraw") || !bankFaction || !cashColumn) {
        outStatus = "ERROR";
        outError = "bad bank operation";
        return true;
    }

    auto exec = [&](const std::string& sql, int n, const char* const* params) {
        return PQexecParams(conn_, sql.c_str(), n, nullptr, params, nullptr, nullptr, 0);
    };
    auto fail = [&](PGresult* res, const char* status) {
        if (res) {
            outError = PQerrorMessage(conn_);
            PQclear(res);
        }
        PQclear(PQexec(conn_, "ROLLBACK"));
        outStatus = status;
        return true;
    };

    PQclear(PQexec(conn_, "BEGIN"));

    const char* idParams[2] = {uid.c_str(), bankFaction};
    PQclear(exec("INSERT INTO bank_accounts (player_id, faction, balance) "
                 "SELECT id, $2, 0 FROM players WHERE uid = $1 ON CONFLICT (player_id, faction) DO NOTHING",
                 2, idParams));
    PGresult* idRes = exec("SELECT b.id, p.id FROM bank_accounts b JOIN players p ON p.id = b.player_id "
                           "WHERE p.uid = $1 AND b.faction = $2 FOR UPDATE OF b, p",
                           2, idParams);
    if (PQresultStatus(idRes) != PGRES_TUPLES_OK) return fail(idRes, "ERROR");
    if (PQntuples(idRes) == 0) {
        PQclear(idRes);
        return fail(nullptr, "NO_ACCOUNT");
    }
    std::string accountId = PQgetvalue(idRes, 0, 0);
    std::string playerId = PQgetvalue(idRes, 0, 1);
    PQclear(idRes);

    const std::string cash = cashColumn;
    const char* cashParams[2] = {playerId.c_str(), amount.c_str()};
    const char* txParams[4] = {accountId.c_str(), amount.c_str(), token.c_str(), playerId.c_str()};

    if (deposit) {
        // Cash out of the pocket (only if it's there), then into the account.
        PGresult* cashRes = exec("UPDATE players SET " + cash + " = " + cash + " - $2::bigint, updated_at = now() "
                                 "WHERE id = $1::bigint AND " + cash + " >= $2::bigint",
                                 2, cashParams);
        if (PQresultStatus(cashRes) != PGRES_COMMAND_OK) return fail(cashRes, "ERROR");
        bool had = std::strcmp(PQcmdTuples(cashRes), "0") != 0;
        PQclear(cashRes);
        if (!had) return fail(nullptr, "INSUFFICIENT");

        PGresult* txRes = exec("INSERT INTO bank_transactions (account_id, type, amount, balance_after, memo, request_token, created_by) "
                               "SELECT id, 'deposit', $2::bigint, balance + $2::bigint, 'ATM deposit', $3, $4::bigint "
                               "FROM bank_accounts WHERE id = $1::bigint",
                               4, txParams);
        if (PQresultStatus(txRes) != PGRES_COMMAND_OK) {
            bool duplicate = IsUniqueViolation(txRes);
            return fail(txRes, duplicate ? "DUPLICATE" : "ERROR");
        }
        PQclear(txRes);
    } else {
        // Out of the account (only if the balance covers it), then into the pocket.
        PGresult* txRes = exec("INSERT INTO bank_transactions (account_id, type, amount, balance_after, memo, request_token, created_by) "
                               "SELECT id, 'withdrawal', -$2::bigint, balance - $2::bigint, 'ATM withdrawal', $3, $4::bigint "
                               "FROM bank_accounts WHERE id = $1::bigint AND balance >= $2::bigint RETURNING id",
                               4, txParams);
        if (PQresultStatus(txRes) != PGRES_TUPLES_OK) {
            bool duplicate = IsUniqueViolation(txRes);
            return fail(txRes, duplicate ? "DUPLICATE" : "ERROR");
        }
        bool covered = PQntuples(txRes) > 0;
        PQclear(txRes);
        if (!covered) return fail(nullptr, "INSUFFICIENT");

        PGresult* cashRes = exec("UPDATE players SET " + cash + " = " + cash + " + $2::bigint, updated_at = now() WHERE id = $1::bigint",
                                 2, cashParams);
        if (PQresultStatus(cashRes) != PGRES_COMMAND_OK) return fail(cashRes, "ERROR");
        PQclear(cashRes);
    }

    PGresult* commitRes = PQexec(conn_, "COMMIT");
    bool committed = PQresultStatus(commitRes) == PGRES_COMMAND_OK;
    if (!committed) outError = PQerrorMessage(conn_);
    PQclear(commitRes);
    outStatus = committed ? "OK" : "ERROR";
    return true;
}

bool Database::RecordSession(const std::string& op, const std::string& uid, std::string& outError) {
    std::lock_guard<std::mutex> lock(mutex_);

    if (!EnsureConnected()) {
        outError = "not connected";
        return false;
    }

    PGresult* res = nullptr;
    const char* params[1] = {uid.c_str()};
    if (op == "start") {
        // Close anything left open for this player (a crash, a missed
        // disconnect), then open the new session -- one statement.
        res = PQexecParams(conn_,
            "WITH p AS (SELECT id FROM players WHERE uid = $1), "
            "closed AS (UPDATE player_sessions SET disconnected_at = now() "
            "           WHERE disconnected_at IS NULL AND player_id = (SELECT id FROM p)) "
            "INSERT INTO player_sessions (player_id) SELECT id FROM p",
            1, nullptr, params, nullptr, nullptr, 0);
    } else if (op == "end") {
        res = PQexecParams(conn_,
            "UPDATE player_sessions SET disconnected_at = now() "
            "WHERE disconnected_at IS NULL AND player_id = (SELECT id FROM players WHERE uid = $1)",
            1, nullptr, params, nullptr, nullptr, 0);
    } else if (op == "reset") {
        // Server start: nobody is connected yet, so close every open session.
        res = PQexec(conn_, "UPDATE player_sessions SET disconnected_at = now() WHERE disconnected_at IS NULL");
    } else {
        outError = "unknown session op";
        return false;
    }

    bool ok = PQresultStatus(res) == PGRES_COMMAND_OK;
    if (!ok) outError = PQerrorMessage(conn_);
    PQclear(res);
    return ok;
}

// Dynamic market (market_prices): the current price of each virtual item.
// Items with no row are at their CfgMarket base price (a wipe or economy
// reset empties the table).
bool Database::MarketLoad(std::string& outResponse, std::string& outError) {
    std::lock_guard<std::mutex> lock(mutex_);

    if (!EnsureConnected()) {
        outError = "not connected";
        return false;
    }
    PGresult* res = PQexec(conn_, "SELECT item, price::text FROM market_prices ORDER BY item");
    if (PQresultStatus(res) != PGRES_TUPLES_OK) {
        outError = PQerrorMessage(conn_);
        PQclear(res);
        return false;
    }
    // [["water","12.40"],...] -- item names are config class names
    // (checked on the way in), prices plain decimals: nothing to escape.
    std::string out = "[";
    for (int i = 0; i < PQntuples(res); ++i) {
        if (i) out += ",";
        out += "[\"";
        out += PQgetvalue(res, i, 0);
        out += "\",";
        out += PQgetvalue(res, i, 1);
        out += "]";
    }
    out += "]";
    PQclear(res);
    outResponse = out;
    return true;
}

bool Database::MarketSet(const std::string& item, const std::string& price, std::string& outError) {
    std::lock_guard<std::mutex> lock(mutex_);

    if (!EnsureConnected()) {
        outError = "not connected";
        return false;
    }
    const char* params[2] = {item.c_str(), price.c_str()};
    PGresult* res = PQexecParams(conn_,
        "INSERT INTO market_prices (item, price) VALUES ($1, $2::numeric) "
        "ON CONFLICT (item) DO UPDATE SET price = EXCLUDED.price, updated_at = now()",
        2, nullptr, params, nullptr, nullptr, 0);
    bool ok = PQresultStatus(res) == PGRES_COMMAND_OK;
    if (!ok) outError = PQerrorMessage(conn_);
    PQclear(res);
    return ok;
}

bool Database::MarketReset(std::string& outError) {
    std::lock_guard<std::mutex> lock(mutex_);

    if (!EnsureConnected()) {
        outError = "not connected";
        return false;
    }
    PGresult* res = PQexec(conn_, "DELETE FROM market_prices");
    bool ok = PQresultStatus(res) == PGRES_COMMAND_OK;
    if (!ok) outError = PQerrorMessage(conn_);
    PQclear(res);
    return ok;
}

// A bought vehicle: a vehicles row (owner, class, faction, a unique plate,
// status 'spawned') and its 'purchase' vehicle_logs row, in one statement.
// The plate is drawn in SQL (letters and digits, no I/O/0/1) and redrawn if
// it's taken.
bool Database::VehicleAdd(const std::string& uid, const std::string& faction, const std::string& classname,
                          const std::string& price, const std::string& skin, std::string& outId,
                          std::string& outPlate, std::string& outError) {
    std::lock_guard<std::mutex> lock(mutex_);

    if (!EnsureConnected()) {
        outError = "not connected";
        return false;
    }
    const char* scope = faction == "cop" ? "police" : faction == "medic" ? "medic" : "civilian";
    const char* params[5] = {uid.c_str(), scope, classname.c_str(), price.c_str(), skin.c_str()};
    for (int attempt = 0; attempt < 5; ++attempt) {
        PGresult* res = PQexecParams(conn_,
            "WITH p AS (SELECT id FROM players WHERE uid = $1), "
            "v AS (INSERT INTO vehicles (owner_player_id, classname, faction_scope, status, skin, plate) "
            "      SELECT id, $3, $2, 'spawned', $5, "
            "             (SELECT string_agg(substr('ABCDEFGHJKLMNPQRSTUVWXYZ23456789', (floor(random() * 32) + 1)::int, 1), '') "
            "              FROM generate_series(1, 6)) "
            "      FROM p RETURNING id, plate, owner_player_id), "
            "l AS (INSERT INTO vehicle_logs (vehicle_id, player_id, action, amount) "
            "      SELECT id, owner_player_id, 'purchase', $4::bigint FROM v) "
            "SELECT id::text, plate FROM v",
            5, nullptr, params, nullptr, nullptr, 0);
        ExecStatusType st = PQresultStatus(res);
        if (st == PGRES_TUPLES_OK) {
            if (PQntuples(res) == 0) {
                PQclear(res);
                outError = "no such player";
                return false;
            }
            outId = PQgetvalue(res, 0, 0);
            outPlate = PQgetvalue(res, 0, 1);
            PQclear(res);
            return true;
        }
        const char* state = PQresultErrorField(res, PG_DIAG_SQLSTATE);
        bool plateTaken = state && std::string(state) == "23505";  // unique_violation: draw another plate
        outError = PQerrorMessage(conn_);
        PQclear(res);
        if (!plateTaken) return false;
    }
    return false;
}

bool Database::VehicleStatus(const std::string& id, const std::string& status, std::string& outError) {
    std::lock_guard<std::mutex> lock(mutex_);

    if (!EnsureConnected()) {
        outError = "not connected";
        return false;
    }
    const char* params[2] = {id.c_str(), status.c_str()};
    PGresult* res = PQexecParams(conn_,
        "UPDATE vehicles SET status = $2 WHERE id = $1::bigint", 2, nullptr, params, nullptr, nullptr, 0);
    bool ok = PQresultStatus(res) == PGRES_COMMAND_OK;
    if (!ok) outError = PQerrorMessage(conn_);
    else if (std::string(PQcmdTuples(res)) == "0") { ok = false; outError = "no such vehicle"; }
    PQclear(res);
    return ok;
}

// ---------------------------------------------------------------------------
// Staff actions from the game's Admin app.
// ---------------------------------------------------------------------------

bool Database::InsertStaffLog(const std::string& staffUid, const std::string& targetUid, const std::string& action,
                              const std::string& reason, const std::string& beforeJson,
                              const std::string& afterJson, std::string& outError) {
    // Caller holds mutex_.
    const char* params[6] = {staffUid.c_str(), targetUid.c_str(), action.c_str(), reason.c_str(),
                             beforeJson.c_str(), afterJson.c_str()};
    PGresult* res = PQexecParams(
        conn_,
        "INSERT INTO staff_log (staff_player_id, target_player_id, action, reason, before_value, after_value, source) "
        "VALUES ((SELECT id FROM players WHERE uid = $1), (SELECT id FROM players WHERE uid = NULLIF($2, '')), "
        "        $3, NULLIF($4, ''), NULLIF($5, '')::jsonb, NULLIF($6, '')::jsonb, 'game')",
        6, nullptr, params, nullptr, nullptr, 0);
    bool ok = PQresultStatus(res) == PGRES_COMMAND_OK;
    if (!ok) outError = PQerrorMessage(conn_);
    PQclear(res);
    return ok;
}

namespace {

// "Permanent ban" / "3-day ban" / "2-hour ban" / "30-minute ban", like the
// website's DurationLabel.
std::string BanLabel(int minutes) {
    if (minutes <= 0) return "Permanent ban";
    if (minutes % 1440 == 0) return std::to_string(minutes / 1440) + "-day ban";
    if (minutes % 60 == 0) return std::to_string(minutes / 60) + "-hour ban";
    return std::to_string(minutes) + "-minute ban";
}

// 1234567 -> "1,234,567", like the website's players.Dollars.
std::string Dollars(long long amount) {
    std::string digits = std::to_string(amount);
    std::string out;
    int count = 0;
    for (auto it = digits.rbegin(); it != digits.rend(); ++it) {
        if (count && count % 3 == 0) out.insert(out.begin(), ',');
        out.insert(out.begin(), *it);
        ++count;
    }
    return out;
}

}  // namespace

bool Database::AdminLog(const std::string& staffUid, const std::string& targetUid, const std::string& action,
                        const std::string& reason, const std::string& afterJson, std::string& outError) {
    std::lock_guard<std::mutex> lock(mutex_);
    if (!EnsureConnected()) {
        outError = "not connected";
        return false;
    }
    return InsertStaffLog(staffUid, targetUid, action, reason, "", afterJson, outError);
}

bool Database::AdminBan(const std::string& staffUid, const std::string& targetUid, int minutes,
                        const std::string& reason, std::string& outStatus, std::string& outError) {
    std::lock_guard<std::mutex> lock(mutex_);
    if (!EnsureConnected()) {
        outError = "not connected";
        return false;
    }
    auto rollback = [&](PGresult* res) {
        if (res) {
            outError = PQerrorMessage(conn_);
            PQclear(res);
        }
        PQclear(PQexec(conn_, "ROLLBACK"));
        return false;
    };
    PQclear(PQexec(conn_, "BEGIN"));

    const char* activeParams[1] = {targetUid.c_str()};
    PGresult* active = PQexecParams(conn_,
        "SELECT 1 FROM banlist WHERE uid = $1 AND (expires_at IS NULL OR expires_at > now()) LIMIT 1",
        1, nullptr, activeParams, nullptr, nullptr, 0);
    if (PQresultStatus(active) != PGRES_TUPLES_OK) return rollback(active);
    bool already = PQntuples(active) > 0;
    PQclear(active);
    if (already) {
        PQclear(PQexec(conn_, "ROLLBACK"));
        outStatus = "ALREADY";
        return true;
    }

    std::string mins = std::to_string(minutes);
    const char* banParams[4] = {staffUid.c_str(), targetUid.c_str(), reason.c_str(), mins.c_str()};
    PGresult* ban = PQexecParams(conn_,
        "INSERT INTO banlist (uid, reason, banned_by, expires_at, player_id, scope) "
        "VALUES ($2, $3, (SELECT id FROM players WHERE uid = $1), "
        "        CASE WHEN $4::int > 0 THEN now() + make_interval(mins => $4::int) END, "
        "        (SELECT id FROM players WHERE uid = $2), 'game') RETURNING id",
        4, nullptr, banParams, nullptr, nullptr, 0);
    if (PQresultStatus(ban) != PGRES_TUPLES_OK) return rollback(ban);
    std::string banId = PQgetvalue(ban, 0, 0);
    PQclear(ban);

    std::string after = "{\"ban\": " + banId + ", \"scope\": \"game\", \"minutes\": " + mins + "}";
    if (!InsertStaffLog(staffUid, targetUid, "ban.issue", BanLabel(minutes) + ": " + reason + " (in game)", "", after,
                        outError)) {
        return rollback(nullptr);
    }
    PGresult* commit = PQexec(conn_, "COMMIT");
    bool ok = PQresultStatus(commit) == PGRES_COMMAND_OK;
    if (!ok) outError = PQerrorMessage(conn_);
    PQclear(commit);
    outStatus = "OK";
    return ok;
}

bool Database::AdminKick(const std::string& staffUid, const std::string& targetUid, const std::string& reason,
                         std::string& outError) {
    std::lock_guard<std::mutex> lock(mutex_);
    if (!EnsureConnected()) {
        outError = "not connected";
        return false;
    }
    PQclear(PQexec(conn_, "BEGIN"));
    const char* params[3] = {staffUid.c_str(), targetUid.c_str(), reason.c_str()};
    PGresult* res = PQexecParams(conn_,
        "INSERT INTO kick_log (target_player_id, kicked_by, kick_type, reason) "
        "SELECT t.id, (SELECT id FROM players WHERE uid = $1), 'manual', $3 FROM players t WHERE t.uid = $2",
        3, nullptr, params, nullptr, nullptr, 0);
    bool ok = PQresultStatus(res) == PGRES_COMMAND_OK;
    if (!ok) outError = PQerrorMessage(conn_);
    PQclear(res);
    if (ok) ok = InsertStaffLog(staffUid, targetUid, "player.kick", reason, "", "", outError);
    PGresult* end = PQexec(conn_, ok ? "COMMIT" : "ROLLBACK");
    if (ok && PQresultStatus(end) != PGRES_COMMAND_OK) {
        ok = false;
        outError = PQerrorMessage(conn_);
    }
    PQclear(end);
    return ok;
}

bool Database::AdminCompensate(const std::string& staffUid, const std::string& targetUid, const std::string& faction,
                               long long amount, const std::string& reason, std::string& outStatus,
                               std::string& outError) {
    std::lock_guard<std::mutex> lock(mutex_);
    if (!EnsureConnected()) {
        outError = "not connected";
        return false;
    }
    const char* bankFaction = BankFaction(faction);
    if (!bankFaction) {
        outError = "invalid faction";
        return false;
    }
    auto rollback = [&](PGresult* res) {
        if (res) {
            outError = PQerrorMessage(conn_);
            PQclear(res);
        }
        PQclear(PQexec(conn_, "ROLLBACK"));
        return false;
    };
    PQclear(PQexec(conn_, "BEGIN"));

    const char* lookupParams[2] = {targetUid.c_str(), bankFaction};
    PGresult* acct = PQexecParams(conn_,
        "SELECT b.id, b.balance FROM bank_accounts b JOIN players p ON p.id = b.player_id "
        "WHERE p.uid = $1 AND b.faction = $2 FOR UPDATE OF b",
        2, nullptr, lookupParams, nullptr, nullptr, 0);
    if (PQresultStatus(acct) != PGRES_TUPLES_OK) return rollback(acct);
    if (PQntuples(acct) == 0) {
        PQclear(acct);
        PQclear(PQexec(conn_, "ROLLBACK"));
        outStatus = "NO_ACCOUNT";
        return true;
    }
    std::string accountId = PQgetvalue(acct, 0, 0);
    long long balance = std::stoll(PQgetvalue(acct, 0, 1));
    PQclear(acct);

    // Same row the website's players.Compensate writes; the
    // bank_transactions trigger updates the balance.
    std::string amountStr = std::to_string(amount);
    std::string after = std::to_string(balance + amount);
    std::string memo = "Compensation: " + reason;
    const char* txParams[5] = {accountId.c_str(), amountStr.c_str(), after.c_str(), memo.c_str(), staffUid.c_str()};
    PGresult* tx = PQexecParams(conn_,
        "INSERT INTO bank_transactions (account_id, type, amount, balance_after, memo, created_by) "
        "VALUES ($1::bigint, 'admin_adjustment', $2::bigint, $3::bigint, $4, (SELECT id FROM players WHERE uid = $5))",
        5, nullptr, txParams, nullptr, nullptr, 0);
    if (PQresultStatus(tx) != PGRES_COMMAND_OK) return rollback(tx);
    PQclear(tx);

    std::string account = std::string("\"") + bankFaction + "\"";
    if (!InsertStaffLog(staffUid, targetUid, "player.compensate",
                        "+$" + Dollars(amount) + " " + bankFaction + " bank: " + reason,
                        "{\"account\": " + account + ", \"balance\": " + std::to_string(balance) + "}",
                        "{\"account\": " + account + ", \"balance\": " + after + ", \"amount\": " + amountStr + "}",
                        outError)) {
        return rollback(nullptr);
    }
    PGresult* commit = PQexec(conn_, "COMMIT");
    bool ok = PQresultStatus(commit) == PGRES_COMMAND_OK;
    if (!ok) outError = PQerrorMessage(conn_);
    PQclear(commit);
    outStatus = "OK:" + after;
    return ok;
}

// ---------------------------------------------------------------------------
// Garage: owned vehicles in and out of the world.
// ---------------------------------------------------------------------------

namespace {

// One statement, no parameters' worth of result handling: runs it and
// returns the first column of the first row (or "" if there are no rows).
bool QueryOne(PGconn* conn, const char* sql, int n, const char* const* params, std::string& out,
              std::string& outError) {
    PGresult* res = PQexecParams(conn, sql, n, nullptr, params, nullptr, nullptr, 0);
    ExecStatusType st = PQresultStatus(res);
    if (st != PGRES_TUPLES_OK && st != PGRES_COMMAND_OK) {
        outError = PQerrorMessage(conn);
        PQclear(res);
        return false;
    }
    out = (st == PGRES_TUPLES_OK && PQntuples(res) > 0) ? PQgetvalue(res, 0, 0) : "";
    if (st == PGRES_COMMAND_OK) out = PQcmdTuples(res);
    PQclear(res);
    return true;
}

}  // namespace

bool Database::VehicleList(const std::string& uid, const std::string& faction, std::string& outResponse,
                           std::string& outError) {
    std::lock_guard<std::mutex> lock(mutex_);
    if (!EnsureConnected()) {
        outError = "not connected";
        return false;
    }
    const char* scope = faction == "cop" ? "police" : faction == "medic" ? "medic" : "civilian";
    const char* params[2] = {uid.c_str(), scope};
    return QueryOne(conn_,
        "SELECT COALESCE(json_agg(json_build_array(v.id::text, v.classname, COALESCE(v.plate, ''), v.status) ORDER BY v.id)::text, '[]') "
        "FROM vehicles v JOIN players p ON p.id = v.owner_player_id "
        "WHERE p.uid = $1 AND v.faction_scope = $2 AND v.status <> 'destroyed'",
        2, params, outResponse, outError);
}

bool Database::VehicleGet(const std::string& id, std::string& outResponse, std::string& outError) {
    std::lock_guard<std::mutex> lock(mutex_);
    if (!EnsureConnected()) {
        outError = "not connected";
        return false;
    }
    const char* params[1] = {id.c_str()};
    if (!QueryOne(conn_,
            "SELECT json_build_array(v.id::text, COALESCE(p.uid, ''), COALESCE(v.faction_scope, ''), v.classname, "
            "       COALESCE(v.plate, ''), v.status, v.fuel, v.damage, v.inventory, v.skin)::text "
            "FROM vehicles v LEFT JOIN players p ON p.id = v.owner_player_id WHERE v.id = $1::bigint",
            1, params, outResponse, outError)) {
        return false;
    }
    if (outResponse.empty()) {
        outError = "no such vehicle";
        return false;
    }
    return true;
}

bool Database::VehicleStore(const std::string& id, const std::string& status, const std::string& fuel,
                            const std::string& damageJson, const std::string& inventoryJson, std::string& outError) {
    std::lock_guard<std::mutex> lock(mutex_);
    if (!EnsureConnected()) {
        outError = "not connected";
        return false;
    }
    const char* params[5] = {id.c_str(), status.c_str(), fuel.c_str(), damageJson.c_str(), inventoryJson.c_str()};
    std::string updated;
    if (!QueryOne(conn_,
            "UPDATE vehicles SET status = $2, fuel = $3::real, damage = $4::jsonb, inventory = $5::jsonb, position = NULL "
            "WHERE id = $1::bigint",
            5, params, updated, outError)) {
        return false;
    }
    if (updated == "0") {
        outError = "no such vehicle";
        return false;
    }
    return true;
}

bool Database::VehicleInventory(const std::string& id, const std::string& inventoryJson, std::string& outError) {
    std::lock_guard<std::mutex> lock(mutex_);
    if (!EnsureConnected()) {
        outError = "not connected";
        return false;
    }
    const char* params[2] = {id.c_str(), inventoryJson.c_str()};
    std::string updated;
    if (!QueryOne(conn_, "UPDATE vehicles SET inventory = $2::jsonb WHERE id = $1::bigint", 2, params, updated, outError)) {
        return false;
    }
    if (updated == "0") {
        outError = "no such vehicle";
        return false;
    }
    return true;
}

bool Database::VehicleReset(std::string& outCount, std::string& outError) {
    std::lock_guard<std::mutex> lock(mutex_);
    if (!EnsureConnected()) {
        outError = "not connected";
        return false;
    }
    return QueryOne(conn_, "UPDATE vehicles SET status = 'garaged', position = NULL WHERE status = 'spawned'", 0, nullptr,
                    outCount, outError);
}

bool Database::VehicleLog(const std::string& id, const std::string& uid, const std::string& action,
                          const std::string& amount, std::string& outError) {
    std::lock_guard<std::mutex> lock(mutex_);
    if (!EnsureConnected()) {
        outError = "not connected";
        return false;
    }
    const char* params[4] = {id.c_str(), uid.c_str(), action.c_str(), amount.c_str()};
    std::string inserted;
    return QueryOne(conn_,
        "INSERT INTO vehicle_logs (vehicle_id, player_id, action, amount) "
        "VALUES ($1::bigint, (SELECT id FROM players WHERE uid = NULLIF($2, '')), $3, NULLIF($4, '')::bigint)",
        4, params, inserted, outError);
}
