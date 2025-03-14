// Copyright Project Contour Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package model

import (
	contourv1alpha1 "github.com/projectcontour/contour/apis/projectcontour/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
)

const (
	// OwningGatewayNameLabel is the owner reference label used for a Contour
	// created by the gateway provisioner. The value should be the name of the Gateway.
	OwningGatewayNameLabel = "projectcontour.io/owning-gateway-name"
)

// Default returns a default instance of a Contour
// for the given namespace/name.
func Default(namespace, name string) *Contour {
	return &Contour{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Spec: ContourSpec{
			ContourReplicas:   2,
			EnvoyWorkloadType: WorkloadTypeDaemonSet,
			EnvoyReplicas:     2, // ignored if not provisioning Envoy as a deployment.
			NetworkPublishing: NetworkPublishing{
				Envoy: EnvoyNetworkPublishing{
					Type: LoadBalancerServicePublishingType,
					ContainerPorts: []ContainerPort{
						{
							Name:       "http",
							PortNumber: 8080,
						},
						{
							Name:       "https",
							PortNumber: 8443,
						},
					},
				},
			},
		},
	}
}

// Contour is the representation of an instance of Contour + Envoy.
type Contour struct {
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired state of Contour.
	Spec ContourSpec `json:"spec,omitempty"`
}

// ContourNodeSelectorExists returns true if a nodeSelector is specified for Contour.
func (c *Contour) ContourNodeSelectorExists() bool {
	if c.Spec.NodePlacement != nil &&
		c.Spec.NodePlacement.Contour != nil &&
		c.Spec.NodePlacement.Contour.NodeSelector != nil {
		return true
	}

	return false
}

// ContourTolerationsExist returns true if tolerations are set for Contour.
func (c *Contour) ContourTolerationsExist() bool {
	if c.Spec.NodePlacement != nil &&
		c.Spec.NodePlacement.Contour != nil &&
		len(c.Spec.NodePlacement.Contour.Tolerations) > 0 {
		return true
	}

	return false
}

// EnvoyNodeSelectorExists returns true if a nodeSelector is specified for Envoy.
func (c *Contour) EnvoyNodeSelectorExists() bool {
	if c.Spec.NodePlacement != nil &&
		c.Spec.NodePlacement.Envoy != nil &&
		c.Spec.NodePlacement.Envoy.NodeSelector != nil {
		return true
	}

	return false
}

// EnvoyTolerationsExist returns true if tolerations are set for Envoy.
func (c *Contour) EnvoyTolerationsExist() bool {
	if c.Spec.NodePlacement != nil &&
		c.Spec.NodePlacement.Envoy != nil &&
		len(c.Spec.NodePlacement.Envoy.Tolerations) > 0 {
		return true
	}

	return false
}

// ContourSpec defines the desired state of Contour.
type ContourSpec struct {
	// ContourReplicas is the desired number of Contour replicas. If unset,
	// defaults to 2.
	ContourReplicas int32

	// EnvoyReplicas is the desired number of Envoy replicas. If WorkloadType
	// is not "Deployment", this field is ignored. Otherwise, if unset,
	// defaults to 2.
	EnvoyReplicas int32

	// NetworkPublishing defines the schema for publishing Contour to a network.
	//
	// See each field for additional details.
	NetworkPublishing NetworkPublishing

	// GatewayControllerName is used to determine which GatewayClass
	// Contour reconciles. The string takes the form of
	// "projectcontour.io/<namespace>/contour". If unset, Contour will not
	// reconcile Gateway API resources.
	GatewayControllerName *string

	// IngressClassName is the name of the IngressClass used by Contour. If unset,
	// Contour will process all ingress objects without an ingress class annotation
	// or ingress objects with an annotation matching ingress-class=contour. When
	// specified, Contour will only process ingress objects that match the provided
	// class.
	//
	// For additional IngressClass details, refer to:
	//   https://projectcontour.io/docs/main/config/annotations/#ingress-class
	IngressClassName *string

	// LogLevel sets the log level for Contour
	// Allowed values are "info", "debug".
	LogLevel contourv1alpha1.LogLevel

	// NodePlacement enables scheduling of Contour and Envoy pods onto specific nodes.
	//
	// See each field for additional details.
	NodePlacement *NodePlacement

	// EnableExternalNameService enables ExternalName Services.
	// ExternalName Services are disabled by default due to CVE-2021-XXXXX
	// You can re-enable them by setting this setting to "true".
	// This is not recommended without understanding the security implications.
	// Please see the advisory at https://github.com/projectcontour/contour/security/advisories/GHSA-5ph6-qq5x-7jwc for the details.
	EnableExternalNameService *bool

	// RuntimeSettings is any user-defined ContourConfigurationSpec to use when provisioning.
	RuntimeSettings *contourv1alpha1.ContourConfigurationSpec

	// EnvoyWorkloadType is the way to deploy Envoy, either "DaemonSet" or "Deployment".
	EnvoyWorkloadType WorkloadType

	// KubernetesLogLevel Enable Kubernetes client debug logging with log level. If unset,
	// defaults to 0.
	KubernetesLogLevel uint8
}

