package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	"sigs.k8s.io/sig-storage-lib-external-provisioner/v10/controller"

	// Ensure this points to your local provisioner package
	"github.com/kubernetes-incubator/external-storage/iscsi/targetd/provisioner" 
)

var (
	provisionerName = flag.String("provisioner", "iscsi-targetd", "Name of the provisioner")
)

func main() {
	klog.InitFlags(nil)
	flag.Parse()

	config, err := rest.InClusterConfig()
	if err != nil {
		kubeconfig := os.Getenv("KUBECONFIG")
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			klog.Fatalf("Failed to create config: %v", err)
		}
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		klog.Fatalf("Failed to create clientset: %v", err)
	}

	logger := klog.Background()
	iscsiProvisioner := provisioner.NewISCSITargetdProvisioner(clientset)

	// Minimal v10 Constructor
	// We only use the functional options that are GUARANTEED to be defined.
	pc := controller.NewProvisionController(
		logger,
		clientset,
		*provisionerName,
		iscsiProvisioner,
		controller.LeaderElection(true),
		controller.LeaseDuration(15*time.Second),
		controller.RenewDeadline(10*time.Second),
		controller.RetryPeriod(2*time.Second),
	)

	ctx, cancel := context.WithCancel(context.Background())
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		klog.Info("Shutting down...")
		cancel()
	}()

	klog.Infof("Starting iSCSI provisioner controller %s", *provisionerName)
	pc.Run(ctx)
}
