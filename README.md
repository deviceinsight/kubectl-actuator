# kubectl-actuator

This kubectl plugin allows you to interact with Spring Boot Actuator endpoints in your Kubernetes cluster. Currently it
provides two main functionalities: listing loggers and setting logger levels.

## Installation

To install `kubectl-actuator`, you can download the latest release from
the [GitHub releases page](https://github.com/deviceinsight/kubectl-actuator/releases). Extract the downloaded archive
and move the `kubectl-actuator` binary to a directory in your PATH.

For [shell completion](https://github.com/kubernetes/kubernetes/pull/105867) to work, you need to have at least kubectl
version 1.26 installed. Also, `kubectl_complete-actuator` needs to be symlinked to `kubectl-actuator`. For example:

```bash
ln -sr ~/.local/bin/kubectl-actuator ~/.local/bin/kubectl_complete-actuator
```

## Usage

The plugin provides two subcommands: `pod` and `deployment`. You can use the `pod` subcommand to execute Actuator
commands for a specific pod, and the `deployment` subcommand to execute Actuator commands for a deployment.

### List loggers

```
❯ kubectl actuator pod <pod-name> logger
LOGGER                                               LEVEL
ROOT                                                 INFO
com.example.app                                      INFO
org.apache.catalina.startup.DigesterFactory          ERROR
org.apache.catalina.util.LifecycleBase               ERROR
org.apache.coyote.http11.Http11NioProtocol           WARN
org.apache.kafka                                     WARN
org.apache.sshd.common.util.SecurityUtils            WARN
org.apache.tomcat.util.net.NioSelectorPool           WARN
org.eclipse.jetty.util.component.AbstractLifeCycle   ERROR
org.hibernate.validator.internal.util.Version        WARN
org.springframework.boot.actuate.endpoint.jmx        WARN
```

### Set logger level

```
❯ actuator pod <pod-name> logger com.example.app INFO
```

### Configuration

When using the default Spring Boot configuration this plugin should with without additional configuration. However, if
actuator runs on a non-standard port or path, you can configure this via labels on the pod.

* `kubectl-actuator.device-insight.com/basePath`: Actuator base path. Defaults to `actuator`
* `kubectl-actuator.device-insight.com/port`: Actuator port. Defaults to `9090`