// WorkloadType is the type of Kubernetes workload to use for a component.
type WorkloadType = contourv1alpha1.WorkloadType

const (
	// A Kubernetes DaemonSet.
	WorkloadTypeDaemonSet = contourv1alpha1.WorkloadTypeDaemonSet

	// A Kubernetes Deployment.
	WorkloadTypeDeployment = contourv1alpha1.WorkloadTypeDeployment
)

// NodePlacement describes node scheduling configuration of Contour and Envoy pods.
type NodePlacement struct {
	// Contour describes node scheduling configuration of Contour pods.
	Contour *ContourNodePlacement

	// Envoy describes node scheduling configuration of Envoy pods.
	Envoy *EnvoyNodePlacement
}

// ContourNodePlacement describes node scheduling configuration for Contour pods.
// If nodeSelector and tolerations are specified, the scheduler will use both to
// determine where to place the Contour pod(s).
type ContourNodePlacement struct {
	// NodeSelector is the simplest recommended form of node selection constraint
	// and specifies a map of key-value pairs. For the Contour pod to be eligible
	// to run on a node, the node must have each of the indicated key-value pairs
	// as labels (it can have additional labels as well).
	//
	// If unset, the Contour pod(s) will be scheduled to any available node.
	NodeSelector map[string]string

	// Tolerations work with taints to ensure that Envoy pods are not scheduled
	// onto inappropriate nodes. One or more taints are applied to a node; this
	// marks that the node should not accept any pods that do not tolerate the
	// taints.
	//
	// The default is an empty list.
	//
	// See https://kubernetes.io/docs/concepts/configuration/taint-and-toleration/
	// for additional details.
	Tolerations []corev1.Toleration
}

// EnvoyNodePlacement describes node scheduling configuration for Envoy pods.
// If nodeSelector and tolerations are specified, the scheduler will use both
// to determine where to place the Envoy pod(s).
type EnvoyNodePlacement struct {
	// NodeSelector is the simplest recommended form of node selection constraint
	// and specifies a map of key-value pairs. For the Envoy pod to be eligible to
	// run on a node, the node must have each of the indicated key-value pairs as
	// labels (it can have additional labels as well).
	//
	// If unset, the Envoy pod(s) will be scheduled to any available node.
	NodeSelector map[string]string

	// Tolerations work with taints to ensure that Envoy pods are not scheduled
	// onto inappropriate nodes. One or more taints are applied to a node; this
	// marks that the node should not accept any pods that do not tolerate the taints.
	//
	// The default is an empty list.
	//
	// See https://kubernetes.io/docs/concepts/configuration/taint-and-toleration/
	// for additional details.
	Tolerations []corev1.Toleration
}

// NamespaceSpec defines the schema of a Contour namespace.
type NamespaceSpec struct {
	// Name is the name of the namespace to run Contour and dependent
	// resources. If unset, defaults to "projectcontour".
	Name string

	// RemoveOnDeletion will remove the namespace when the Contour is
	// deleted. If set to True, deletion will not occur if any of the
	// following conditions exist:
	//
	// 1. The Contour namespace is "default", "kube-system" or the
	//    contour-operator's namespace.
	//
	// 2. Another Contour exists in the namespace.
	//
	// 3. The namespace does not contain the Contour owning label.
	RemoveOnDeletion bool
}

