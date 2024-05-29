# API Reference

## Packages
- [lke.anza-labs.dev/v1alpha1](#lkeanza-labsdevv1alpha1)


## lke.anza-labs.dev/v1alpha1

Package v1alpha1 contains API Schema definitions for the lke v1alpha1 API group

### Resource Types
- [LKEClusterConfig](#lkeclusterconfig)



#### LKEClusterConfig



LKEClusterConfig is the Schema for the lkeclusterconfigs API.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `lke.anza-labs.dev/v1alpha1` | | |
| `kind` _string_ | `LKEClusterConfig` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[LKEClusterConfigSpec](#lkeclusterconfigspec)_ |  |  |  |


#### LKEClusterConfigSpec



LKEClusterConfigSpec defines the desired state of an LKEClusterConfig resource.



_Appears in:_
- [LKEClusterConfig](#lkeclusterconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `region` _string_ | Region is the geographical region where the LKE cluster will be provisioned. |  | Required: {} <br /> |
| `tokenSecretRef` _[SecretRef](#secretref)_ | TokenSecretRef references the Kubernetes secret that stores the Linode API token.<br />If not provided, then default token will be used. |  | Required: {} <br /> |
| `highAvailability` _boolean_ | HighAvailability specifies whether the LKE cluster should be configured for high<br />availability. | false | Optional: {} <br /> |
| `nodePools` _object (keys:string, values:[LKENodePool](#lkenodepool))_ | NodePools contains the specifications for each node pool within the LKE cluster. |  | MinProperties: 1 <br />Required: {} <br /> |
| `kubernetesVersion` _string_ | KubernetesVersion indicates the Kubernetes version of the LKE cluster. | latest | Optional: {} <br /> |


#### LKENodePool



LKENodePool represents a pool of nodes within the LKE cluster.



_Appears in:_
- [LKEClusterConfigSpec](#lkeclusterconfigspec)
- [NodePoolStatus](#nodepoolstatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `nodeCount` _integer_ | NodeCount specifies the number of nodes in the node pool. |  | Required: {} <br /> |
| `linodeType` _string_ | LinodeType specifies the Linode instance type for the nodes in the pool. |  | Required: {} <br /> |
| `autoscaler` _[LKENodePoolAutoscaler](#lkenodepoolautoscaler)_ | Autoscaler specifies the autoscaling configuration for the node pool. |  | Optional: {} <br /> |


#### LKENodePoolAutoscaler



LKENodePoolAutoscaler represents the autoscaler configuration for a node pool.



_Appears in:_
- [LKENodePool](#lkenodepool)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `min` _integer_ | Min specifies the minimum number of nodes in the pool. |  | Maximum: 100 <br />Minimum: 0 <br />Required: {} <br /> |
| `max` _integer_ | Max specifies the maximum number of nodes in the pool. |  | Maximum: 100 <br />Minimum: 3 <br />Required: {} <br /> |






#### SecretRef



SecretRef references a Kubernetes secret.



_Appears in:_
- [LKEClusterConfigSpec](#lkeclusterconfigspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `namespace` _string_ |  |  |  |
| `name` _string_ |  |  |  |


