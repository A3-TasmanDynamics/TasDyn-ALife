# src/mission/

SQF mission source for the ALife gamemode. Client requests, server decides:
gameplay code sends an intent to the server; the server is the only thing
that reads or writes persistent state via `src/cpp_extension`.
