package virtualmachines

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/Azure/azure-sdk-for-go/profiles/2019-03-01/network/mgmt/network"
	"github.com/Azure/go-autorest/autorest/to"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/apis/azureprovider/v1beta1"
)

func TestGetTagListFromSpec(t *testing.T) {
	testCases := []struct {
		spec     *Spec
		expected map[string]*string
	}{
		{
			spec: &Spec{
				Name: "test",
				Tags: map[string]string{
					"foo": "bar",
				},
			},
			expected: map[string]*string{
				"foo": to.StringPtr("bar"),
			},
		},
		{
			spec: &Spec{
				Name: "test",
			},
			expected: nil,
		},
	}

	for _, tc := range testCases {
		tagList := getTagListFromSpec(tc.spec)
		if !reflect.DeepEqual(tagList, tc.expected) {
			t.Errorf("Expected %v, got: %v", tc.expected, tagList)
		}
	}
}

func getTestNic(vmSpec *Spec, subscription, resourcegroup, location string) network.Interface {
	return network.Interface{
		Etag:     to.StringPtr("foobar"),
		ID:       to.StringPtr(fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/networkInterfaces/%s", subscription, resourcegroup, vmSpec.NICName)),
		Name:     &vmSpec.NICName,
		Type:     to.StringPtr("awesome"),
		Location: to.StringPtr("location"),
		Tags:     map[string]*string{},
	}
}

func getTestVMSpec(updateSpec func(*Spec)) *Spec {
	spec := &Spec{
		Name:       "my-awesome-machine",
		NICName:    "gxqb-master-nic",
		SSHKeyData: "",
		Size:       "Standard_D4s_v3",
		Zone:       "",
		Image: v1beta1.Image{
			Publisher: "Red Hat Inc",
			Offer:     "ubi",
			SKU:       "ubi7",
			Version:   "latest",
		},
		OSDisk: v1beta1.OSDisk{
			OSType:     "Linux",
			DiskSizeGB: 256,
		},
		CustomData:      "",
		ManagedIdentity: "",
		Tags:            map[string]string{},
		SecurityProfile: nil,
	}

	if updateSpec != nil {
		updateSpec(spec)
	}

	return spec
}
