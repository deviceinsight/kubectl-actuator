# kubectl-actuator

A kubectl plugin for interacting with Spring Boot Actuator endpoints.

## Features

- **Logger Management**: List all loggers and dynamically change log levels at runtime
- **Scheduled Tasks Monitoring**: View scheduled tasks with execution status, timing, and schedules
- **Application Info**: View application build and runtime information
- **Health Status**: Check application health and component status
- **Metrics**: View and filter application metrics (JVM, HTTP, custom metrics)
- **Environment**: Inspect environment properties and configuration
- **Thread Dump**: Analyze thread states and detect potential issues
- **Beans**: View Spring bean registry and dependencies

## Installation

Make sure you have [krew](https://krew.sigs.k8s.io/) installed.

```bash
# Install the plugin
kubectl krew install actuator

# Enable shell completion (optional)
ln -sr ~/.krew/bin/kubectl-actuator ~/.krew/bin/kubectl_complete-actuator
```

## Configuration

### Actuator Endpoint Configuration

By default, the plugin expects Spring Boot Actuator on `http://localhost:8080/actuator`. You can customize this in two ways:

#### Command-line Flags (highest priority)

- `--port <port>`: Override actuator port
- `--base-path <path>`: Override actuator base path

Example:
```bash
kubectl actuator --pod my-app-pod --port 9090 --base-path management/actuator logger
```

#### Pod Annotations

- `kubectl-actuator.device-insight.com/port`: Actuator port (default: `8080`)
- `kubectl-actuator.device-insight.com/basePath`: Actuator base path (default: `actuator`)

**Note:** Command-line flags take precedence over pod annotations, which take precedence over defaults.

## Usage

### Global Flags

All commands support target selection:

- `--pod <pod-name>` or `-p`: Target one or more specific pods
- `--deployment <deployment-name>` or `-d`: Target all pods in a deployment
- `--selector <label-selector>` or `-l`: Target pods by label selector (e.g., `app=myapp,env=prod`)

Actuator configuration:

- `--port <port>`: Override actuator port (default: read from pod annotation or `8080`)
- `--base-path <path>`: Override actuator base path (default: read from pod annotation or `actuator`)

Standard kubectl flags such as `--namespace` or `--context` are also supported.

### Loggers

#### List all loggers

View current logger configuration:

```bash
❯ kubectl actuator --pod my-app-pod logger
LOGGER                                               LEVEL
ROOT                                                 INFO
com.example.app                                      INFO
com.example.app.service                              DEBUG
org.apache.catalina.startup.DigesterFactory          ERROR
org.apache.catalina.util.LifecycleBase               ERROR
org.springframework.web                              INFO
```

#### Set logger level

Change a logger's level at runtime:

```bash
# Set a specific logger to DEBUG
❯ kubectl actuator --pod my-app-pod logger com.example.app.service DEBUG

# Set ROOT logger level
❯ kubectl actuator --pod my-app-pod logger ROOT WARN
```

**Available log levels**: TRACE, DEBUG, INFO, WARN, ERROR, FATAL, OFF, RESET

**Note:** Use `RESET` to clear a configured level and inherit from the parent logger.

#### Show all loggers

By default, only loggers with explicitly configured levels are shown. Use `--all-loggers` to see all loggers including those inheriting their level:

```bash
❯ kubectl actuator --pod my-app-pod logger --all-loggers
LOGGER                                               LEVEL
ROOT                                                 INFO
com                                                  INFO (effective)
com.example                                          INFO (effective)
com.example.app                                      DEBUG
com.example.app.service                              DEBUG (effective)
```

### Scheduled Tasks

View scheduled tasks with execution details:

```bash
❯ kubectl actuator --deployment my-app scheduled-tasks
TYPE         TARGET                                  SCHEDULE                           NEXT            LAST         STATUS
cron         BackupScheduler.scheduleBackups         cron(0 * * * * *)                  in 49s          11s ago      SUCCESS
fixedDelay   CacheRefreshService.refreshCache        fixedDelay=5m                      in 4m33s        27s ago      SUCCESS
fixedDelay   HealthCheckService.checkServiceHealth   fixedDelay=12h initialDelay=15m    in 11h59m58s    27s ago      ERROR - Connection timeout
fixedDelay   CleanupScheduler.triggerCleanup         fixedDelay=24h                     in 23h44m33s    15m27s ago   SUCCESS
fixedDelay   StatusWatcher.checkStatus               fixedDelay=5s                      -               2s ago       STARTED
fixedRate    UpdateService.checkForUpdates           fixedRate=30m                      in 14m33s       15m27s ago   SUCCESS
```

**Note:** Execution tracking (NEXT, LAST, STATUS columns) requires Spring Boot 3.4.0 or later. Earlier versions will only show task type, target, and schedule information.

### Application Info

View application build and runtime information:

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

Kubernetes:
  Namespace:     default
  Pod Name:      my-app-5d4c8f9b-xk7pq
  Pod IP:        10.0.0.23
  Host IP:       10.0.0.10
  Node Name:     node-1
  Service Account: my-app
```

### Health

Check application health status and component health:

```bash
❯ kubectl actuator --pod my-app-pod health
COMPONENT       STATUS
diskSpace       UP
livenessState   UP
ping            UP
readinessState  UP
ssl             UP
[overall]       UP
```

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

View application metrics:

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
```

### Environment

Inspect environment properties and configuration:

```bash
# View all environment properties (shows active profiles and all properties)
❯ kubectl actuator --pod my-app-pod env

# Filter properties by name pattern
❯ kubectl actuator --pod my-app-pod env --filter server.port
Active Profiles: []

NAME               VALUE  ORIGIN
local.server.port  8080   server.ports

# Filter and show only names
❯ kubectl actuator --pod my-app-pod env --filter spring -o name
spring.application.version
spring.application.pid
spring.application.name
```

**Note:** To get the raw JSON response from the `/actuator/env` endpoint, use:
```bash
❯ kubectl actuator --pod my-app-pod raw env
```

### Thread Dump

Analyze application threads:

```bash
# Get full thread dump
❯ kubectl actuator --pod my-app-pod threaddump
Total Threads: 45

Thread States:
  RUNNABLE: 12
  TIMED_WAITING: 28
  WAITING: 5

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
❯ kubectl actuator --pod my-app-pod threaddump --name "http-nio"

# Show summary only
❯ kubectl actuator --pod my-app-pod threaddump --summary

# Show thread list without stack traces
❯ kubectl actuator --pod my-app-pod threaddump --no-stacktrace
```

### Beans

View Spring application beans:

```bash
# List all beans in table format (default)
❯ kubectl actuator --pod my-app-pod beans
NAME                        TYPE                                  SCOPE        DEPENDENCIES
applicationTaskExecutor     o.s.scheduling.concurrent.Thread...   singleton    2
basicErrorController        o.s.b.ac.web.servlet.error.Basic...   singleton    2
beansEndpoint               o.s.boot.actuate.beans.BeansEndp...   singleton    2
cachesEndpoint              o.s.boot.actuate.cache.CachesEnd...   singleton    1
...

# Filter beans by name
❯ kubectl actuator --pod my-app-pod beans --filter controller
NAME                  TYPE                                  SCOPE        DEPENDENCIES
basicErrorController  o.s.b.ac.web.servlet.error.Basic...   singleton    2
userController        com.example.app.UserController        singleton    3

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

# List only bean names with -o name
❯ kubectl actuator --pod my-app-pod beans -o name
userController
userService
dataSource
...
```

### Raw Endpoint Access

Access any actuator endpoint and get raw JSON output:

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

### Multi-Target Operations

#### Target multiple pods

```bash
# Target specific pods
❯ kubectl actuator --pod app-pod-1 --pod app-pod-2 logger

app-pod-1:
LOGGER                    LEVEL
ROOT                      INFO
com.example.app           DEBUG

app-pod-2:
LOGGER                    LEVEL
ROOT                      INFO
com.example.app           DEBUG
```

#### Target deployments

Automatically targets all pods from the deployment's selector:

```bash
# Target all pods in a deployment
❯ kubectl actuator --deployment my-app logger

# Target multiple deployments
❯ kubectl actuator --deployment app-1 --deployment app-2 scheduled-tasks
```

#### Target by label selector

Use standard Kubernetes label selectors to target pods:

```bash
# Target pods by label
❯ kubectl actuator --selector app.kubernetes.io/component=backend logger

# Combine with other target options
❯ kubectl actuator --selector app.kubernetes.io/component=backend --deployment frontend-app logger
```

## Building from Source

```bash
# Clone the repository
git clone https://github.com/deviceinsight/kubectl-actuator.git
cd kubectl-actuator

# Build
go build -o kubectl-actuator .

# Install
mv kubectl-actuator ~/.local/bin/
```

## License

See [LICENSE](LICENSE) file for details.
