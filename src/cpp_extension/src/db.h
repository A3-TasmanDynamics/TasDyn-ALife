#pragma once

#include <libpq-fe.h>

#include <mutex>
#include <string>

#include "config.h"

// Thin wrapper around a single libpq connection. Phase 1's job is proving
// the connection actually works end to end (see Ping) -- prepared statements
// and a real connection pool land with cmd_save/cmd_load.
class Database {
public:
    ~Database();

    bool Connect(const DbConfig& config, std::string& outError);
    void Disconnect();
    bool IsConnected() const;

    // Round-trips SELECT 1 through the real connection -- the smallest
    // possible proof the whole chain (extension -> libpq -> Postgres) works.
    bool Ping(std::string& outError);

private:
    PGconn* conn_ = nullptr;
    mutable std::mutex mutex_;
};
