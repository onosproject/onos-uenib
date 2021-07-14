# Deploying onos-topo

This guide deploys `onos-uenib` through its [Helm] chart assumes you have a [Kubernetes] cluster running 
with an atomix controller deployed in a namespace.
`onos-uenib` Helm chart is based on Helm 3.0 version, with no need for the Tiller pod to be present. 
If you don't have a cluster running and want to try on your local machine please follow first 
the [Kubernetes] setup steps outlined in [deploy with Helm](https://docs.onosproject.org/developers/deploy_with_helm/).
The following steps assume you have the setup outlined in that page, including the `micro-onos` namespace configured. 

## Installing the Chart

To install the chart in the `micro-onos` namespace run from the root directory of the `onos-helm-charts` repo the command:
```bash
helm install -n micro-onos onos-uenib onos-uenib
```
The output should be:
```bash
NAME: onos-uenib
LAST DEPLOYED: Wed Jul 14 14:41:16 2021
NAMESPACE: micro-onos
STATUS: deployed
REVISION: 1
TEST SUITE: None
```

`helm install` assigns a unique name to the chart and displays all the k8s resources that were
created by it. To list the charts that are installed and view their statuses, run `helm ls`:

```bash
$ helm -n micro-onos ls
NAME     	NAMESPACE 	REVISION	UPDATED                            	STATUS  	CHART           	APP VERSION
onos-uenib	micro-onos	1       	2021-07-14 14:41:16.860966 -0700 PDT	deployed	onos-uenib-1.0.3	v0.0.3
```

### Topology Partition Set

The `onos-uenib` chart also deploys a custom Atomix `PartitionSet` resource to store all the 
topology information in a replicated and fail-safe manner. 
In the following example there is only one partition set deployed `onos-uenib-consensus-store-1-0`.

```bash
$ kubectl -n micro-onos get pods
NAME                            READY   STATUS    RESTARTS   AGE
onos-uenib-5d49c8d8b6-wbkl2      2/3     Running   0          3m8s
onos-uenib-consensus-store-1-0   1/1     Running   0          3m6s
```

One can customize the number of partitions and replicas by modifying, in `values.yaml`, under `store/raft` 
the values of 
```bash 
partitions: 1
partitionSize: 1
```

### Installing the chart in a different namespace.

Issue the `helm install` command substituting `micro-onos` with your namespace.
```bash
helm install -n <your_name_space> onos-uenib onos-uenib
```

### Troubleshoot

Helm offers two flags to help you debug your chart. This can be useful if your chart does not install, 
the pod is not running for some reason, or you want to trouble-shoot custom configuration values,

* `--dry-run` check the chart without actually installing the pod. 
* `--debug` prints out more information about your chart

```bash
helm install -n micro-onos onos-uenib --debug --dry-run onos-uenib/
```
## Uninstalling the chart.

To remove the `onos-uenib` pod issue
```bash
 helm delete -n micro-onos onos-uenib
```
## Pod Information

To view the pods that are deployed, run `kubectl -n micro-onos get pods`.

[Helm]: https://helm.sh/
[Kubernetes]: https://kubernetes.io/
[kind]: https://kind.sigs.k8s.io

