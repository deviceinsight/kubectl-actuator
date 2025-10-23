# kubectl-actuator

A kubectl plugin for interacting with Spring Boot Actuator endpoints.

## Features

- **Logger Management**: List all loggers and dynamically change log levels at runtime
- **Scheduled Tasks Monitoring**: View scheduled tasks with execution status, timing, and schedules
- **Application Info**: View application build and runtime information

## Installation

Make sure you have [krew](https://krew.sigs.k8s.io/) installed.

```bash
# Install the plugin
kubectl krew install --manifest-url https://raw.githubusercontent.com/deviceinsight/kubectl-actuator/refs/heads/main/.krew.yaml

# Enable shell completion (optional)
ln -sr ~/.krew/bin/kubectl-actuator ~/.krew/bin/kubectl_complete-actuator
```

## Configuration

### Pod Annotations

By default, the plugin expects Spring Boot Actuator on `http://localhost:8080/actuator`. Customize via pod annotations:

- `kubectl-actuator.device-insight.com/port`: Actuator port (default: `8080`)
- `kubectl-actuator.device-insight.com/basePath`: Actuator base path (default: `actuator`)

## Usage

### Global Flags

All commands support target selection:

- `--pod <pod-name>` or `-p`: Target one or more specific pods
- `--deployment <deployment-name>` or `-d`: Target all pods in a deployment
- `--selector <label-selector>` or `-l`: Target pods by label selector (e.g., `app=myapp,env=prod`)

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

**Available log levels**: TRACE, DEBUG, INFO, WARN, ERROR, FATAL, OFF

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

### Application Info

View application build and runtime information:

```bash
❯ kubectl actuator --pod my-app-pod info
{
  "build": {
    "artifact": "my-app",
    "name": "my-app",
    "time": "2025-10-21T22:34:55.709Z",
    "version": "1.0.0",
    "group": "com.example"
  },
  "kubernetes": {
    "nodeName": "node-1",
    "podIp": "10.0.0.23",
    "hostIp": "10.0.0.10",
    "namespace": "default",
    "podName": "my-app-5d4c8f9b-xk7pq",
    "serviceAccount": "my-app",
    "inside": true
  }
}
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