// NetworkPublishing defines the schema for publishing Contour to a network.
type NetworkPublishing struct {
	// Envoy provides the schema for publishing the network endpoints of Envoy.
	//
	// If unset, defaults to:
	//   type: LoadBalancerService
	//   containerPorts:
	//   - name: http
	//     portNumber: 8080
	//   - name: https
	//     portNumber: 8443
	Envoy EnvoyNetworkPublishing
}

// EnvoyNetworkPublishing defines the schema to publish Envoy to a network.
type EnvoyNetworkPublishing struct {
	// Type is the type of publishing strategy to use. Valid values are:
	//
	// * LoadBalancerService
	//
	// In this configuration, network endpoints for Envoy use container networking.
	// A Kubernetes LoadBalancer Service is created to publish Envoy network
	// endpoints. The Service uses port 80 to publish Envoy's HTTP network endpoint
	// and port 443 to publish Envoy's HTTPS network endpoint.
	//
	// See: https://kubernetes.io/docs/concepts/services-networking/service/#loadbalancer
	//
	// * NodePortService
	//
	// Publishes Envoy network endpoints using a Kubernetes NodePort Service.
	//
	// In this configuration, Envoy network endpoints use container networking. A Kubernetes
	// NodePort Service is created to publish the network endpoints.
	//
	// See: https://kubernetes.io/docs/concepts/services-networking/service/#nodeport
	//
	// * ClusterIPService
	//
	// Publishes Envoy network endpoints using a Kubernetes ClusterIP Service.
	//
	// In this configuration, Envoy network endpoints use container networking. A Kubernetes
	// ClusterIP Service is created to publish the network endpoints.
	//
	// See: https://kubernetes.io/docs/concepts/services-networking/service/#publishing-services-service-types
	Type NetworkPublishingType

	// LoadBalancer holds parameters for the load balancer. Present only if type is
	// LoadBalancerService.
	//
	// If unspecified, defaults to an external Classic AWS ELB.
	LoadBalancer LoadBalancerStrategy

	// ServicePorts is a list of ports to expose on the Envoy service.
	// TODO(sk) ServicePorts, NodePorts and ContainerPorts should collapse
	// into a single struct.
	ServicePorts []ServicePort

	// NodePorts is a list of network ports to expose on each node's IP at a static
	// port number using a NodePort Service. Present only if type is NodePortService.
	// A ClusterIP Service, which the NodePort Service routes to, is automatically
	// created. You'll be able to contact the NodePort Service, from outside the
	// cluster, by requesting <NodeIP>:<NodePort>.
	//
	// If type is NodePortService and nodePorts is unspecified, two nodeports will be
	// created, one named "http" and the other named "https", with port numbers auto
	// assigned by Kubernetes API server. For additional information on the NodePort
	// Service, see:
	//
	//  https://kubernetes.io/docs/concepts/services-networking/service/#nodeport
	//
	// Names and port numbers must be unique in the list. Two ports must be specified,
	// one named "http" for Envoy's insecure service and one named "https" for Envoy's
	// secure service.
	NodePorts []NodePort

	// ContainerPorts is a list of container ports to expose from the Envoy container(s).
	// Exposing a port here gives the system additional information about the network
	// connections the Envoy container uses, but is primarily informational. Not specifying
	// a port here DOES NOT prevent that port from being exposed by Envoy. Any port which is
	// listening on the default "0.0.0.0" address inside the Envoy container will be accessible
	// from the network. Names and port numbers must be unique in the list container ports. Two
	// ports must be specified, one named "http" for Envoy's insecure service and one named
	// "https" for Envoy's secure service.
	ContainerPorts []ContainerPort

	// ServiceAnnotations is a set of annotations to add to the provisioned Envoy service.
	ServiceAnnotations map[string]string
}

type NetworkPublishingType = contourv1alpha1.NetworkPublishingType

const (
	// LoadBalancerServicePublishingType publishes a network endpoint using a Kubernetes
	// LoadBalancer Service.
	LoadBalancerServicePublishingType NetworkPublishingType = contourv1alpha1.LoadBalancerServicePublishingType

	// NodePortServicePublishingType publishes a network endpoint using a Kubernetes
	// NodePort Service.
	NodePortServicePublishingType NetworkPublishingType = contourv1alpha1.NodePortServicePublishingType

	// ClusterIPServicePublishingType publishes a network endpoint using a Kubernetes
	// ClusterIP Service.
	ClusterIPServicePublishingType NetworkPublishingType = contourv1alpha1.ClusterIPServicePublishingType
)

