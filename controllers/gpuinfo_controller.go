/*
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

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
//
// Description:		Controller Logic to slect some suitable K8s node for a particual role
//			In this case, simuation code has been added to replace some current non-existing
//			functionality
//
// Author:	   	Cormac J. Hogan (VMware)
//
// Date:		11 Feb 2021
//
////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

package controllers

import (
	"context"
	"fmt"
	"math/rand"
	"reflect"
	"time"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/view"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/mo"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/go-logr/logr"
	k8serr "k8s.io/apimachinery/pkg/api/errors"

	topologyv1 "gpuinfo/api/v1"

	// APis added
	corev1 "k8s.io/api/core/v1"
)

// GPUInfoReconciler reconciles a GPUInfo object
type GPUInfoReconciler struct {
	client.Client
	VC1    *vim25.Client
	VC2    *govmomi.Client
	Finder *find.Finder
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// CandidateList stores candidates suitable for long-running job
type CandidateList struct {
	hostName        string
	availAccTime    int
	hasGPU          bool
	nodeMemoryUsage int32
	nodeCpuUsage    int32
	nodeName        string
}

// +kubebuilder:rbac:groups=topology.corinternal.com,resources=gpuinfoes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=topology.corinternal.com,resources=gpuinfoes/status,verbs=get;update;patch

func (r *GPUInfoReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("gpuinfo", req.NamespacedName)

	gpu := &topologyv1.GPUInfo{}
	if err := r.Client.Get(ctx, req.NamespacedName, gpu); err != nil {
		if !k8serr.IsNotFound(err) {
			log.Error(err, "unable to fetch GPUInfo")
		}
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	//
	// Check if this update is due to the previous Reconcile Request, or
	// if it is a new Reconcile Request
	//

	if gpu.Status.SuitableNodeName == "" {
		msg := fmt.Sprintf("received reconcile request for %q (namespace: %q)", gpu.GetName(), gpu.GetNamespace())
		log.Info(msg)

		var candidate []CandidateList
		var bestCandidates []CandidateList
		var winnerCandidate CandidateList

		var nodes []*corev1.Node

		suitableCandidates := 0

		myNodeList := &corev1.NodeList{}

		//
		//  Part 1 - Retrieve the list of Kubernetes Nodes. We need this to find out the related VM / ESXi host capabilities
		//
		err := r.Client.List(context.TODO(), myNodeList)
		if err != nil {
			fmt.Println(fmt.Errorf("unable to retrieve nodes from node lister: %v", err))
			return ctrl.Result{}, client.IgnoreNotFound(err)
		}

		if len(myNodeList.Items) == 0 {
			log.Error(err, "unable to find any nodes")
			return ctrl.Result{}, err
		} else {
			msg := fmt.Sprintf("DEBUG: found %v nodes in cluster", len(myNodeList.Items))
			log.Info(msg)
		}

		for i := range myNodeList.Items {
			nodes = append(nodes, &myNodeList.Items[i])
		}

		//for _, node := range nodes {
		//	msg := fmt.Sprintf("DEBUG : Found a node %v", node.Name)
		//	log.Info(msg)
		//}

		//
		// Part 2 - Simulation Code for generating next maintenance slot, in hours
		//

		rand.Seed(time.Now().UnixNano())
		mmMin := 300
		mmMax := 400

		//
		// Part 3 - Start retrieving some useful vSphere information, once the K8s node is matched to a VM
		//

		//
		// Create a view manager
		//

		m := view.NewManager(r.VC1)

		//
		// Create a container view of VM objects
		//

		v, err := m.CreateContainerView(ctx, r.VC1.ServiceContent.RootFolder, []string{"VirtualMachine"}, true)
		if err != nil {
			log.Error(err, "Unable to create Virtual Machine Container View: error")
			return ctrl.Result{}, err
		}

		defer v.Destroy(ctx)

		//
		// Retrieve summary property for all virtual machines - descriptions of objects are available at the following links
		//
		// Ref: https://vdc-download.vmware.com/vmwb-repository/dcr-public/b50dcbbf-051d-4204-a3e7-e1b618c1e384/538cf2ec-b34f-4bae-a332-3820ef9e7773/vim.vm.Summary.html
		//

		var vms []mo.VirtualMachine
		err = v.Retrieve(ctx, []string{"VirtualMachine"}, []string{"summary"}, &vms)

		if err != nil {
			log.Error(err, "Unable to retrieve VM information: error")
			return ctrl.Result{}, err
		}

		//
		// Create a container view of HostSystem objects
		//

		h, err := m.CreateContainerView(ctx, r.VC1.ServiceContent.RootFolder, []string{"HostSystem"}, true)

		if err != nil {
			log.Error(err, "Unable to create Host Container View: error")
			return ctrl.Result{}, err
		}

		defer h.Destroy(ctx)

		//
		// Retrieve summary property for all ESXi hosts
		//
		// Ref: https://vdc-download.vmware.com/vmwb-repository/dcr-public/b50dcbbf-051d-4204-a3e7-e1b618c1e384/538cf2ec-b34f-4bae-a332-3820ef9e7773/vim.HostSystem.html
		//

		var hss []mo.HostSystem
		err = h.Retrieve(ctx, []string{"HostSystem"}, []string{"summary"}, &hss)

		if err != nil {
			log.Error(err, "Unable to retrieve Host information: error")
			return ctrl.Result{}, err
		}

		//
		// Retrieve summary property for VM which matches K8s node name
		//

		for i := 0; i < len(myNodeList.Items); i++ {

			for _, vm := range vms {

				if vm.Summary.Config.Name == myNodeList.Items[i].ObjectMeta.Name {

					//
					// Find ESXi Hypervisor/Host where VM/Node runs, and display relevant info  (currently just the name of the host)
					//

					for _, hs := range hss {
						if reflect.DeepEqual(hs.Summary.Host, vm.Summary.Runtime.Host) {
							candidate = append(candidate, CandidateList{
								hs.Summary.Config.Name,
								//
								// Simulation Code for generating next maintenance slot, in hours
								//
								rand.Intn(mmMax-mmMin+1) + mmMin,

								//
								// Simulation Code for randomly selecting if host has GPU or not - basically get True or False
								//
								(bool)(rand.Float32() < 0.5),

								//
								// Get some CPU and Memory usage stats from the node - we will use this to decide the best node in the case of multiple node candidate being available
								//
								vm.Summary.QuickStats.GuestMemoryUsage,
								vm.Summary.QuickStats.OverallCpuDemand,

								//
								// VM Name / K8s Node Name
								//
								vm.Summary.Config.Name})
						}
					}
				}

			}
		}

		//
		// More simulator code:
		//
		// First step is to just return suitable candidates for the long running job
		// This simply means that it matches both the Desired Accelerator Time and GPU
		// Requirement from the spec.
		//
		// Once the list of candidates is found, search through them for the winning candidate
		// We decided to use the node/virtual machine that had the least amount of CPU used
		//

		for _, entry := range candidate {

			if (int64(entry.availAccTime) >= gpu.Spec.DesAccTime) && (entry.hasGPU == gpu.Spec.GPURequired) {

				suitableCandidates++
				bestCandidates = append(bestCandidates, entry)
			}
		}

		//
		// At this point, bestCandidates has all available candidates that match the spec.
		// We now go through the bestCandidate and pick the winningCandidate, based on CPU usage
		//

		//
		// If there are no suitable candidates, send back a status that reports this
		//

		if suitableCandidates == 0 {
			msg := fmt.Sprintf("Found  *** NO *** suitable candidates for the long running job\n")
			log.Info(msg)

			gpu.Status.SuitableNodeName = "None available"
			gpu.Status.SuitableHostName = "None available"
			gpu.Status.NodeMemoryUsage = 0
			gpu.Status.NodeCPUUsage = 0
			gpu.Status.AvailableAcceleratorTime = 0
			//
			// OK - so there is at least one suitable candidate
			//
		} else if suitableCandidates == 1 {
			msg := fmt.Sprintf("Found a total of *** 1 *** suitable candidates for the long running job\n")
			log.Info(msg)

			for _, singleentry := range bestCandidates {

				gpu.Status.SuitableHostName = singleentry.hostName
				gpu.Status.SuitableNodeName = singleentry.nodeName
				gpu.Status.NodeCPUUsage = int64(singleentry.nodeCpuUsage)
				gpu.Status.NodeMemoryUsage = int64(singleentry.nodeMemoryUsage)
				gpu.Status.AvailableAcceleratorTime = int64(singleentry.availAccTime)
			}
			//
			// So there are multiple candidates (VMs/Nodes) -  this logic simply selects the node which has the least amount of CPU usage
			//

		} else if suitableCandidates > 1 {

			msg := fmt.Sprintf("Found a total of *** %v *** suitable candidates for the long running job\n", suitableCandidates)
			log.Info(msg)

			//
			// Initialize the array of the winning candidate
			//

			winnerCandidate.hostName = "Unknown"
			winnerCandidate.nodeName = "Unknown"
			winnerCandidate.nodeMemoryUsage = 0
			winnerCandidate.nodeCpuUsage = 999999
			winnerCandidate.availAccTime = 0
			//
			// Search the list of suitable candidates, and update the winning candidate if the
			// CPU usage is less that the current winning candidate
			//
			for _, newentry := range bestCandidates {

				if newentry.nodeCpuUsage < winnerCandidate.nodeCpuUsage {
					winnerCandidate.hostName = newentry.hostName
					winnerCandidate.nodeName = newentry.nodeName
					winnerCandidate.nodeMemoryUsage = newentry.nodeMemoryUsage
					winnerCandidate.nodeCpuUsage = newentry.nodeCpuUsage
					winnerCandidate.availAccTime = newentry.availAccTime
				} else {

					msg = fmt.Sprintf("DEBUG: multiple candidates : Node %s is not the winning candidate\n", newentry.nodeName)
					log.Info(msg)
				}
			}
			//
			// Winning candidate from all of the suitable candidates is identified, update the status
			//

			gpu.Status.SuitableNodeName = winnerCandidate.nodeName
			gpu.Status.SuitableHostName = winnerCandidate.hostName
			gpu.Status.NodeMemoryUsage = int64(winnerCandidate.nodeMemoryUsage)
			gpu.Status.NodeCPUUsage = int64(winnerCandidate.nodeCpuUsage)
			gpu.Status.AvailableAcceleratorTime = int64(winnerCandidate.availAccTime)

		} else {
			log.Error(err, "Problem with the number of candidates found")
			return ctrl.Result{}, err
		}

		//
		// Update the GPU Info status fields
		//

		if err := r.Status().Update(ctx, gpu); err != nil {
			log.Error(err, "unable to update GPUInfo status")
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *GPUInfoReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&topologyv1.GPUInfo{}).
		Complete(r)
}
