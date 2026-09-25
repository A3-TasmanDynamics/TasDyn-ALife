# src/cpp_extension/

The C++ bridge between Arma 3 (`callExtension`) and PostgreSQL, built on `libpqxx`.

Rules for this extension:
- All queries use `exec_params` (prepared statements). Never build SQL by
  concatenating strings from `callExtension` input.
- Any array/string returned to SQF has a fixed, documented field order.
  Document that order here (or in `docs/`) before writing the SQF side that
  parses it — the previous prototype lost rank data to exactly this kind of
  undocumented positional mismatch.
