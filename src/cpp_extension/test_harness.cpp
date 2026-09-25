// Standalone native test harness: loads the built extension DLL and calls
// its exports exactly the way Arma's callExtension does (LoadLibrary +
// GetProcAddress), so the extension can be smoke-tested without an Arma 3
// install. Not part of the shipped extension -- dev tool only.
//
// Usage (from the build/ directory, after build.ps1):
//   test_harness.exe ping

#include <windows.h>

#include <cstdio>
#include <string>
#include <vector>

typedef void(__stdcall* RVExtensionVersionFn)(char*, int);
typedef void(__stdcall* RVExtensionFn)(char*, int, const char*);
typedef int(__stdcall* RVExtensionArgsFn)(char*, int, const char*, const char**, int);

int main(int argc, char** argv) {
    std::string command = (argc > 1) ? argv[1] : "ping";
    std::vector<std::string> args;
    for (int i = 2; i < argc; ++i) args.emplace_back(argv[i]);

    HMODULE hModule = LoadLibraryA("tasdyn_alife_x64.dll");
    if (!hModule) {
        printf("LoadLibrary failed, error code %lu\n", GetLastError());
        return 1;
    }

    auto version = reinterpret_cast<RVExtensionVersionFn>(GetProcAddress(hModule, "RVExtensionVersion"));
    auto extArgs = reinterpret_cast<RVExtensionArgsFn>(GetProcAddress(hModule, "RVExtensionArgs"));

    if (!version || !extArgs) {
        printf("GetProcAddress failed to find one or more exports.\n");
        FreeLibrary(hModule);
        return 1;
    }

    char versionBuf[256] = {};
    version(versionBuf, sizeof(versionBuf));
    printf("RVExtensionVersion: %s\n", versionBuf);

    std::vector<const char*> argv_c;
    for (auto& a : args) argv_c.push_back(a.c_str());

    char outputBuf[10240] = {};
    extArgs(outputBuf, sizeof(outputBuf), command.c_str(), argv_c.data(), static_cast<int>(argv_c.size()));
    printf("RVExtensionArgs(\"%s\") => %s\n", command.c_str(), outputBuf);

    FreeLibrary(hModule);
    return 0;
}
