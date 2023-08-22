package clustertype

// Define a enum type for the cluster type
type ClusterType string

// Define the enum values
const (
	ClusterTypeHybrid ClusterType = "CLUSTER_TYPE_HYBRID"
	ClusterTypeKiC       ClusterType = "CLUSTER_TYPE_K8S_INGRESS_CONTROLLER"
	ClusterTypeComposite        ClusterType = "CLUSTER_TYPE_COMPOSITE"
)
