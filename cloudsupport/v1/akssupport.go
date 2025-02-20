package v1

import (
	"context"
	"fmt"
	"os"

	// "github.com/Azure/azure-sdk-for-go/services/containerservice/mgmt/2019-04-30/containerservice"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	armauthorization "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/authorization/armauthorization/v2"
	armcontainerservice "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v2"
	"github.com/kubescape/k8s-interface/k8sinterface"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	AZURE_SUBSCRIPTION_ID_ENV_VAR = "AZURE_SUBSCRIPTION_ID"
	AZURE_RESOURCE_GROUP_ENV_VAR  = "AZURE_RESOURCE_GROUP"
)

type IAKSSupport interface {
	GetClusterDescribe(subscriptionId string, clusterName string, resourceGroup string) (*armcontainerservice.ManagedCluster, error)
	GetContextName(*armcontainerservice.ManagedCluster) string
	GetSubscriptionID() (string, error)
	GetResourceGroup() (string, error)
	ListAllRolesForScope(subscriptionId string, scope string) ([]*armauthorization.RoleAssignment, error)
	GetGroupIdsRoleBindings(kapi *k8sinterface.KubernetesApi, namespace string) ([]string, error)
}
type AKSSupport struct {
}

func NewAKSSupport() *AKSSupport {
	return &AKSSupport{}
}

// Get descriptive info about cluster running in AKS.
func (AKSSupport *AKSSupport) GetClusterDescribe(subscriptionId string, clusterName string, resourceGroup string) (*armcontainerservice.ManagedCluster, error) {

	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return nil, err
	}
	aksclient, err := armcontainerservice.NewManagedClustersClient(subscriptionId, cred, nil)
	if err != nil {
		return nil, err
	}

	ctx := context.Background()

	resp, err := aksclient.Get(ctx, resourceGroup, clusterName, nil)
	if err != nil {
		return nil, err
	}
	return &resp.ManagedCluster, nil

}

func (AKSSupport *AKSSupport) GetContextName(managedCluster *armcontainerservice.ManagedCluster) string {
	if managedCluster != nil {
		if managedCluster.Name != nil {
			return *managedCluster.Name
		}
	}
	return ""
}

func (AKSSupport *AKSSupport) GetSubscriptionID() (string, error) {
	if subscriptionId, ok := os.LookupEnv(AZURE_SUBSCRIPTION_ID_ENV_VAR); ok {
		return subscriptionId, nil
	}
	return "", fmt.Errorf("error retrieving azure subscription id: environment variable %s not set", AZURE_SUBSCRIPTION_ID_ENV_VAR)
}

func (AKSSupport *AKSSupport) GetResourceGroup() (string, error) {
	if subscriptionId, ok := os.LookupEnv(AZURE_RESOURCE_GROUP_ENV_VAR); ok {
		return subscriptionId, nil
	}
	return "", fmt.Errorf("error retrieving azure subscription id: environment variable %s not set", AZURE_RESOURCE_GROUP_ENV_VAR)
}

// List all role assignments that apply to a scope
// scope - The scope of the operation or resource. Valid scopes are:
// subscriptionID (format: '/subscriptions/{subscriptionId}'),
// resource group ID (format:'/subscriptions/{subscriptionId}/resourceGroups/{resourceGroupName}', or
// resource ID (format:'/subscriptions/{subscriptionId}/resourceGroups/{resourceGroupName}/providers/{resourceProviderNamespace}/[{parentResourcePath}/]{resourceType}/{resourceName}'
func (AKSSupport *AKSSupport) ListAllRolesForScope(subscriptionId string, scope string) ([]*armauthorization.RoleAssignment, error) {

	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return nil, err
	}
	ctx := context.Background()

	client, err := armauthorization.NewRoleAssignmentsClient(subscriptionId, cred, nil)
	if err != nil {
		return nil, err
	}

	pager := client.NewListForScopePager(scope, &armauthorization.RoleAssignmentsClientListForScopeOptions{Filter: nil,
		TenantID:  nil,
		SkipToken: nil,
	})

	var roleList []*armauthorization.RoleAssignment

	for pager.More() {
		nextResult, err := pager.NextPage(ctx)
		if err != nil {
			return roleList, fmt.Errorf("failed to advance page: %v", err)
		}

		roleList = append(roleList, nextResult.Value...)
	}

	return roleList, nil

}

// Rolebindings contains the group-object-ids
func (AKSSupport *AKSSupport) GetGroupIdsRoleBindings(kapi *k8sinterface.KubernetesApi, namespace string) ([]string, error) {

	listgroupids := make([]string, 0)

	if namespace == "" {

		// throughout the cluster access
		clusterrolebindings, err := kapi.KubernetesClient.RbacV1().ClusterRoleBindings().List(context.Background(), metav1.ListOptions{})

		if err != nil {
			return nil, fmt.Errorf("no clusterrolebindings are found inside the cluster")
		}
		for _, rolebinding := range clusterrolebindings.Items {
			for _, subjects := range rolebinding.Subjects {
				if subjects.Kind == "Group" {
					listgroupids = append(listgroupids, subjects.Name)
				}
			}
		}

	}

	// rolebindings inside a particular namespace
	rolebindings, err := kapi.KubernetesClient.RbacV1().RoleBindings(namespace).List(context.Background(), metav1.ListOptions{})

	if err != nil {
		return nil, fmt.Errorf("no rolebindings are found in the %s namespace ", namespace)
	}

	for _, rolebinding := range rolebindings.Items {
		for _, subjects := range rolebinding.Subjects {
			if subjects.Kind == "Group" {
				listgroupids = append(listgroupids, subjects.Name)
			}
		}
	}

	return listgroupids, nil

}