// LoadBalancerStrategy holds parameters for a load balancer.
type LoadBalancerStrategy struct {
	// Scope indicates the scope at which the load balancer is exposed.
	// Possible values are "External" and "Internal".
	Scope LoadBalancerScope

	// ProviderParameters contains load balancer information specific to
	// the underlying infrastructure provider.
	ProviderParameters ProviderLoadBalancerParameters

	// LoadBalancerIP is the IP (or hostname) to request
	// for the LoadBalancer service.
	LoadBalancerIP string
}

// LoadBalancerScope is the scope at which a load balancer is exposed.
// +kubebuilder:validation:Enum=Internal;External
type LoadBalancerScope string

var (
	// InternalLoadBalancer is a load balancer that is exposed only on the
	// cluster's private network.
	InternalLoadBalancer LoadBalancerScope = "Internal"

	// ExternalLoadBalancer is a load balancer that is exposed on the
	// cluster's public network (which is typically on the Internet).
	ExternalLoadBalancer LoadBalancerScope = "External"
)

// ProviderLoadBalancerParameters holds desired load balancer information
// specific to the underlying infrastructure provider.
type ProviderLoadBalancerParameters struct {
	// Type is the underlying infrastructure provider for the load balancer.
	// Allowed values are "AWS", "Azure", and "GCP".
	Type LoadBalancerProviderType

	// AWS provides configuration settings that are specific to AWS
	// load balancers.
	//
	// If empty, defaults will be applied. See specific aws fields for
	// details about their defaults.
	AWS *AWSLoadBalancerParameters

	// Azure provides configuration settings that are specific to Azure
	// load balancers.
	//
	// If empty, defaults will be applied. See specific azure fields for
	// details about their defaults.
	Azure *AzureLoadBalancerParameters

	// GCP provides configuration settings that are specific to GCP
	// load balancers.
	//
	// If empty, defaults will be applied. See specific gcp fields for
	// details about their defaults.
	GCP *GCPLoadBalancerParameters
}

// LoadBalancerProviderType is the underlying infrastructure provider for the
// load balancer. Allowed values are "AWS", "Azure", and "GCP".
type LoadBalancerProviderType string

const (
	AWSLoadBalancerProvider   LoadBalancerProviderType = "AWS"
	AzureLoadBalancerProvider LoadBalancerProviderType = "Azure"
	GCPLoadBalancerProvider   LoadBalancerProviderType = "GCP"
)

// AWSLoadBalancerParameters provides configuration settings that are specific to
// AWS load balancers.
type AWSLoadBalancerParameters struct {
	// Type is the type of AWS load balancer to manage.
	//
	// Valid values are:
	//
	// * "Classic": A Classic load balancer makes routing decisions at either the
	//   transport layer (TCP/SSL) or the application layer (HTTP/HTTPS). See
	//   the following for additional details:
	//
	//     https://docs.aws.amazon.com/AmazonECS/latest/developerguide/load-balancer-types.html#clb
	//
	// * "NLB": A Network load balancer makes routing decisions at the transport
	//   layer (TCP/SSL). See the following for additional details:
	//
	//     https://docs.aws.amazon.com/AmazonECS/latest/developerguide/load-balancer-types.html#nlb
	//
	// If unset, defaults to "Classic".
	Type AWSLoadBalancerType

	// AllocationIDs is a list of Allocation IDs of Elastic IP addresses that are
	// to be assigned to the Network Load Balancer. Works only with type NLB.
	// If you are using Amazon EKS 1.16 or later, you can assign Elastic IP addresses
	// to Network Load Balancer with AllocationIDs. The number of Allocation IDs
	// must match the number of subnets used for the load balancer.
	//
	// Example: "eipalloc-<xxxxxxxxxxxxxxxxx>"
	//
	// See: https://docs.aws.amazon.com/eks/latest/userguide/load-balancing.html
	AllocationIDs []string
}

// AWSLoadBalancerType is the type of AWS load balancer to manage.
type AWSLoadBalancerType string

const (
	AWSClassicLoadBalancer AWSLoadBalancerType = "Classic"
	AWSNetworkLoadBalancer AWSLoadBalancerType = "NLB"
)

