// Entry point for tasdyn_alife_x64.dll -- the Arma 3 extension bridge to
// Postgres. Exports the four functions Arma's callExtension looks for
// (RVExtension/RVExtensionArgs/RVExtensionVersion); signatures verified
// against https://community.bistudio.com/wiki/Arma:_Extensions and a real
// working extension (github.com/Ni1kko/Arma-Extension-Callback), since the
// local SQF command DB (docs/arma/) doesn't cover the native extension
// interface itself (a different wiki namespace from scripting commands).

#include <windows.h>

#include <cstring>
#include <string>
#include <vector>

#include "commands.h"
#include "config.h"
#include "db.h"

namespace {

Database g_db;
bool g_connected = false;
std::string g_initError;
std::string g_commandPassword;  // config.ini [server] command_password

// Connects on first use rather than at DLL load (DllMain) -- doing blocking
// I/O from DllMain is undefined-behavior-adjacent on Windows (loader lock),
// and Arma calling RVExtensionVersion immediately after load shouldn't have
// to wait on a DB round-trip that command doesn't need.
void EnsureConnected() {
    if (g_connected) return;

    DbConfig config;
    std::string error;
    if (!LoadDbConfig(config, error)) {
        g_initError = error;
        return;
    }

    g_commandPassword = config.commandPassword;
    if (!g_db.Connect(config, error)) {
        g_initError = error;
        return;
    }

    g_connected = true;
}

void WriteOutput(char* output, unsigned int outputSize, const std::string& value) {
    if (outputSize == 0) return;
    size_t len = value.size();
    if (len >= outputSize) len = outputSize - 1;  // always leave room for the null terminator
    memcpy(output, value.data(), len);
    output[len] = '\0';
}

std::vector<std::string> ToVector(const char** argv, int argc) {
    std::vector<std::string> result;
    result.reserve(static_cast<size_t>(argc > 0 ? argc : 0));
    for (int i = 0; i < argc; ++i) {
        // Every arg arrives in Arma's `str` form -- see UnwrapArmaString.
        result.emplace_back(UnwrapArmaString(argv[i] ? argv[i] : ""));
    }
    return result;
}

std::string RunCommand(const std::string& command, const std::vector<std::string>& args) {
    EnsureConnected();
    // ["server", ["password"]]: the server's command password, for the
    // mission's serverCommand "#kick" (the extension only runs on the
    // server, so only server-side SQF can ask). Never logged.
    if (command == "server") {
        if (args.empty() || args[0] != "password") return "ERROR:unknown server op";
        if (g_commandPassword.empty()) return "ERROR:no command password in config.ini";
        return g_commandPassword;
    }
    if (!g_connected) {
        return "ERROR:" + g_initError;
    }
    return DispatchCommand(g_db, command, args);
}

}  // namespace

extern "C" {

__declspec(dllexport) void __stdcall RVExtensionVersion(char* output, int outputSize) {
    WriteOutput(output, static_cast<unsigned int>(outputSize), "tasdyn_alife 0.1.0");
}

__declspec(dllexport) void __stdcall RVExtension(char* output, int outputSize, const char* function) {
    std::string command(function ? function : "");
    std::string result = RunCommand(command, {});
    WriteOutput(output, static_cast<unsigned int>(outputSize), result);
}

__declspec(dllexport) int __stdcall RVExtensionArgs(char* output, int outputSize, const char* function,
                                                      const char** argv, int argc) {
    std::string command(function ? function : "");
    std::vector<std::string> args = ToVector(argv, argc);
    std::string result = RunCommand(command, args);
    WriteOutput(output, static_cast<unsigned int>(outputSize), result);
    return 0;
}

}  // extern "C"

BOOL APIENTRY DllMain(HMODULE /*hModule*/, DWORD reason, LPVOID /*reserved*/) {
    if (reason == DLL_PROCESS_DETACH) {
        g_db.Disconnect();
    }
    return TRUE;
}
