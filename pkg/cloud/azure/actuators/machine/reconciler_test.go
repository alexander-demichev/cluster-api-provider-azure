package machine

import (
	"context"
	"strings"
	"testing"

	"github.com/Azure/azure-sdk-for-go/profiles/2019-03-01/compute/mgmt/compute"
	. "github.com/onsi/gomega"
	machinev1 "github.com/openshift/machine-api-operator/pkg/apis/machine/v1beta1"
	machinecontroller "github.com/openshift/machine-api-operator/pkg/controller/machine"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/apis/azureprovider/v1beta1"
	providerspecv1 "sigs.k8s.io/cluster-api-provider-azure/pkg/apis/azureprovider/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure/actuators"
)

func TestExists(t *testing.T) {
	testCases := []struct {
		vmService *FakeVMService
		expected  bool
	}{
		{
			vmService: &FakeVMService{
				Name:              "machine-test",
				ID:                "machine-test-ID",
				ProvisioningState: string(v1beta1.VMStateSucceeded),
			},
			expected: true,
		},
		{
			vmService: &FakeVMService{
				Name:              "machine-test",
				ID:                "machine-test-ID",
				ProvisioningState: string(v1beta1.VMStateUpdating),
			},
			expected: true,
		},
		{
			vmService: &FakeVMService{
				Name:              "machine-test",
				ID:                "machine-test-ID",
				ProvisioningState: string(v1beta1.VMStateCreating),
			},
			expected: true,
		},
		{
			vmService: &FakeVMService{
				Name:              "machine-test",
				ID:                "machine-test-ID",
				ProvisioningState: "",
			},
			expected: false,
		},
	}
	for _, tc := range testCases {
		r := newFakeReconciler(t)
		r.virtualMachinesSvc = tc.vmService
		exists, err := r.Exists(context.TODO())
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if exists != tc.expected {
			t.Fatalf("Expected: %v, got: %v", tc.expected, exists)
		}
	}
}

func TestSetMachineCloudProviderSpecifics(t *testing.T) {
	testStatus := "testState"
	testSize := compute.BasicA4
	testRegion := "testRegion"
	testZone := "testZone"
	testZones := []string{testZone}

	maxPrice := "1"
	r := Reconciler{
		scope: &actuators.MachineScope{
			Machine: &machinev1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "",
					Namespace: "",
				},
			},
			MachineConfig: &v1beta1.AzureMachineProviderSpec{
				SpotVMOptions: &v1beta1.SpotVMOptions{
					MaxPrice: &maxPrice,
				},
			},
		},
	}

	vm := compute.VirtualMachine{
		VirtualMachineProperties: &compute.VirtualMachineProperties{
			ProvisioningState: &testStatus,
			HardwareProfile: &compute.HardwareProfile{
				VMSize: testSize,
			},
		},
		Location: &testRegion,
		Zones:    &testZones,
	}

	r.setMachineCloudProviderSpecifics(vm)

	actualInstanceStateAnnotation := r.scope.Machine.Annotations[MachineInstanceStateAnnotationName]
	if actualInstanceStateAnnotation != testStatus {
		t.Errorf("Expected instance state annotation: %v, got: %v", actualInstanceStateAnnotation, vm.VirtualMachineProperties.ProvisioningState)
	}

	actualMachineTypeLabel := r.scope.Machine.Labels[MachineInstanceTypeLabelName]
	if actualMachineTypeLabel != string(vm.HardwareProfile.VMSize) {
		t.Errorf("Expected machine type label: %v, got: %v", actualMachineTypeLabel, string(vm.HardwareProfile.VMSize))
	}

	actualMachineRegionLabel := r.scope.Machine.Labels[machinecontroller.MachineRegionLabelName]
	if actualMachineRegionLabel != *vm.Location {
		t.Errorf("Expected machine region label: %v, got: %v", actualMachineRegionLabel, *vm.Location)
	}

	actualMachineAZLabel := r.scope.Machine.Labels[machinecontroller.MachineAZLabelName]
	if actualMachineAZLabel != strings.Join(*vm.Zones, ",") {
		t.Errorf("Expected machine zone label: %v, got: %v", actualMachineAZLabel, strings.Join(*vm.Zones, ","))
	}

	if _, ok := r.scope.Machine.Spec.Labels[machinecontroller.MachineInterruptibleInstanceLabelName]; !ok {
		t.Error("Missing spot instance label in machine spec")
	}

}

