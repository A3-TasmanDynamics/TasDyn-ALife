#include "config.h"

#include <windows.h>

#include <fstream>
#include <string>

namespace {

std::string GetModuleDirectory() {
    HMODULE hModule = nullptr;
    // GET_MODULE_HANDLE_EX_FLAG_FROM_ADDRESS resolves the module that
    // *this function* lives in -- i.e. this DLL, not the host .exe (Arma).
    GetModuleHandleExA(
        GET_MODULE_HANDLE_EX_FLAG_FROM_ADDRESS | GET_MODULE_HANDLE_EX_FLAG_UNCHANGED_REFCOUNT,
        reinterpret_cast<LPCSTR>(&GetModuleDirectory),
        &hModule);

    char path[MAX_PATH] = {};
    GetModuleFileNameA(hModule, path, MAX_PATH);

    std::string full(path);
    size_t pos = full.find_last_of("\\/");
    return (pos == std::string::npos) ? "" : full.substr(0, pos + 1);
}

std::string Trim(const std::string& s) {
    size_t start = s.find_first_not_of(" \t\r\n");
    if (start == std::string::npos) return "";
    size_t end = s.find_last_not_of(" \t\r\n");
    return s.substr(start, end - start + 1);
}

}  // namespace

bool LoadDbConfig(DbConfig& outConfig, std::string& outError) {
    std::string iniPath = GetModuleDirectory() + "config.ini";
    std::ifstream file(iniPath);
    if (!file.is_open()) {
        outError = "config.ini not found next to the extension DLL (" + iniPath + ")";
        return false;
    }

    std::string line;
    std::string section;
    while (std::getline(file, line)) {
        std::string trimmed = Trim(line);
        if (trimmed.empty() || trimmed[0] == ';' || trimmed[0] == '#') continue;

        if (trimmed.front() == '[' && trimmed.back() == ']') {
            section = trimmed.substr(1, trimmed.size() - 2);
            continue;
        }

        size_t eq = trimmed.find('=');
        if (eq == std::string::npos) continue;
        if (section != "database") continue;

        std::string key = Trim(trimmed.substr(0, eq));
        std::string value = Trim(trimmed.substr(eq + 1));

        if (key == "host") outConfig.host = value;
        else if (key == "port") outConfig.port = value;
        else if (key == "dbname") outConfig.dbname = value;
        else if (key == "user") outConfig.user = value;
        else if (key == "password") outConfig.password = value;
    }

    if (outConfig.dbname.empty() || outConfig.user.empty()) {
        outError = "config.ini is missing a required [database] dbname or user value";
        return false;
    }

    return true;
}
