# Docker container cleanup

User-scoped maintenance for the local Colima Docker socket. The launchd job
`com.sjaconette.docker-cleanup` runs at login and daily at 13:40 local time.
A missed calendar run during sleep runs on wake. If Colima is unavailable,
the job logs a skip; it does not start or restart Docker or the VM.

The job removes containers stopped for at least seven full days, using
`State.FinishedAt`. Docker's `container prune --until` uses creation time,
which could delete a long-lived container immediately after it stops.
Never-started containers have no valid stop time and are retained.

Running, paused, and restarting containers are retained. So are Compose-managed
containers, containers with a restart policy other than `no`, and containers
with either a `keep` or `local.cleanup.keep` label (any value). Images, networks,
build cache, bind mounts, and volumes are outside this job's removal scope.
Removed containers' writable layers, metadata, and logs are deleted.

For new containers to keep, add `--label local.cleanup.keep=true`.
For existing containers, add their exact name (without `/`) or full ID to
`~/.config/docker-cleanup/keep.txt`, one per line. Blank lines and lines beginning
with `#` are ignored. Restartable services and Compose services need no extra pin.

```sh
docker-cleanup          # preview
docker-cleanup --apply  # apply the same seven-day policy now
launchctl print gui/501/com.sjaconette.docker-cleanup
```

The tool rechecks each full container ID and stop timestamp before `docker rm`.
It never passes `--force` or `--volumes`: Docker refuses to remove a container
that is running at removal time, and volumes remain intact. Docker does not offer
conditional removal based on a stop timestamp, so a rapid start-and-exit between
the last inspection and removal remains a small race. Use the keep list or label
for containers that must be retained.

Each Docker invocation has a 30-second timeout and the sweep has a five-minute
budget. A file lock prevents overlapping manual and scheduled runs. Inspection
failures preserve the affected container and produce an error. Only cleanup
metadata is inspected; container environment variables and logs are not read.
The explicit socket and executable prevent a selected remote Docker context
from changing the target.

Logs are in `~/.local/state/docker-cleanup/service.log`, rotated at 1 MiB with
one previous copy. Times in logs are UTC. BuildKit already has its native cache
GC enabled; its existing defaults remain active and need no extra pruning job.

To disable scheduling:

```sh
launchctl bootout gui/501 ~/Library/LaunchAgents/com.sjaconette.docker-cleanup.plist
```

To rebuild after source changes (Go standard library only), then enable:

```sh
~/.local/share/docker-cleanup/install.sh
launchctl bootstrap gui/501 ~/Library/LaunchAgents/com.sjaconette.docker-cleanup.plist
```

References:

- [Docker container pruning and its creation-time filter](https://docs.docker.com/reference/cli/docker/container/prune/)
- [Docker removal semantics](https://docs.docker.com/reference/cli/docker/container/rm/)
- [Native BuildKit garbage collection](https://docs.docker.com/build/cache/garbage-collection/)
- [Apple launchd calendar scheduling](https://developer.apple.com/library/archive/documentation/MacOSX/Conceptual/BPSystemStartup/Chapters/ScheduledJobs.html)
