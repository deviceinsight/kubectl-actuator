# kubectl-actuator-plugin

## Purpose

This kubectl cli plugin enables you to interact with the Spring Boot Actuator component.

## Functionality

Retrieving the logger configuration if an application:
```
❯ kubectl actuator logger get --pod ms-aks-dev-ms-asset-manager-66d5ff5f66-czp7w
LOGGER                                               LEVEL
ROOT                                                 INFO
com.azure.core.amqp                                  WARN
com.azure.messaging.eventhubs                        WARN
com.deviceinsight.ms                                 INFO
org.apache.catalina.startup.DigesterFactory          ERROR
org.apache.catalina.util.LifecycleBase               ERROR
org.apache.coyote.http11.Http11NioProtocol           WARN
org.apache.sshd.common.util.SecurityUtils            WARN
org.apache.tomcat.util.net.NioSelectorPool           WARN
org.eclipse.jetty.util.component.AbstractLifeCycle   ERROR
org.hibernate.validator.internal.util.Version        WARN
org.springframework.boot.actuate.endpoint.jmx        WARN
```

Setting the level of a logger:
```
❯ kubectl actuator logger set --pod ms-aks-dev-ms-asset-manager-66d5ff5f66-czp7w com.deviceinsight.ms INFO
```

## Installation
Right now there is no binary build available, so you have to build the plugin yourself:

```
❯ git clone git@gitlab.device-insight.com:mwa/kubectl-actuator-plugin.git
❯ cd kubectl-actuator-plugin/
❯ go build
❯ cp -p kubectl-actuator-plugin ~/.local/bin/kubectl-actuator
❯ kubectl actuator --help
```

## TODO
* Code cleanup
* Investigate why command line completion is not working
  * Provide autocomplete support for logger and log levels
* Validate log level options
* Support deployments and statefulsets
* Make Actuator port and URL configurable via pod labels
* Support other actuator components
