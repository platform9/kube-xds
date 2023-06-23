# kube-xds

Kube-xds is a small xDS server configurable using Kubernetes APIs (such as a ConfigMap).

It is intentionally lightweight with its primary use case being situations in
which it is not possible or allowed to use 
[dynamic configuration from the filesystem](https://www.envoyproxy.io/docs/envoy/latest/start/quick-start/configuration-dynamic-filesystem)
and where using a service mesh (such as Istio) is (not yet) possible or feasible.

## Installation

### 1. Deploy kube-xds 

#### A. Cluster 

Assuming the `KUBECONFIG` is set to the target cluster, run the following to deploy

```bash
kubectl apply -k ./config/default
```

#### B. Locally

Assuming the `KUBECONFIG` is set to the target cluster, to run kube-xds locally 
for development or testing:

```bash
go run .
```

### 2. Configure Envoy Proxy

To make Envoy to use kube-xds, ensure that the following configuration is 
present in the Envoy bootstrap configuration: 

```yaml
dynamic_resources:
  ads_config:
    api_type: GRPC
    transport_api_version: V3
    grpc_services:
      envoy_grpc:
        cluster_name: service_kube_xds
  lds_config: 
    ads: {}
  cds_config:
    ads: {}
clusters:
  - name: service_kube_xds
    connect_timeout: 1s
    type: LOGICAL_DNS
    lb_policy: ROUND_ROBIN
    http2_protocol_options: {}
    load_assignment:
      cluster_name: service_kube_xds
      endpoints:
        - lb_endpoints:
            - endpoint:
                address:
                  socket_address:
                    protocol: TCP
                    # Replace this with the address where kube-xds is running.
                    address: kube-xds
                    port_value: 18000
```

## Usage

### Provision an Envoy node with a static configuration

The easiest way to get started is to define a static bootstrap configuration for 
Envoy, the same way as you would when using 
[static configuration](https://www.envoyproxy.io/docs/envoy/latest/start/quick-start/configuration-static).

To configure a node simply create a ConfigMap with an 
Envoy bootstrap config.
Add the `xds.pf9.io/kind` label to ensure that kube-xds picks it up.

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  labels:
    # The label is used to help kube-xds filter ConfigMaps for ones related to 
    # xDS. It is also used to identify the type of Envoy config present in.
    xds.pf9.io/kind: "Bootstrap"
  name: example-xds
data:
  # Any key will work, but "envoy.json" is considered to be a sensible default.
  envoy.json: |-
    {
      "node": {
        "id": "front.proxies.example.com",
        "cluster": "proxies.example.com"
      },
      "staticResources": {
        "clusters": [
          {
            "name": "example_dynamic_service",
            "type": "LOGICAL_DNS",
            "loadAssignment": {
              "clusterName": "example_dynamic_service",
              "endpoints": [
                {
                  "lbEndpoints": [
                    {
                      "endpoint": {
                        "address": {
                          "socketAddress": {
                            "address": "example.com",
                            "portValue": 443
                          }
                        }
                      }
                    }
                  ]
                }
              ]
            },
            "dnsLookupFamily": "V4_ONLY",
            "transportSocket": {
              "name": "envoy.transport_sockets.tls",
              "typedConfig": {
                "@type": "type.googleapis.com/envoy.extensions.transport_sockets.tls.v3.UpstreamTlsContext"
              }
            }
          }
        ]
      }
    }
```

:warning: Currently no merging of configs is supported; only one config should 
be set per node ID to avoid conflicts.
