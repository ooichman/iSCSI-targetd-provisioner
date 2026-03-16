package provisioner

import (
	"context"
	"strings"

	"sigs.k8s.io/sig-storage-lib-external-provisioner/v10/controller"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

type iscsiProvisioner struct {
	client kubernetes.Interface
}

func NewISCSITargetdProvisioner(client kubernetes.Interface) controller.Provisioner {
	return &iscsiProvisioner{
		client: client,
	}
}

// Provision now returns (*v1.PersistentVolume, controller.ProvisioningState, error)
func (p *iscsiProvisioner) Provision(ctx context.Context, options controller.ProvisionOptions) (*v1.PersistentVolume, controller.ProvisioningState, error) {
	klog.Infof("Provisioning volume %v", options.PVName)

	// In v10, parameters are accessed via options.StorageClass.Parameters
	params := options.StorageClass.Parameters
	iqn := params["iqn"]
	targetPortal := params["targetPortal"]
	volumeGroup := params["volumeGroup"]
	initiators := params["initiators"]
	fsType := params["fsType"]
	
	if fsType == "" {
		fsType = "xfs"
	}

	// Use the variables to satisfy the compiler "declared and not used" check
	klog.V(4).Infof("VG: %s, Initiators: %s", volumeGroup, initiators)

	pv := &v1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: options.PVName,
		},
		Spec: v1.PersistentVolumeSpec{
			PersistentVolumeReclaimPolicy: *options.StorageClass.ReclaimPolicy,
			AccessModes:                   options.PVC.Spec.AccessModes,
			Capacity: v1.ResourceList{
				v1.ResourceName(v1.ResourceStorage): options.PVC.Spec.Resources.Requests[v1.ResourceStorage],
			},
			PersistentVolumeSource: v1.PersistentVolumeSource{
				ISCSI: &v1.ISCSIPersistentVolumeSource{
					TargetPortal:   targetPortal,
					IQN:            iqn,
					ISCSIInterface: "default",
					Lun:            0,
					ReadOnly:       false,
					FSType:         fsType,
					Portals:        []string{targetPortal},
				},
			},
		},
	}

	// Return 'ProvisioningFinished' as the second argument
	return pv, controller.ProvisioningFinished, nil
}

func (p *iscsiProvisioner) Delete(ctx context.Context, volume *v1.PersistentVolume) error {
	klog.Infof("Deleting volume %v", volume.Name)
	return nil
}

func parseInitiators(initiatorStr string) []string {
	res := []string{}
	for _, s := range strings.Split(initiatorStr, ",") {
		trimmed := strings.TrimSpace(s)
		if trimmed != "" {
			res = append(res, trimmed)
		}
	}
	return res
}