func TestSetMachineCloudProviderSpecificsTable(t *testing.T) {
	abcZones := []string{"a", "b", "c"}

	testCases := []struct {
		name                string
		scope               func(t *testing.T) *actuators.MachineScope
		vm                  compute.VirtualMachine
		expectedLabels      map[string]string
		expectedAnnotations map[string]string
		expectedSpecLabels  map[string]string
	}{
		{
			name:  "with a blank vm",
			scope: func(t *testing.T) *actuators.MachineScope { return newFakeScope(t, "worker") },
			vm:    compute.VirtualMachine{},
			expectedLabels: map[string]string{
				providerspecv1.MachineRoleLabel: "worker",
				machinev1.MachineClusterIDLabel: "clusterID",
			},
			expectedAnnotations: map[string]string{
				MachineInstanceStateAnnotationName: "",
			},
			expectedSpecLabels: nil,
		},
		{
			name:  "with a running vm",
			scope: func(t *testing.T) *actuators.MachineScope { return newFakeScope(t, "good-worker") },
			vm: compute.VirtualMachine{
				VirtualMachineProperties: &compute.VirtualMachineProperties{
					ProvisioningState: pointer.StringPtr("Running"),
				},
			},
			expectedLabels: map[string]string{
				providerspecv1.MachineRoleLabel: "good-worker",
				machinev1.MachineClusterIDLabel: "clusterID",
			},
			expectedAnnotations: map[string]string{
				MachineInstanceStateAnnotationName: "Running",
			},
			expectedSpecLabels: nil,
		},
		{
			name:  "with a VMSize set vm",
			scope: func(t *testing.T) *actuators.MachineScope { return newFakeScope(t, "sized-worker") },
			vm: compute.VirtualMachine{
				VirtualMachineProperties: &compute.VirtualMachineProperties{
					HardwareProfile: &compute.HardwareProfile{
						VMSize: "big",
					},
				},
			},
			expectedLabels: map[string]string{
				providerspecv1.MachineRoleLabel: "sized-worker",
				machinev1.MachineClusterIDLabel: "clusterID",
				MachineInstanceTypeLabelName:    "big",
			},
			expectedAnnotations: map[string]string{
				MachineInstanceStateAnnotationName: "",
			},
			expectedSpecLabels: nil,
		},
		{
			name:  "with a vm location",
			scope: func(t *testing.T) *actuators.MachineScope { return newFakeScope(t, "located-worker") },
			vm: compute.VirtualMachine{
				Location: pointer.StringPtr("nowhere"),
			},
			expectedLabels: map[string]string{
				providerspecv1.MachineRoleLabel: "located-worker",
				machinev1.MachineClusterIDLabel: "clusterID",
				MachineRegionLabelName:          "nowhere",
			},
			expectedAnnotations: map[string]string{
				MachineInstanceStateAnnotationName: "",
			},
			expectedSpecLabels: nil,
		},
		{
			name:  "with a vm with zones",
			scope: func(t *testing.T) *actuators.MachineScope { return newFakeScope(t, "zoned-worker") },
			vm: compute.VirtualMachine{
				Zones: &abcZones,
			},
			expectedLabels: map[string]string{
				providerspecv1.MachineRoleLabel: "zoned-worker",
				machinev1.MachineClusterIDLabel: "clusterID",
				MachineAZLabelName:              "a,b,c",
			},
			expectedAnnotations: map[string]string{
				MachineInstanceStateAnnotationName: "",
			},
			expectedSpecLabels: nil,
		},
		{
			name: "with a vm on spot",
			scope: func(t *testing.T) *actuators.MachineScope {
				scope := newFakeScope(t, "spot-worker")
				scope.MachineConfig.SpotVMOptions = &v1beta1.SpotVMOptions{}
				return scope
			},
			vm: compute.VirtualMachine{},
			expectedLabels: map[string]string{
				providerspecv1.MachineRoleLabel:                         "spot-worker",
				machinev1.MachineClusterIDLabel:                         "clusterID",
				machinecontroller.MachineInterruptibleInstanceLabelName: "",
			},
			expectedAnnotations: map[string]string{
				MachineInstanceStateAnnotationName: "",
			},
			expectedSpecLabels: map[string]string{
				machinecontroller.MachineInterruptibleInstanceLabelName: "",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			r := newFakeReconcilerWithScope(t, tc.scope(t))
			r.setMachineCloudProviderSpecifics(tc.vm)

			machine := r.scope.Machine
			g.Expect(machine.Labels).To(Equal(tc.expectedLabels))
			g.Expect(machine.Annotations).To(Equal(tc.expectedAnnotations))
			g.Expect(machine.Spec.Labels).To(Equal(tc.expectedSpecLabels))
		})
	}
}
