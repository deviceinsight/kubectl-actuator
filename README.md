# kubectl-actuator

A kubectl plugin for interacting with Spring Boot Actuator endpoints.

## Installation

Make sure you have [krew](https://krew.sigs.k8s.io/) installed.

```bash
# Install the plugin
kubectl krew install actuator

# Enable shell completion (optional)
ln -s ~/.krew/bin/kubectl-actuator ~/.krew/bin/kubectl_complete-actuator
```

## Quick start

```bash
# Check health of every pod in a deployment
kubectl actuator -d my-app health

# List loggers with configured levels, then turn one up
kubectl actuator -d my-app logger
kubectl actuator -d my-app logger com.example.service DEBUG

# Query any other actuator endpoint as JSON
kubectl actuator -d my-app raw mappings
```

## Configuration

The plugin talks to each pod through the Kubernetes API server's port-forwarding mechanism.
The actuator endpoints only need to be
[exposed over HTTP](https://docs.spring.io/spring-boot/reference/actuator/endpoints.html#actuator.endpoints.exposing)
inside the pod (`management.endpoints.web.exposure.include`). No manual `kubectl port-forward` required.

By default, the plugin expects Spring Boot Actuator on `http://localhost:8080/actuator`. You can customize this in two
ways:

### Command-line Flags

- `--port <port>`: Actuator port (default: `8080`)
- `--base-path <path>`: Actuator base path (default: `actuator`; use `/` if the endpoints are served at the root path)

### Pod Annotations

- `kubectl-actuator.device-insight.com/port`: Actuator port
- `kubectl-actuator.device-insight.com/basePath`: Actuator base path

**Note:** Command-line flags take precedence over pod annotations, which take precedence over defaults.

## Usage

### Target Selection

All commands support target selection:

- `--pod <pod-name>` or `-p`: Target one or more specific pods (repeat the flag or comma-separate: `-p pod-1,pod-2`)
- `--deployment <deployment-name>` or `-d`: Target all pods in one or more deployments
- `--selector <label-selector>` or `-l`: Target pods by label selector (e.g., `app=myapp,env=prod`)

### Discovering Endpoints

```bash
❯ kubectl actuator -d my-app endpoints
ENDPOINT        AVAILABLE  KUBECTL ACTUATOR SUPPORT
beans           true       beans
conditions      true       -
configprops     true       -
env             true       env
health          true       health
heapdump        true       heapdump
info            true       info
logfile         false      logfile
loggers         true       logger
metrics         true       metrics
scheduledtasks  true       scheduledtasks
threaddump      true       threaddump
```

### Loggers

#### List loggers

```bash
❯ kubectl actuator --pod my-app-pod logger
LOGGER                                               LEVEL
ROOT                                                 INFO
com.example.app                                      INFO
com.example.app.service                              DEBUG
org.apache.catalina.startup.DigesterFactory          ERROR
org.apache.catalina.util.LifecycleBase               ERROR
org.springframework.web                              INFO

GROUP  CONFIGURED  MEMBERS
sql    -           org.springframework.jdbc.core, org.hibernate.SQL, …
web    -           org.springframework.core.codec, org.springframework.http, …
```

Only loggers with a configured level are shown by default. Use `--all` to list every logger.

#### Set logger level

```bash
# Set a specific logger to DEBUG
❯ kubectl actuator --pod my-app-pod logger com.example.app.service DEBUG

# Set ROOT logger level
❯ kubectl actuator --pod my-app-pod logger ROOT WARN

# Set a logger group
❯ kubectl actuator --pod my-app-pod logger web DEBUG
```

**Note:** Use `RESET` to clear a configured level and inherit from the parent logger.

### Scheduled Tasks

```bash
❯ kubectl actuator --deployment my-app scheduledtasks
TYPE         TARGET                                  SCHEDULE                           NEXT            LAST         STATUS
cron         BackupScheduler.scheduleBackups         cron(0 * * * * *)                  in 49s          11s ago      SUCCESS
fixedDelay   CacheRefreshService.refreshCache        fixedDelay=5m                      in 4m33s        27s ago      SUCCESS
fixedDelay   CleanupScheduler.triggerCleanup         fixedDelay=24h                     in 23h44m33s    15m27s ago   SUCCESS
fixedDelay   HealthCheckService.checkServiceHealth   fixedDelay=12h initialDelay=15m    in 11h59m58s    27s ago      ERROR - Connection timeout
fixedDelay   StatusWatcher.checkStatus               fixedDelay=5s                      -               2s ago       STARTED
fixedRate    UpdateService.checkForUpdates           fixedRate=30m                      in 14m33s       15m27s ago   SUCCESS
```

**Note:** Execution tracking (NEXT, LAST, STATUS columns) requires Spring Boot 3.4.0 or later.

### Application Info

```bash
❯ kubectl actuator --pod my-app-pod info
Application:
  Name:         my-app
  Description:  My Spring Boot application

Build:
  Group:        com.example
  Artifact:     my-app
  Version:      1.0.0
  Time:         2025-10-21T22:34:55.709Z

Java:
  Vendor:
    Name:  Eclipse Adoptium
  Version:  21.0.1

Kubernetes:
  Host IP:          10.0.0.10
  Namespace:        default
  Node Name:        node-1
  Pod IP:           10.0.0.23
  Pod Name:         my-app-5d4c8f9b-xk7pq
  Service Account:  my-app
```

### Health

```bash
❯ kubectl actuator --pod my-app-pod health
COMPONENT       STATUS
diskSpace       UP
livenessState   UP
ping            UP
readinessState  UP
ssl             UP
[overall]       UP

Groups: liveness, readiness

# Query a health group or a single component
❯ kubectl actuator --pod my-app-pod health readiness
COMPONENT        STATUS
readinessState   UP
[overall]        UP
```

Exit codes: `0` if every targeted pod is UP, `1` if at least one pod reports a status other than UP, `2` if the check
itself failed.

For detailed health information including component details:

```bash
❯ kubectl actuator --pod my-app-pod health -o wide
COMPONENT       STATUS  DETAILS
diskSpace       UP      {"exists":true,"free":7046635520,"path":"/app/.","threshold":10485760,"total":254431723520}
livenessState   UP      -
ping            UP      -
readinessState  UP      -
ssl             UP      {"validChains":[],"invalidChains":[]}
[overall]       UP      -
```

### Metrics

```bash
# List all available metrics
❯ kubectl actuator --pod my-app-pod metrics
jvm.memory.used
jvm.memory.max
jvm.threads.live
http.server.requests
system.cpu.usage
...

# Filter metrics by name
❯ kubectl actuator --pod my-app-pod metrics --filter jvm.memory
jvm.memory.used
jvm.memory.max
jvm.memory.committed

# Get detailed information for a specific metric
❯ kubectl actuator --pod my-app-pod metrics jvm.memory.used
NAME         jvm.memory.used
DESCRIPTION  The amount of used memory
BASE UNIT    bytes

MEASUREMENTS
STATISTIC  VALUE
VALUE      102.5 MB

AVAILABLE TAGS
TAG   VALUES
area  heap, nonheap
id    CodeHeap 'profiled nmethods', G1 Old Gen, ...

# Drill down by tag
❯ kubectl actuator --pod my-app-pod metrics jvm.memory.used --tag area=heap
NAME         jvm.memory.used
DESCRIPTION  The amount of used memory
BASE UNIT    bytes
TAGS         area:heap

MEASUREMENTS
STATISTIC  VALUE
VALUE      64.2 MB

AVAILABLE TAGS
TAG  VALUES
id   G1 Eden Space, G1 Old Gen, G1 Survivor Space
```

### Environment

```bash
# View all environment properties
❯ kubectl actuator --pod my-app-pod env

# Filter properties by name
❯ kubectl actuator --pod my-app-pod env --filter server.port
Active Profiles: -

NAME               VALUE  ORIGIN
local.server.port  8080   server.ports

# Filter and show only names
❯ kubectl actuator --pod my-app-pod env --filter spring -o name
spring.application.name
spring.application.pid
spring.application.version
```

### Thread Dump

```bash
# Get full thread dump
❯ kubectl actuator --pod my-app-pod threaddump
Total Threads: 45

Thread States:
  RUNNABLE: 12
  WAITING: 5
  TIMED_WAITING: 28

Thread #1: main (ID: 1)
  State: RUNNABLE
  Daemon: false, In Native: false, Suspended: false
  Stack Trace:
    at java.net.SocketInputStream.socketRead0(Native Method)
    at java.net.SocketInputStream.socketRead(SocketInputStream.java:116)
    ...

# Filter by thread state
❯ kubectl actuator --pod my-app-pod threaddump --state BLOCKED

# Filter by thread name
❯ kubectl actuator --pod my-app-pod threaddump --filter "http-nio"

# Show summary only
❯ kubectl actuator --pod my-app-pod threaddump --summary

# Show thread list without stack traces
❯ kubectl actuator --pod my-app-pod threaddump --no-stacktrace
```

### Beans

```bash
# List all beans in table format
❯ kubectl actuator --pod my-app-pod beans
NAME                     TYPE                                SCOPE      DEPENDENCIES
applicationTaskExecutor  o.s.s.c.ThreadPoolTaskExecutor      singleton  2
basicErrorController     o.s.b.a.w.s.e.BasicErrorController  singleton  2
beansEndpoint            o.s.b.a.b.BeansEndpoint             singleton  2
cachesEndpoint           o.s.b.a.c.CachesEndpoint            singleton  1
...

# Filter beans by name
❯ kubectl actuator --pod my-app-pod beans --filter controller
NAME                  TYPE                                SCOPE      DEPENDENCIES
basicErrorController  o.s.b.a.w.s.e.BasicErrorController  singleton  2
userController        c.e.a.UserController                singleton  3

# Show detailed information with -o wide
❯ kubectl actuator --pod my-app-pod beans --filter userController -o wide
Context: my-app
Beans: 1

Bean: userController
  Type: com.example.app.UserController
  Scope: singleton
  Dependencies (3):
    - userService
    - validationService
    - objectMapper
```

### Heap Dump

```bash
# Download a heap dump (written to heapdump-<pod>-<timestamp>.hprof)
❯ kubectl actuator --pod my-app-pod heapdump
Requesting heap dump from pod "my-app-pod"...
Wrote 245.3 MB to heapdump-my-app-pod-20260723-154210.hprof

# Only live objects (forces a full GC), custom file name
❯ kubectl actuator --pod my-app-pod heapdump --live --output-file dump.hprof

# Stream to stdout for piping
❯ kubectl actuator --pod my-app-pod heapdump --output-file - | gzip > dump.hprof.gz
```

**Note:** The `heapdump` endpoint must be exposed. Since Spring Boot 3.5 access to it is additionally restricted by
default; re-enable it with `management.endpoint.heapdump.access: unrestricted`.

### Log File

Requires `logging.file.name` or `logging.file.path` to be configured in the application.

```bash
# Print the application log file to stdout
❯ kubectl actuator --pod my-app-pod logfile

# Only the last 64 KiB
❯ kubectl actuator --pod my-app-pod logfile --tail-bytes 65536

# Save to a file
❯ kubectl actuator --pod my-app-pod logfile --output-file app.log
```

### Raw Endpoint Access

```bash
# Get raw JSON from any endpoint
❯ kubectl actuator --pod my-app-pod raw health
{
  "pods": [
    {
      "name": "my-app-pod",
      "data": {
        "status": "UP",
        "components": { ... }
      },
      "error": null
    }
  ]
}

# Access endpoints not directly supported by this tool
❯ kubectl actuator --pod my-app-pod raw mappings
❯ kubectl actuator --pod my-app-pod raw configprops
❯ kubectl actuator --pod my-app-pod raw conditions
```

### Scripting (JSON/YAML output)

All read commands support `-o json` and `-o yaml`. The output is a per-pod envelope; `data` carries the actuator
endpoint's own response schema, with command filters applied:

```bash
❯ kubectl actuator --pod my-app-pod health -o json
{
  "pods": [
    {
      "name": "my-app-pod",
      "data": {
        "status": "UP",
        "components": { ... }
      },
      "error": null
    }
  ]
}

# Works with filters and multiple pods
❯ kubectl actuator -d my-app env --filter spring -o json | jq '.pods[].data'
```

Pods that fail are reported in their `error` field and the command exits non-zero.

## Building from Source

```bash
# Clone the repository
git clone https://github.com/deviceinsight/kubectl-actuator.git
cd kubectl-actuator

# Build
make build

# Install
mv kubectl-actuator ~/.local/bin/
```

## Development

```bash
# Unit tests and static checks
make test
make lint

# Integration tests (requires Docker)
make test-integration

# Spin up a local k3s cluster with a test app, for trying commands by hand
make start-testenvironment
```

## License

[Apache-2.0](LICENSE)