type AzureLoadBalancerParameters struct {
	// Address is the desired load balancer IP address. If scope is "Internal", address
	// must reside in same virtual network as AKS and must not already be assigned
	// to a resource. If address does not reside in same subnet as AKS, the subnet
	// parameter is also required.
	//
	// Address must already exist (e.g. `az network public-ip create`).
	//
	// See:
	// 	 https://docs.microsoft.com/en-us/azure/aks/static-ip#create-a-service-using-the-static-ip-address
	// 	 https://docs.microsoft.com/en-us/azure/aks/internal-lb#specify-an-ip-address
	Address *string

	// ResourceGroup is the resource group name where the "address" resides. Relevant
	// only if scope is "External".
	//
	// Omit if desired IP is created in same resource group as AKS cluster.
	ResourceGroup *string

	// Subnet is the subnet name where the "address" resides. Relevant only
	// if scope is "Internal" and desired IP does not reside in same subnet as AKS.
	//
	// Omit if desired IP is in same subnet as AKS cluster.
	//
	// See: https://docs.microsoft.com/en-us/azure/aks/internal-lb#specify-an-ip-address
	Subnet *string
}

type GCPLoadBalancerParameters struct {
	// Address is the desired load balancer IP address. If scope is "Internal", the address
	// must reside in same subnet as the GKE cluster or "subnet" has to be provided.
	//
	// See:
	// 	 https://cloud.google.com/kubernetes-engine/docs/tutorials/configuring-domain-name-static-ip#use_a_service
	// 	 https://cloud.google.com/kubernetes-engine/docs/how-to/internal-load-balancing#lb_subnet
	Address *string

	// Subnet is the subnet name where the "address" resides. Relevant only
	// if scope is "Internal" and desired IP does not reside in same subnet as GKE
	// cluster.
	//
	// Omit if desired IP is in same subnet as GKE cluster.
	//
	// See: https://cloud.google.com/kubernetes-engine/docs/how-to/internal-load-balancing#lb_subnet
	Subnet *string
}

type ServicePort struct {
	Name       string
	PortNumber int32
}

// NodePort is the schema to specify a network port for a NodePort Service.
type NodePort struct {
	// Name is an IANA_SVC_NAME within the NodePort Service.
	Name string

	// PortNumber is the network port number to expose for the NodePort Service.
	// If unspecified, a port number will be assigned from the the cluster's
	// nodeport service range, i.e. --service-node-port-range flag
	// (default: 30000-32767).
	//
	// If specified, the number must:
	//
	// 1. Not be used by another NodePort Service.
	// 2. Be within the cluster's nodeport service range, i.e. --service-node-port-range
	//    flag (default: 30000-32767).
	// 3. Be a valid network port number, i.e. greater than 0 and less than 65536.
	PortNumber *int32
}

// ContainerPort is the schema to specify a network port for a container.
// A container port gives the system additional information about network
// connections a container uses, but is primarily informational.
type ContainerPort struct {
	// Name is an IANA_SVC_NAME within the pod.
	Name string

	// PortNumber is the network port number to expose on the envoy pod.
	// The number must be greater than 0 and less than 65536.
	PortNumber int32
}

const (
	// ContourAvailableConditionType indicates that the contour is running
	// and available.
	ContourAvailableConditionType = "Available"
)

// OwningSelector returns a label selector using "projectcontour.io/owning-gateway-name".
func OwningSelector(contour *Contour) *metav1.LabelSelector {
	return &metav1.LabelSelector{
		MatchLabels: map[string]string{
			OwningGatewayNameLabel: contour.Name,
		},
	}
}

// OwnerLabels returns owner labels for the provided contour.
func OwnerLabels(contour *Contour) map[string]string {
	return map[string]string{
		OwningGatewayNameLabel: contour.Name,
	}
}

// MakeNodePorts returns a nodeport slice using the ports key as the nodeport name
// and the ports value as the nodeport number.
func MakeNodePorts(ports map[string]int) []NodePort {
	nodePorts := []NodePort{}
	for k, v := range ports {
		p := NodePort{
			Name:       k,
			PortNumber: pointer.Int32Ptr(int32(v)),
		}
		nodePorts = append(nodePorts, p)
	}
	return nodePorts
}
