package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"time"
)

type VolumeMetadata struct {
	IQN              string   `json:"iqn"`
	Portals          []string `json:"portals"`
	MultipathEnabled bool     `json:"multipath_enabled"`
	LUN              int      `json:"lun"`
}

func main() {
	controllerURL := os.Getenv("CONTROLLER_SERVICE_URL")
	pvcName := os.Getenv("PVC_NAME")

	if controllerURL == "" || pvcName == "" {
		log.Fatal("Env vars CONTROLLER_SERVICE_URL and PVC_NAME are required")
	}

	// 1. Fetch Metadata
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(fmt.Sprintf("%s/v1/volumes/%s", controllerURL, pvcName))
	if err != nil {
		log.Fatalf("Controller unreachable: %v", err)
	}
	defer resp.Body.Close()

	var meta VolumeMetadata
	json.NewDecoder(resp.Body).Decode(&meta)

	// 2. Logic: Multipath or Single Path
	portals := meta.Portals
	if !meta.MultipathEnabled {
		portals = []string{meta.Portals[0]}
	}

	// 3. iSCSI Login via Host Binaries
	for _, p := range portals {
		// Discover
		exec.Command("/usr/sbin/iscsiadm", "-m", "discovery", "-t", "sendtargets", "-p", p).Run()
		// Login
		exec.Command("/usr/sbin/iscsiadm", "-m", "node", "-T", meta.IQN, "-p", p, "--login").Run()
	}

	// 4. Rescan & Multipath
	exec.Command("/usr/sbin/iscsiadm", "-m", "node", "-T", meta.IQN, "--rescan").Run()
	if meta.MultipathEnabled {
		log.Println("Multipath enabled: refreshing host maps...")
		exec.Command("/usr/sbin/multipath", "-r").Run()
	}

	log.Println("iSCSI Node setup complete. Standing by.")
	select {}
}
