package provisioner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	"sigs.k8s.io/sig-storage-lib-external-provisioner/v10/controller"
)

type iscsiProvisioner struct {
	client kubernetes.Interface
}

func NewISCSITargetdProvisioner(client kubernetes.Interface) controller.Provisioner {
	return &iscsiProvisioner{client: client}
}

func (p *iscsiProvisioner) Provision(ctx context.Context, options controller.ProvisionOptions) (*v1.PersistentVolume, controller.ProvisioningState, error) {
	klog.Infof("Provisioning volume %v", options.PVName)

	params := options.StorageClass.Parameters
	volSize := options.PVC.Spec.Resources.Requests[v1.ResourceStorage]

	// 1. Dynamic Node IQN Lookup
	var initiatorIQNs []string
	if options.SelectedNode != nil {
		node, err := p.client.CoreV1().Nodes().Get(ctx, options.SelectedNode.Name, metav1.GetOptions{})
		if err == nil {
			if iqn, ok := node.Annotations["storage.openshift.io/iscsi-initiator"]; ok {
				initiatorIQNs = append(initiatorIQNs, iqn)
			}
		}
	}

	// Fallback to static initiators in StorageClass
	if len(initiatorIQNs) == 0 && params["initiators"] != "" {
		initiatorIQNs = parseInitiators(params["initiators"])
	}

	if len(initiatorIQNs) == 0 {
		return nil, controller.ProvisioningFinished, fmt.Errorf("no initiator IQN found")
	}

	// 2. targetd API Calls (Create + Export)
	targetdURL := fmt.Sprintf("http://%s/targetd/rpc", os.Getenv("TARGETD_ADDRESS"))
	
	// Create LVM Volume
	if err := p.callTargetd(targetdURL, "vol_create", map[string]interface{}{
		"pool": params["volumeGroup"],
		"name": options.PVName,
		"size": volSize.Value(),
	}); err != nil {
		return nil, controller.ProvisioningFinished, err
	}

	// Export LUN to Node
	if err := p.callTargetd(targetdURL, "export_create", map[string]interface{}{
		"pool":          params["volumeGroup"],
		"name":          options.PVName,
		"initiator_wwn": initiatorIQNs[0],
		"lun":           0,
	}); err != nil {
		return nil, controller.ProvisioningFinished, err
	}

	// 3. Construct PV
	pv := &v1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{Name: options.PVName},
		Spec: v1.PersistentVolumeSpec{
			PersistentVolumeReclaimPolicy: *options.StorageClass.ReclaimPolicy,
			AccessModes:                   options.PVC.Spec.AccessModes,
			Capacity:                      v1.ResourceList{v1.ResourceStorage: volSize},
			PersistentVolumeSource: v1.PersistentVolumeSource{
				ISCSI: &v1.ISCSIPersistentVolumeSource{
					TargetPortal:   params["targetPortal"],
					IQN:            params["iqn"],
					ISCSIInterface: params["iscsiInterface"],
					Lun:            0,
					ReadOnly:       false,
					FSType:         params["fsType"],
				},
			},
		},
	}

	return pv, controller.ProvisioningFinished, nil
}

func (p *iscsiProvisioner) Delete(ctx context.Context, volume *v1.PersistentVolume) error {
	klog.Infof("Deleting volume %v", volume.Name)
	targetdURL := fmt.Sprintf("http://%s:18657/targetd/rpc", os.Getenv("TARGETD_ADDRESS"))
	
	p.callTargetd(targetdURL, "export_destroy", map[string]interface{}{
		"pool": "vg-targetd",
		"name": volume.Name,
	})
	return p.callTargetd(targetdURL, "vol_destroy", map[string]interface{}{
		"pool": "vg-targetd",
		"name": volume.Name,
	})
}

func (p *iscsiProvisioner) callTargetd(url, method string, params map[string]interface{}) error {
	payload := map[string]interface{}{"jsonrpc": "2.0", "method": method, "params": params, "id": 1}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(body))
	req.SetBasicAuth(os.Getenv("TARGETD_USERNAME"), os.Getenv("TARGETD_PASSWORD"))
	req.Header.Set("Content-Type", "application/json")

	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("targetd error: %s", resp.Status)
	}
	return nil
}

func parseInitiators(initiatorStr string) []string {
	res := []string{}
	for _, s := range strings.Split(initiatorStr, ",") {
		if trimmed := strings.TrimSpace(s); trimmed != "" {
			res = append(res, trimmed)
		}
	}
	return res
}
