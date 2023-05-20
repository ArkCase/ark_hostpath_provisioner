/*
Copyright 2018 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"context"
	"errors"
	"flag"
	"os"
	"path"
	"syscall"

	"sigs.k8s.io/sig-storage-lib-external-provisioner/v7/controller"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	klog "k8s.io/klog/v2"
)

// Fetch provisioner name from environment variable HOSTPATH_PROVISIONER_NAME
// if not set uses default hostpath name
func GetProvisionerName() string {
	provisionerName := os.Getenv("HOSTPATH_PROVISIONER_NAME")
	if provisionerName == "" {
		provisionerName = "hostpath"
	}
	return provisionerName
}


type hostPathProvisioner struct {
	// The directory to create PV-backing directories in
	pvDir string

	// Identity of this hostPathProvisioner, set to node's name. Used to identify
	// "this" provisioner's PVs.
	identity string

	// The annotation name to look for within PVCs when a specific location is
	// desired within the path tree
	hostPathAnnotation string
}

// NewHostPathProvisioner creates a new hostpath provisioner
func NewHostPathProvisioner() controller.Provisioner {
	nodeName := os.Getenv("NODE_NAME")
	if nodeName == "" {
		klog.Fatal("env variable NODE_NAME must be set so that this provisioner can identify itself")
	}
	nodeHostPath := os.Getenv("NODE_HOST_PATH")
	if nodeHostPath == "" {
		nodeHostPath = "/mnt/hostpath"
	}
	hostPathAnnotation := os.Getenv("NODE_HOST_PATH_ANNOTATION")
	if nodeHostPathAnnotation == "" {
		nodeHostPathAnnotation = "hostPath"
	}
	return &hostPathProvisioner{
		pvDir:    nodeHostPath,
		identity: nodeName,
		hostPathAnnotation: hostPathAnnotation,
	}
}

var _ controller.Provisioner = &hostPathProvisioner{}

// Provision creates a storage asset and returns a PV object representing it.
func (p *hostPathProvisioner) Provision(ctx context.Context, options controller.ProvisionOptions) (*v1.PersistentVolume, controller.ProvisioningState, error) {
	hostPath := options.PVName

	// Allow the use of an annotation to request a specific location within the
	// directory hierarchy.
	ann, ok := options.PVC.Annotations[p.hostPathAnnotation]
	if ok {
		hostPath = ann
	}
	path := path.Join(p.pvDir, hostPath)

	if err := os.MkdirAll(path, 0777); err != nil {
		return nil, controller.ProvisioningFinished, err
	}

	pv := &v1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: options.PVName,
			Annotations: map[string]string{
				"hostPathProvisionerIdentity": p.identity,
				"hostPathProvisionerPath": path,
			},
		},
		Spec: v1.PersistentVolumeSpec{
			PersistentVolumeReclaimPolicy: *options.StorageClass.ReclaimPolicy,
			AccessModes:                   options.PVC.Spec.AccessModes,
			Capacity: v1.ResourceList{
				v1.ResourceName(v1.ResourceStorage): options.PVC.Spec.Resources.Requests[v1.ResourceName(v1.ResourceStorage)],
			},
			PersistentVolumeSource: v1.PersistentVolumeSource{
				HostPath: &v1.HostPathVolumeSource{
					Path: path,
				},
			},
		},
	}

	return pv, controller.ProvisioningFinished, nil
}

// Delete removes the storage asset that was created by Provision represented
// by the given PV.
func (p *hostPathProvisioner) Delete(ctx context.Context, volume *v1.PersistentVolume) error {
	ann, ok := volume.Annotations["hostPathProvisionerIdentity"]
	if !ok {
		return errors.New("identity annotation not found on PV")
	}
	if ann != p.identity {
		return &controller.IgnoredError{Reason: "identity annotation on PV does not match ours"}
	}

	// This annotation is used to store the path where the volume was created
	path, ok := volume.Annotations["hostPathProvisionerPath"]
	if !ok {
		// If the annotation isn't there, this may be a legacy volume so we use
		// the default method for computing its location
		path := path.Join(p.pvDir, volume.Name)
	}

	if err := os.RemoveAll(path); err != nil {
		return err
	}

	return nil
}

func main() {
	syscall.Umask(0)

	flag.Parse()
	flag.Set("logtostderr", "true")

	// Create an InClusterConfig and use it to create a client for the controller
	// to use to communicate with Kubernetes
	config, err := rest.InClusterConfig()
	if err != nil {
		klog.Fatalf("Failed to create config: %v", err)
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		klog.Fatalf("Failed to create client: %v", err)
	}

	// Create the provisioner: it implements the Provisioner interface expected by
	// the controller
	hostPathProvisioner := NewHostPathProvisioner()

	// Start the provision controller which will dynamically provision hostPath
	// PVs
	pc := controller.NewProvisionController(clientset, GetProvisionerName(), hostPathProvisioner)

	// Never stops.
	pc.Run(context.Background())
}
