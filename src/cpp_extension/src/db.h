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
    // Bank ledger changes (docs/DATA_CONTRACT.md "bank"): op "purchase"
    // debits uid's faction account; op "transfer" moves money from uid's
    // faction account to otherUid's otherFaction account. Locks the
    // account(s), refuses to go below zero, and writes bank_transactions
    // rows (DB triggers update the balance and players.*_bank). The
    // request token makes a retried request a no-op. outStatus: "OK",
    // "INSUFFICIENT", "DUPLICATE", "NO_ACCOUNT" or "ERROR".
    bool BankTx(const std::string& op, const std::string& uid, const std::string& faction,
                const std::string& amount, const std::string& token, const std::string& otherUid,
                const std::string& otherFaction, const std::string& memo, std::string& outStatus,
                std::string& outError);

    // ATM: op "deposit" moves amount from players.<faction>_cash into the
    // faction bank account; op "withdraw" the other way. One transaction,
    // both rows locked, never below zero, a bank_transactions row each time
    // (idempotent by request token). outStatus as BankTx.
    bool BankCash(const std::string& op, const std::string& uid, const std::string& faction,
                  const std::string& amount, const std::string& token, std::string& outStatus,
                  std::string& outError);

    // Who's online (player_sessions, read by the website): op "start" opens
    // a session for uid (closing any left open), "end" closes it, "reset"
    // closes every open session (server start).
    bool RecordSession(const std::string& op, const std::string& uid, std::string& outError);

    // Dynamic market (docs/DATA_CONTRACT.md "market"): MarketLoad returns
    // every saved price as [["item","price"],...]; MarketSet upserts one;
    // MarketReset clears them all (back to the CfgMarket base prices).
    bool MarketLoad(std::string& outResponse, std::string& outError);
    bool MarketSet(const std::string& item, const std::string& price, std::string& outError);
    bool MarketReset(std::string& outError);

    // Vehicles (docs/DATA_CONTRACT.md "vehicle"): VehicleAdd records a bought
    // vehicle and its purchase log, returning its id and new plate;
    // VehicleStatus sets spawned / garaged / impounded / destroyed.
    bool VehicleAdd(const std::string& uid, const std::string& faction, const std::string& classname,
                    const std::string& price, const std::string& skin, std::string& outId, std::string& outPlate,
                    std::string& outError);
    // The garage (ALife_fnc_garage*): a player's vehicles for one faction
    // (not destroyed) as [[id, class, plate, status], ...]; one vehicle as
    // [id, ownerUid, scope, class, plate, status, fuel, damage, inventory,
    // skin]; storing one (status garaged / impounded, with its fuel,
    // hit-point damage and trunk); saving just its trunk; sending every
    // vehicle still "spawned" back to the garage (server start); and a
    // vehicle_logs row (impound / return).
    bool VehicleList(const std::string& uid, const std::string& faction, std::string& outResponse, std::string& outError);
    bool VehicleGet(const std::string& id, std::string& outResponse, std::string& outError);
    bool VehicleStore(const std::string& id, const std::string& status, const std::string& fuel,
                      const std::string& damageJson, const std::string& inventoryJson, std::string& outError);
    bool VehicleInventory(const std::string& id, const std::string& inventoryJson, std::string& outError);
    bool VehicleReset(std::string& outCount, std::string& outError);
    bool VehicleLog(const std::string& id, const std::string& uid, const std::string& action,
                    const std::string& amount, std::string& outError);
    bool VehicleStatus(const std::string& id, const std::string& status, std::string& outError);

    bool RecordCombatLog(const std::string& uid, const std::string& faction, const std::string& killer,
                         const std::string& weapon, const std::string& distance,
                         const std::string& secondsIncapacitated, std::string& outError);

    // Staff actions from the game's Admin app (ALife_fnc_adminAction). All
    // land in staff_log with source 'game' (posted to #staff-log like the
    // website's), alongside banlist / kick_log / bank_transactions -- the
    // same rows the website writes for the same actions.
    //   AdminLog:        just the staff_log row (afterJson may be "")
    //   AdminBan:        minutes 0 = permanent. outStatus "OK" / "ALREADY"
    //   AdminKick:       kick_log 'manual' + staff_log
    //   AdminCompensate: an 'admin_adjustment' credit to the target's
    //                    faction bank account. outStatus "OK:<new balance>"
    //                    / "NO_ACCOUNT"
    bool AdminLog(const std::string& staffUid, const std::string& targetUid, const std::string& action,
                  const std::string& reason, const std::string& afterJson, std::string& outError);
    bool AdminBan(const std::string& staffUid, const std::string& targetUid, int minutes,
                  const std::string& reason, std::string& outStatus, std::string& outError);
    bool AdminKick(const std::string& staffUid, const std::string& targetUid, const std::string& reason,
                   std::string& outError);
    bool AdminCompensate(const std::string& staffUid, const std::string& targetUid, const std::string& faction,
                         long long amount, const std::string& reason, std::string& outStatus,
                         std::string& outError);

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
    bool InsertStaffLog(const std::string& staffUid, const std::string& targetUid, const std::string& action,
                        const std::string& reason, const std::string& beforeJson, const std::string& afterJson,
                        std::string& outError);
};
