#include "db.h"

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

bool Database::Ping(std::string& outError) {
    std::lock_guard<std::mutex> lock(mutex_);

    if (!conn_ || PQstatus(conn_) != CONNECTION_OK) {
        outError = "not connected";
        return false;
    }

    // PQexecParams with zero params, not PQexec -- keeps every query path
    // through this extension on the parameterized/prepared-statement API,
    // even this trivial one, so there's no path that ever hand-builds SQL.
    PGresult* res = PQexecParams(conn_, "SELECT 1", 0, nullptr, nullptr, nullptr, nullptr, 0);

    bool ok = (PQresultStatus(res) == PGRES_TUPLES_OK);
    if (!ok) {
        outError = PQerrorMessage(conn_);
    }

    PQclear(res);
    return ok;
}
