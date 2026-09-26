#include "db.h"

#include <array>
#include <cstring>
#include <sstream>

namespace {

// Escapes a plain-text value for embedding inside a double-quoted SQF/JSON
// string literal (only used for genuinely free-text fields -- name, dept,
// staff_rank_key -- never for the JSONB array-of-pairs fields, which are
// embedded verbatim since they're already valid array literals).
std::string EscapeStringLiteral(const std::string& raw) {
    std::string out;
    out.reserve(raw.size() + 8);
    for (char c : raw) {
        if (c == '"' || c == '\\') out.push_back('\\');
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

constexpr std::array<SaveFieldSpec, 17> kSaveFields{{
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
        " connect_timeout=5";

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
        "       p.civ_bounty "
        "FROM players p LEFT JOIN staff_ranks sr ON sr.id = p.staff_rank_id "
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
    out += "]";

    PQclear(res);

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
