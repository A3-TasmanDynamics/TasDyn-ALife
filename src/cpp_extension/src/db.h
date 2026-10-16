#pragma once

#include <libpq-fe.h>

#include <mutex>
#include <string>

#include "config.h"

// Thin wrapper around a single libpq connection. Every query goes through
// PQexecParams against a fixed, compile-time SQL string -- parameterized,
// never built from callExtension input, satisfying "prepared statements
// only" without the extra bookkeeping of ~20 named PQprepare statements
// for Phase 1's save field allowlist. A real connection pool is future
// work; Phase 1's job was proving a single connection works end to end.
class Database {
public:
    ~Database();

    bool Connect(const DbConfig& config, std::string& outError);
    void Disconnect();
    bool IsConnected() const;

    // Round-trips SELECT 1 -- the smallest possible proof the whole chain
    // (extension -> libpq -> Postgres) works.
    bool Ping(std::string& outError);

    // docs/DATA_CONTRACT.md "load": creates a blank record (+ one
    // bank_accounts row per faction) if uid has none yet, then returns the
    // full record as a parseSimpleArray-compatible [[key,value],...] string.
    bool LoadPlayer(const std::string& uid, std::string& outResponse, std::string& outError);

    // docs/DATA_CONTRACT.md "save": persists one allowlisted field.
    // outStatus is "OK", "ERROR", or "DUPLICATE" (cash fields only) --
    // never a raw error string, that's what outError is for (logging, not
    // the SQF-visible response).
    bool SaveField(const std::string& uid, const std::string& field, const std::string& value,
                   const std::string& token, std::string& outStatus, std::string& outError);

    // A player disconnected while incapacitated (combat logging). Writes, in
    // one statement, an anti_cheat_flags row ('combat_log', for review on the
    // website) and a staff_log row (website Staff Log, posted to Discord
    // #staff-log by the website's audit poster). killer/weapon/distance are
    // "" when nobody else was involved.
    bool RecordCombatLog(const std::string& uid, const std::string& faction, const std::string& killer,
                         const std::string& weapon, const std::string& distance,
                         const std::string& secondsIncapacitated, std::string& outError);

private:
    PGconn* conn_ = nullptr;
    mutable std::mutex mutex_;

    // Caller must hold mutex_. Recovers from a dropped connection via
    // PQreset (closes and reopens using the same parameters Connect()
    // originally supplied -- libpq tracks those internally, nothing extra
    // to store here). Every public method calls this first: without it, one
    // network blip between the extension and Postgres would silently fail
    // every load/save for the rest of the server's uptime, since nothing
    // else here ever re-connects.
    bool EnsureConnected();

    // Caller must hold mutex_.
    bool EnsureBlankPlayer(const std::string& uid, std::string& outError);
    bool SaveCashDelta(const std::string& uid, const std::string& faction, const std::string& value,
                        const std::string& token, std::string& outStatus, std::string& outError);
};
