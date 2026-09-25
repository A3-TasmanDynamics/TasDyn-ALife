# database/

PostgreSQL schema for the ALife backend. This directory holds exactly one
canonical schema — do not let a second, drifting copy accumulate here.

Column names defined here are the contract the C++ extension (`src/cpp_extension`)
and the SQF save/load functions (`src/mission`) both code against. If a column
is renamed or reordered, update all three in the same PR.
