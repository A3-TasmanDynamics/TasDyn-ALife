#pragma once
#include <string>

// Connection settings loaded from config.ini, which must sit next to the
// compiled extension DLL. See config.ini.example at the repo root.
struct DbConfig {
    std::string host = "127.0.0.1";
    std::string port = "5432";
    std::string dbname;
    std::string user;
    std::string password;
};

// Loads config.ini from the directory this DLL is running from (not the
// process's working directory -- Arma's working directory is the server
// install root, not the extension's folder). Returns false and fills
// outError if the file is missing or missing a required key.
bool LoadDbConfig(DbConfig& outConfig, std::string& outError);
