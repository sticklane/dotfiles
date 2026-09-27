# Rust/cargo removed — reinstall with: curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh
. "$HOME/.cargo/env"

# Go memory guard. Three watchdog panics in Sep 2026 came from swap exhaustion on
# this 16 GB machine; the 2026-09-25 one was two Go `compile` processes at 7.3 and
# 5.2 GiB during `go test ./...`. GOMEMLIMIT is a soft heap limit that every Go
# program honours (compiler, linker, test binaries). An explicit GOGC keeps the
# compiler from turning its own GC off for a large starting heap.
export GOMEMLIMIT="${GOMEMLIMIT:-2GiB}"
export GOGC="${GOGC:-100}"
