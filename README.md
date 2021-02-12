# A simple Kubernetes Operator to return the most suitable Kubernetes Node / Virtual Machine for a long running ML job (prototype) #

This repository contains a Kubernetes Operator that uses VMware's __govmomi__ to identify the best Kubernetes node (Virtual Machine) for running a long running machine learning job. This is simply a prototype, and includes some simulation to address non-existing fucntionality at present.
A manifest is created with a number of specification field, and the most suitable Kubernetes node is returned in a number of the status fields of a __Custom Resource (CR)__. This tutorial will require us to extend Kubernetes with a new __Custom Resource Definition (CRD)__. The code shown here is for education purposes only, showing one way in which a Kubernetes controller / operator can access the underlying vSphere infrastructure for the purposes of querying resources.

This is the fourth operator in this education series. The previous operators ```HostInfo```, ```VMInfo```, and ```FCDInfo``` returned information related to ESXi hosts, vSphere Virtual Machines and First Class Disks. This operator is focusing on getting further vSphere information by taking a set of directives from the CR manifest, and depending on those directives, will return the best virtual machine / Kubernetes node for a long running machine learning job. The manifest contains information such as the length the job must run, and if the job requires a GPU or not. The controller code simulates availability of the host and whether or not it has a GPU using some very simple code.

You can think of a CRD as representing the desired state of a Kubernetes object or Custom Resource, and the function of the operator is to run the logic or code to make that desired state happen - in other words the operator has the logic to do whatever is necessary to achieve the object's desired state.

## What are we going to do in this tutorial? ##

In this example, we will create a CRD called ```GPUInfo```. GPUInfo will contain the length of time needed for the job, and if the job requires a GPU in its specification. When a Custom Resource (CR) is created and subsequently queried, we will call an operator (logic in a controller) whereby the most suitable Kubernetes node will be returned via the status fields of the object through govmomi API calls.

The following will be created as part of this tutorial:

* A __Customer Resource Definition (CRD)__
  * Group: ```Topology```
    * Kind: ```GPUInfo```
    * Version: ```v1```
    * Specification will include a two item: ```Spec.desAccTime``` and ```Spec.gpuRequired```.

* One or more __GPUInfo Custom Resource / Object__ will be created through yaml manifests, each manifest containing the hostname of an ESXi host that we wish to query. The fields which will be updated to contain the relevant information about the most suitable Kubernetes node (when the CR is queried) are:
  * ```Status.availAcceleratorTime```
  * ```Status.suitableHostName```
  * ```Status.suitableNodeName```
  * ```Status.nodeCPUUsage```
  * ```Status.nodeMemoryUsage```

* An __Operator__ (or business logic) to retrieve the node will be coded into the controller for this CR.

__Note:__ As mentioned, there is a similar tutorial to create an operator to get both ESXi host information and virtual machine information. These can be found [here](https://github.com/cormachogan/hostinfo-operator), [here](https://github.com/cormachogan/vminfo-operator) and again [here](https://github.com/cormachogan/fcdinfo-operator).

## What is not covered in this tutorial? ##

The assumption is that you already have a working Kubernetes cluster. Installation and deployment of a Kubernetes is outside the scope of this tutorial. If you do not have a Kubernetes cluster available, consider using __Kubernetes in Docker__ (shortened to __Kind__) which uses containers as Kubernetes nodes. A quickstart guide can be found here:

* [Kind (Kubernetes in Docker)](https://kind.sigs.K8s.io/docs/user/quick-start/)

The assumption is that you also have a __VMware vSphere environment__ comprising of at least one ESXi hypervisor which is managed by a vCenter server. For this operator to work, your Kubernetes cluster must be running on vSphere infrastructure, and thus this operator will help you examine how the underlying vSphere resources are being consumed by the Kubernetes clusters running on top.

## What if I just want to understand some basic CRD concepts? ##

If this sounds even too daunting at this stage, I strongly recommend checking out the excellent tutorial on CRDs from my colleague, __Rafael Brito__. His [RockBand](https://github.com/brito-rafa/k8s-webhooks/blob/master/single-gvk/README.md) CRD tutorial uses some very simple concepts to explain how CRDs, CRs, Operators, spec and status fields work, and is a great place to get started on your operator journey.

## Step 1 - Software Requirements ##

You will need the following components pre-installed on your desktop or workstation before we can build the CRD and operator.

* A __git__ client/command line
* [Go (v1.15+)](https://golang.org/dl/) - earlier versions may work but I used v1.15.
* [Docker Desktop](https://www.docker.com/products/docker-desktop)
* [Kubebuilder](https://go.kubebuilder.io/quick-start.html)
* [Kustomize](https://kubernetes-sigs.github.io/kustomize/installation/)
* Access to a Container Image Repositor (docker.io, quay.io, harbor)
* A __make__ binary - used by Kubebuilder

If you are interested in learning more about Golang basics, I found [this site](https://tour.golang.org/welcome/1) very helpful.

## Step 2 - KubeBuilder Scaffolding ##

The CRD is built using [kubebuilder](https://go.kubebuilder.io/).  I'm not going to spend a great deal of time talking about __KubeBuilder__. Suffice to say that KubeBuilder builds a directory structure containing all of the templates (or scaffolding) necessary for the creation of CRDs. Once this scaffolding is in place, this turorial will show you how to add your own specification fields and status fields, as well as how to add your own operator logic. In this example, our logic will login to vSphere, query and return Kubernetes node / virtual machine information  via a Kubernetes CR / object / Kind called GPUInfo, the values of which will be used to populate status fields in our CRs.

The following steps will create the scaffolding to get started.

```cmd
mkdir accelerator-operator
$ cd accelerator-operator
```

Next, define the Go module name of your CRD. In my case, I have called it __accelerator-operator__. This creates a __go.mod__ file with the name of the module and the Go version (v1.15 here).

```cmd
$ go mod init gpuinfo
go: creating new go.mod: module gpuinfo
```

```cmd
$ ls
go.mod
```

```cmd
$ cat go.mod
module gpuinfo

go 1.15
```

Now we can proceed with building out the rest of the directory structure. The following __kubebuilder__ commands (__init__ and __create api__) creates all the scaffolding necessary to build our CRD and operator. You may choose an alternate __domain__ here if you wish. Simply make note of it as you will be referring to it later in the tutorial.

```cmd
kubebuilder init --domain corinternal.com
```

Here is what the output from the command looks like:

```cmd
$ kubebuilder init --domain corinternal.com
Writing scaffold for you to edit...
Get controller runtime:
$ go get sigs.k8s.io/controller-runtime@v0.5.0
Update go.mod:
$ go mod tidy
Running make:
$ make
go: creating new go.mod: module tmp
go: found sigs.k8s.io/controller-tools/cmd/controller-gen in sigs.k8s.io/controller-tools v0.2.5
/usr/share/go/bin/controller-gen object:headerFile="hack/boilerplate.go.txt" paths="./..."
go fmt ./...
go vet ./...
go build -o bin/manager main.go
Next: define a resource with:
$ kubebuilder create api
$
```

As the output from the previous command states, we must now define a resource. To do that, we again use kubebuilder to create the resource, specifying the API group, its version and supported kind. My group is called topology, my kind is called ```GPUInfo``` and my initial version is v1.

```cmd
kubebuilder create api \
--group topology       \
--version v1           \
--kind GPUInfo         \
--resource=true        \
--controller=true
```

Here is the output from that command. Note that it is building the __types.go__ and __controller.go__, both of which we will be editing shortly:

```cmd
$ kubebuilder create api --group topology --version v1 --kind GPUInfo --resource=true --controller=true
Writing scaffold for you to edit...
api/v1/gpuinfo_types.go
controllers/gpuinfo_controller.go
Running make:
$ make
go: creating new go.mod: module tmp
go: found sigs.k8s.io/controller-tools/cmd/controller-gen in sigs.k8s.io/controller-tools v0.2.5
/usr/share/go/bin/controller-gen object:headerFile="hack/boilerplate.go.txt" paths="./..."
go fmt ./...
go vet ./...
go build -o bin/manager main.go
```

Our operator scaffolding (directory structure) is now in place. The next step is to define the specification and status fields in our CRD. After that, we create the controller logic which will watch our Custom Resources, and bring them to desired state (called a reconcile operation). More on this shortly.

## Step 3 - Create the CRD ##

Customer Resource Definitions [CRD](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/) are a way to extend Kubernetes through Custom Resources. We are going to extend a Kubernetes cluster with a new custom resource called __GPUInfo__ which will retrieve information from a PV whose name is specified in a Custom Resource. Thus, I will need to create fields called __DesAccTime__ and __GPURequired__ in the CRD - this defines the specification of the custom resource. We also add five status fields, as these will be used to return information about the most suitabel node to run the long running job.

This is done by modifying the __api/v1/gpuinfo_types.go__ file. Here is the initial scaffolding / template provided by kubebuilder:

```go
// GPUInfoSpec defines the desired state of GPUInfo
type GPUInfoSpec struct {
        // INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
        // Important: Run "make" to regenerate code after modifying this file

        // Foo is an example field of GPUInfo. Edit GPUInfo_types.go to remove/update
        Foo string `json:"foo,omitempty"`
}

// GPUInfoStatus defines the observed state of GPUInfo
type GPUInfoStatus struct {
        // INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
        // Important: Run "make" to regenerate code after modifying this file
}

// +kubebuilder:object:root=true
```

This file is modified to include a the __spec.DesAccTime__ and __spec.GPURequired__ fields. We will then return five __status__ fields. There are also a number of kubebuilder fields added, which are used to do validation and other kubebuilder related functions. The shortname "__gpu__" will be used later on in our controller logic. This can also be used with kubectl, e.g ```kubectl get gpu``` rather than```kubectl get gpuinfo```. Also, when we query any Custom Resources created with the CRD, e.g. ```kubectl get gpuinfo```, we want the output to display the Desired Access Time and GPU Required fields.

Note that what we are doing here is for education purposes only. Typically what you would observe is that the spec and status fields would be similar, and it is the function of the controller to reconcile and differences between the two to achieve eventual consistency. But we are keeping things simple, as the purpose here is to show how vSphere can be queried from a Kubernetes Operator. Below is a snippet of the __gpuinfo_types.go__ showing the code changes. It does not include the __imports__ which also need to be added.  The code-complete [gpuinfo_types.go](api/v1/gpuinfo_types.go) is here.

```go
// GPUInfoSpec defines the desired state of GPUInfo
type GPUInfoSpec struct {
	DesAccTime  int64 `json:"desAccTime"`
	GPURequired bool  `json:"gpuRequired"`
}

// GPUInfoStatus defines the observed state of GPUInfo
type GPUInfoStatus struct {
	SuitableNodeName         string `json:"suitableNodeName"`
	SuitableHostName         string `json:"suitableHostName"`
	NodeMemoryUsage          int64  `json:"nodeMemoryUsage"`
	NodeCPUUsage             int64  `json:"nodeCPUUsage"`
	AvailableAcceleratorTime int64  `json:"availableAcceleratorTime"`
}
```

We are now ready to create the CRD. There is one final step however, and this involves updating the __Makefile__ which kubebuilder has created for us. In the default Makefile created by kubebuilder, the following __CRD_OPTIONS__ line appears:

```Makefile
# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:trivialVersions=true"
```

This CRD_OPTIONS entry should be changed to the following:

```Makefile
# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:preserveUnknownFields=false,crdVersions=v1,trivialVersions=true"
```

Now we can build our CRD with the spec and status fields that we have place in the __api/v1/gpuinfo_types.go__ file.

```cmd
make manifests && make generate
```

Here is the output from the make:

```Makefile
$ make manifests && make generate
go: creating new go.mod: module tmp
go: found sigs.k8s.io/controller-tools/cmd/controller-gen in sigs.k8s.io/controller-tools v0.2.5
/usr/share/go/bin/controller-gen "crd:trivialVersions=true" rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=config/crd/bases
go: creating new go.mod: module tmp
go: found sigs.k8s.io/controller-tools/cmd/controller-gen in sigs.k8s.io/controller-tools v0.2.5
/usr/share/go/bin/controller-gen object:headerFile="hack/boilerplate.go.txt" paths="./..."
```

## Step 4 - Install the CRD ##

The CRD is not currently installed in the Kubernetes Cluster.

```shell
$ kubectl get crd
NAME                                                               CREATED AT
antreaagentinfos.clusterinformation.antrea.tanzu.vmware.com        2020-11-18T17:14:03Z
antreacontrollerinfos.clusterinformation.antrea.tanzu.vmware.com   2020-11-18T17:14:03Z
clusternetworkpolicies.security.antrea.tanzu.vmware.com            2020-11-18T17:14:03Z
traceflows.ops.antrea.tanzu.vmware.com                             2020-11-18T17:14:03Z
```

To install the CRD, run the following make command:

```cmd
make install
```

The output should look something like this:

```makefile
$ make install
go: creating new go.mod: module tmp
go: found sigs.k8s.io/controller-tools/cmd/controller-gen in sigs.k8s.io/controller-tools v0.2.5
/usr/share/go/bin/controller-gen "crd:trivialVersions=true" rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=config/crd/bases
kustomize build config/crd | kubectl apply -f -
customresourcedefinition.apiextensions.k8s.io/gpuinfoes.topology.corinternal.com created
```

Now check to see if the CRD is installed running the same command as before.

```shell
$ kubectl get crd
NAME                                                               CREATED AT
antreaagentinfos.clusterinformation.antrea.tanzu.vmware.com        2021-02-08T13:54:42Z
antreacontrollerinfos.clusterinformation.antrea.tanzu.vmware.com   2021-02-08T13:54:42Z
clusternetworkpolicies.security.antrea.tanzu.vmware.com            2021-02-08T13:54:42Z
gpuinfoes.topology.corinternal.com                                 2021-02-09T13:07:19Z
traceflows.ops.antrea.tanzu.vmware.com                             2021-02-08T13:54:42Z
```

Our new CRD ```gpuinfoes.topology.corinternal.com``` is now visible. Another useful way to check if the CRD has successfully deployed is to use the following command against our API group. Remember back in step 2 we specified the domain as ```corinternal.com``` and the group as ```topology```. Thus the command to query api-resources for this CRD is as follows:

```shell
$ kubectl api-resources --api-group=topology.corinternal.com
NAME         SHORTNAMES   APIGROUP                   NAMESPACED   KIND
gpuinfoes    gpu           topology.corinternal.com   true        GPUInfo
```

## Step 5 - Test the CRD ##

At this point, we can do a quick test to see if our CRD is in fact working. To do that, we can create a manifest file with a Custom Resource that uses our CRD, and see if we can instantiate such an object (or custom resource) on our Kubernetes cluster. Fortunately kubebuilder provides us with a sample manifest that we can use for this. It can be found in __config/samples__.

```shell
$ cd config/samples
$ ls
topology_v1_gpuinfo.yaml
```

```yaml
$ cat topology_v1_gpuinfo.yaml
apiVersion: topology.corinternal.com/v1
kind: GPUInfo
metadata:
  name: gpuinfo-sample
spec:
  # Add fields here
  foo: bar
```

We need to slightly modify this sample manifest so that the specification field matches what we added to our CRD. Note the spec: above where it states 'Add fields here'. We have removed the __foo__ field and added a __spec.desAcctime__ and __spec.gpuRequired__ fields, as per the __api/v1/gpuinfo_types.go__ modification earlier. Thus, after a simple modification, the CR manifest looks like this.

```yaml
$ cat topology_v1_gpuinfo.yaml
apiVersion: topology.corinternal.com/v1
kind: GPUInfo
metadata:
  name: gpuinfo-sample
spec:
  # Add fields here
  desAccTime: 100
  gpuRequired: True
```

To see if it works, we need to create this GPUInfo Custom Resource.

```shell
$ kubectl create -f topology_v1_gpuinfo.yaml
gpuinfo.topology.corinternal.com/gpuinfo-sample created
```

```shell
$ kubectl get gpuinfo
NAME             DESIRED ACCESS TIME (HRS)   GPU REQUIRED
gpuinfo-sample   100                         true
```

Or use the shortcut, "gpu":

```shell
$ kubectl get gpu
NAME             DESIRED ACCESS TIME (HRS)   GPU REQUIRED
gpuinfo-sample   100                         true
```

Note that the Desired Access Time and GPU Requied fields are also printed, as per the kubebuilder directive that we placed in the __api/v1/gpuinfo_types.go__. As a final test, we will display the CR in yaml format.

```yaml
$ kubectl get gpu gpuinfo-sample -o yaml

apiVersion: topology.corinternal.com/v1
kind: GPUInfo
metadata:
  creationTimestamp: "2021-02-11T15:17:20Z"
  generation: 1
  managedFields:
  - apiVersion: topology.corinternal.com/v1
    fieldsType: FieldsV1
    fieldsV1:
      f:spec:
        .: {}
        f:desAccTime: {}
        f:gpuRequired: {}
    manager: kubectl
    operation: Update
    time: "2021-02-11T15:17:20Z"
  name: gpuinfo-sample
  namespace: default
  resourceVersion: "1129225"
  selfLink: /apis/topology.corinternal.com/v1/namespaces/default/gpuinfoes/gpu1
  uid: 740ef10b-9723-49cd-9409-14cd52a2cb4f
spec:
  desAccTime: 100
  gpuRequired: true
kind: List
metadata:
  resourceVersion: ""
  selfLink: ""
```

## Step 6 - Create the controller / manager ##

This appears to be working as expected. However there are no __Status__ fields displayed with our PV information in the __yaml__ output above. To see this information, we need to implement our operator / controller logic to do this. The controller implements the desired business logic. In this controller, we first read the vCenter server credentials from a Kubernetes secret (which we will create shortly). We will then open a session to my vCenter server, and get a list of virtual machines. We will then match the Kubernetes nodes to virtual machines. If it turns out that more than one Kubernetes node is suitable for running our long running job, we will pick the one with the least amount of CPU usage. Finally we will update the appropriate Status field with information about the most suitable Kubernetes node, and we should be able to query it using the __kubectl get gpu -o yaml__ command seen previously.

### Step 6.1 - Open a session to vSphere ###

__Note:__ Let's first look at the login function which resides in __main.go__. This __vlogin__ function creates the vSphere session in main.go. One thing to note is that I am enabling insecure logins (true) by default. This is something that you may wish to change in your code. One other item to note is that I am testing two different client logins here, __govmomi.Client__ and __vim25.Client__. The ```govmomi.Client``` uses __Finder__ for getting vSphere information, and treats the vSphere inventory as a virtual filesystem. The ```vim25.Client``` uses __ContainerView__, and tends to generate more response data. As mentioned, this is a tutorial, so this operator shows both login types simply for informational purposes. Most likely, you could achieve the same results using a single login client.

```go
//
// - vSphere session login function
//

func vlogin(ctx context.Context, vc, user, pwd string) (*vim25.Client, *govmomi.Client, error) {

//
// This section allows for insecure govmomi logins
//

        var insecure bool
        flag.BoolVar(&insecure, "insecure", true, "ignore any vCenter TLS cert validation error")

//
// Create a vSphere/vCenter client
//
// The govmomi client requires a URL object, u.
// You cannot use a string representation of the vCenter URL.
// soap.ParseURL provides the correct object format.
//

        u, err := soap.ParseURL(vc)

        if u == nil {
                setupLog.Error(err, "Unable to parse URL. Are required environment variables set?", "controller", "GPUInfo")
                os.Exit(1)
        }

        if err != nil {
                setupLog.Error(err, "URL parsing not successful", "controller", "GPUInfo")
                os.Exit(1)
        }

        u.User = url.UserPassword(user, pwd)

//
// Session cache example taken from https://github.com/vmware/govmomi/blob/master/examples/examples.go
//
// Share govc's session cache
//
        s := &cache.Session{
                URL:      u,
                Insecure: true,
        }

//
// Create new vim25 client
//
        c1 := new(vim25.Client)

//
// Login using vim25 client c and cache session s
//
        err = s.Login(ctx, c1, nil)

        if err != nil {
                setupLog.Error(err, "GPUInfo: vim25 login not successful", "controller", "GPUInfo")
                os.Exit(1)
        }

//
// Create new govmomi client
//

        c2, err := govmomi.NewClient(ctx, u, insecure)

        if err != nil {
                setupLog.Error(err, "GPUInfo: gomvomi login not successful", "controller", "GPUInfo")
                os.Exit(1)
        }

        return c1, c2, nil
}
```

Within the main function, there is a call to the __vlogin__ function with the parameters received from the environment variables shown below.

```go
//
// Retrieve vCenter URL, username and password from environment variables
// These are provided via the manager manifest when controller is deployed
//

        vc := os.Getenv("GOVMOMI_URL")
        user := os.Getenv("GOVMOMI_USERNAME")
        pwd := os.Getenv("GOVMOMI_PASSWORD")

//
// Create context, and get vSphere session information
//

        ctx, cancel := context.WithCancel(context.Background())
        defer cancel()

        c1, c2, err := vlogin(ctx, vc, user, pwd)
        if err != nil {
                setupLog.Error(err, "unable to get login session to vSphere")
                os.Exit(1)
        }

        finder := find.NewFinder(c2.Client, true)

//
// -- find and set the default datacenter
//

        dc, err := finder.DefaultDatacenter(ctx)

        if err != nil {
                setupLog.Error(err, "GPUInfo: Could not get default datacenter")
        } else {
                finder.SetDatacenter(dc)
        }
```

There is also an updated __GPUInfoReconciler__ call with new fields (VC1 & VC2) which have the vSphere session details. This login information can now be used from within the GPUInfoReconciler controller function, as we will see shortly.

```go
 if err = (&controllers.GPUInfoReconciler{
                Client: mgr.GetClient(),
                VC1:    c1,
                VC2:    c2,
                Finder: finder,
                Log:    ctrl.Log.WithName("controllers").WithName("GPUInfo"),
                Scheme: mgr.GetScheme(),
        }).SetupWithManager(mgr); err != nil {
                setupLog.Error(err, "unable to create controller", "controller", "GPUInfo")
                os.Exit(1)
```

Click here for the complete [__main.go__](./main.go) code.

### Step 6.2 - Controller Reconcile Logic ###

Now we turn our attention to the business logic of the controller. Once the business logic is added in the controller, it will need to be able to run in a Kubernetes cluster. To achieve this, a container image to run the controller logic must be built. This will be provisioned in the Kubernetes cluster using a Deployment manifest. The deployment contains a single Pod that runs the container (it is called __manager__). The deployment ensures that the controller manager Pod is restarted in the event of a failure.

This is what kubebuilder provides as controller scaffolding - it is found in __controllers/gpuinfo_controller.go__. We are most interested in the __GPUInfoReconciler__ function:

```go
func (r *GPUInfoReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
        _ = context.Background()
        _ = r.Log.WithValues("gpuinfo", req.NamespacedName)

        // your logic here

        return ctrl.Result{}, nil
}
```

Considering the business logic that I described above, this is what my updated __GPUInfoReconciler__ function looks like. Hopefully the comments make is easy to understand, but at the end of the day, when this controller gets a reconcile request (something as simple as a get command will trigger this), the status fields of the Custom Resource are updated with the information from the from the specification field. Note that I have omitted a number of required imports that also need to be added to the controller. Refer to the code for the complete [__gpuinfo_controller.go__](./controllers/gpuinfo_controller.go) code.

First, lets look at the modified GPUInfoReconciler structure, which now has 2 new members representing the different clients, VC1 and VC2. It also has an entry for the Finder construct.

```go
// GPUInfoReconciler reconciles a GPUInfo object
type GPUInfoReconciler struct {
        client.Client
        VC1    *vim25.Client
        VC2    *govmomi.Client
        Finder *find.Finder
        Log    logr.Logger
        Scheme *runtime.Scheme
}
```

Now lets look at the business logic / Reconcile code. Event though I have both govmomi and vim25 clients to get different information, I am using the vim25 client to  information in this tutorial. Again, this is just a learning exercise, to show various ways to retrieve vSphere information from an operator. The flow here is that we first get a list of Kubernetes nodes, then the list of VMs, and then we find which ones match. With the list of matching nodes, we see if they meet the criteria placed in the specification.  Once the list of nodes is narrowed down to the most suitable node, we populate the status fields with the requested information. I have added some additional logging messages to this controller logic, and we can check the manager logs to see these messages later on.

First let's look at getting the list of Kubernetes nodes:

```go
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
```

Now let's look at the code that matches this K8s node to a VM:

```go
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
```

With the controller logic now in place, we can now proceed to build the Manager. The Manager is an executable that wraps one or more Controllers. 

## Step 7 - Test running the controller in the current context ##

The Makefile that is provided with __Kubebuilder__ allows us to do a few cool things when it comes to testing the controller logic. First, we can run a make command that will run the manager in the foreground, and it will run locally against the cluster defined in ~/.kube/config. Note this requires a running Kubernetes cluster to be accessible with the ~/.kube/config. 

Another requirement is that the environment variables needed for vCenter connectivity will need to be set in your local shell where the controller is run, e.g.

```shell
export VC_HOST='192.168.0.100'
export VC_USER='administrator@vsphere.local'
export VC_PASS='VMware123!'
```

The command to launch the controller is ```make run``` and a sample output from such a command is as follows:

```Makefile
$ make run
go: creating new go.mod: module tmp
go: found sigs.k8s.io/controller-tools/cmd/controller-gen in sigs.k8s.io/controller-tools v0.2.5
/usr/share/go/bin/controller-gen object:headerFile="hack/boilerplate.go.txt" paths="./..."
go fmt ./...
go vet ./...
/usr/share/go/bin/controller-gen "crd:preserveUnknownFields=false,crdVersions=v1,trivialVersions=true" rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=config/crd/bases
go run ./main.go
2021-02-12T09:25:02.741Z        INFO    controller-runtime.metrics      metrics server is starting to listen    {"addr": ":8080"}
2021-02-12T09:25:02.868Z        INFO    setup   starting manager
2021-02-12T09:25:02.869Z        INFO    controller-runtime.manager      starting metrics server {"path": "/metrics"}
2021-02-12T09:25:02.869Z        INFO    controller-runtime.controller   Starting EventSource    {"controller": "gpuinfo", "source": "kind source: /, Kind="}
2021-02-12T09:25:02.969Z        INFO    controller-runtime.controller   Starting Controller     {"controller": "gpuinfo"}
2021-02-12T09:25:02.969Z        INFO    controller-runtime.controller   Starting workers        {"controller": "gpuinfo", "worker count": 1}
```

This looks good so far - no errors in the output. At this point, you could skip ahead to step 13 to do a test of the controller functionality.

## Step 8 - Run the controller in 'development mode' ##

This next step allows us to create the actual controller and lets us manually run it locally. This means that the environment variables needed for vCenter connectivity will also need to be set, as per the previous step. If the controller is still runnning from the previous step, simple control-C it to stop it.

Here is an example of how we can run the controller in 'development mode':

```Makefile
$ make manager
go: creating new go.mod: module tmp
go: found sigs.k8s.io/controller-tools/cmd/controller-gen in sigs.k8s.io/controller-tools v0.2.5
/usr/share/go/bin/controller-gen object:headerFile="hack/boilerplate.go.txt" paths="./..."
go fmt ./...
go vet ./...
go build -o bin/manager main.go
```

```shell
$ bin/manager
2021-02-12T09:39:13.559Z        INFO    controller-runtime.metrics      metrics server is starting to listen    {"addr": ":8080"}
2021-02-12T09:39:13.630Z        INFO    setup   starting manager
2021-02-12T09:39:13.631Z        INFO    controller-runtime.manager      starting metrics server {"path": "/metrics"}
2021-02-12T09:39:13.632Z        INFO    controller-runtime.controller   Starting EventSource    {"controller": "gpuinfo", "source": "kind source: /, Kind="}
2021-02-12T09:39:13.732Z        INFO    controller-runtime.controller   Starting Controller     {"controller": "gpuinfo"}
2021-02-12T09:39:13.733Z        INFO    controller-runtime.controller   Starting workers        {"controller": "gpuinfo", "worker count": 1}
```

This continues to look good - again, no errors in the output. At this point, you could once again skip ahead to step [13.4](https://github.com/cormachogan/gpuinfo#step-134---check-a-if-suitable-candidate-is-returned-in-the-status) to do a test of the controller functionality.

If everything is working as expected, we can now proceed with creating the Manager as a container which can be run in the cluster.

## Step 9 - Build the Manager executable as a container ##

At this point everything is in place to enable us to deploy the controller to the Kubernete cluster. If you remember back to the prerequisites in step 1, we said that you need access to a container image registry, such as __docker.io__ or __quay.io__, or VMware's own [Harbor](https://github.com/goharbor/harbor/blob/master/README.md) registry. This is where we need this access to a registry, as we need to push the controller's container image somewhere that can be accessed from your Kubernetes cluster. In this example, I am using quay.io as my repository.

The __Dockerfile__ with the appropriate directives is already in place to build the container image and include the controller / manager logic. This was once again taken care of by kubebuilder. You must ensure that you login to your image repository, i.e. docker login, before proceeding with the __make__ commands, e.g.

```shell
$ docker login quay.io
Username: cormachogan
Password: ***********
WARNING! Your password will be stored unencrypted in /home/cormac/.docker/config.json.
Configure a credential helper to remove this warning. See
https://docs.docker.com/engine/reference/commandline/login/#credentials-store

Login Succeeded
$
```

Next, set an environment variable called __IMG__ to point to your container image repository along with the name and version of the container image, e.g:

```shell
export IMG=quay.io/cormachogan/gpuinfo-controller:v1
```

Next, to create the container image of the controller / manager, and push it to the image container repository in a single step, run the following __make__ command. You could of course run this as two seperate commands as well, ```make docker-build``` followed by ```make docker-push``` if you so wished.

```cmd
make docker-build docker-push IMG=quay.io/cormachogan/gpuinfo-controller:v1
```

The output has been shortened in this example:

```Makefile
$ make docker-build docker-push IMG=quay.io/cormachogan/gpuinfo-controller:v1
go: creating new go.mod: module tmp
go: found sigs.k8s.io/controller-tools/cmd/controller-gen in sigs.k8s.io/controller-tools v0.2.5
/usr/share/go/bin/controller-gen object:headerFile="hack/boilerplate.go.txt" paths="./..."
go fmt ./...
go vet ./...
/usr/share/go/bin/controller-gen "crd:preserveUnknownFields=false,crdVersions=v1,trivialVersions=true" rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=config/crd/bases
go test ./... -coverprofile cover.out
?       gpuinfo [no test files]
?       gpuinfo/api/v1  [no test files]
ok      gpuinfo/controllers     8.595s  coverage: 0.0% of statements
docker build . -t quay.io/cormachogan/gpuinfo-controller:v1
Sending build context to Docker daemon  40.27MB
Step 1/14 : FROM golang:1.13 as builder
 ---> d6f3656320fe
Step 2/14 : WORKDIR /workspace
 ---> Using cache
 ---> 0f6c055c6fc8
Step 3/14 : COPY go.mod go.mod
 ---> 3312f986aae7
Step 4/14 : COPY go.sum go.sum
 ---> 94b418c8c809
Step 5/14 : RUN go mod download
 ---> Running in 4d8f9baef8dd
go: finding cloud.google.com/go v0.38.0
go: finding github.com/Azure/go-ansiterm v0.0.0-20170929234023-d6e3b3328b78
go: finding github.com/Azure/go-autorest/autorest v0.9.0
go: finding github.com/Azure/go-autorest/autorest/adal v0.5.0
.
. <-- snip!
.
go: finding sigs.k8s.io/controller-runtime v0.5.0
go: finding sigs.k8s.io/structured-merge-diff v1.0.1-0.20191108220359-b1b620dd3f06
go: finding sigs.k8s.io/yaml v1.1.0
Removing intermediate container 4d8f9baef8dd
 ---> 356f55a22080
Step 6/14 : COPY main.go main.go
 ---> 667ef6adcc1b
Step 7/14 : COPY api/ api/
 ---> ec53a81d33ab
Step 8/14 : COPY controllers/ controllers/
 ---> 76d50bddb1f4
Step 9/14 : RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on go build -a -o manager main.go
 ---> Running in 65c6ec99969f
Removing intermediate container 65c6ec99969f
 ---> 06d5809e0c60
Step 10/14 : FROM gcr.io/distroless/static:nonroot
 ---> aa99000bc55d
Step 11/14 : WORKDIR /
 ---> Using cache
 ---> 8bcbc4c15403
Step 12/14 : COPY --from=builder /workspace/manager .
 ---> a3c9917e9534
Step 13/14 : USER nonroot:nonroot
 ---> Running in b7616960444d
Removing intermediate container b7616960444d
 ---> 2f83d33251f3
Step 14/14 : ENTRYPOINT ["/manager"]
 ---> Running in 25e5348f5857
Removing intermediate container 25e5348f5857
 ---> 0d5df043fa77
Successfully built 0d5df043fa77
Successfully tagged quay.io/cormachogan/gpuinfo-controller:v1
docker push quay.io/cormachogan/gpuinfo-controller:v1
The push refers to repository [quay.io/cormachogan/gpuinfo-controller]
8e30502bb918: Pushed
7a5b9c0b4b14: Layer already exists
v1: digest: sha256:f9d0c5242d31dfc71c81201b6bcb27f83e28e213a4b03c20bfca6d6f45388257 size: 739
$
```

The container image of the controller is now built and pushed to the container image registry. But we have not yet deployed it. We have to do one or two further modifications before we take that step.

## Step 10 - Modify the Manager manifest to include environment variables ##

Kubebuilder provides a manager manifest scaffold file for deploying the controller. However, since we need to provide vCenter details to our controller, we need to add these to the controller/manager manifest file. This is found in __config/manager/manager.yaml__. This manifest contains the deployment for the controller. In the spec, we need to add an additional __spec.env__ section which has the environment variables defined, as well as the name of our __secret__ (which we will create shortly). Below is a snippet of that code. Here is the code-complete [config/manager/manager.yaml](./config/manager/manager.yaml)).

```yaml
    spec:
      .
      .
        env:
          - name: GOVMOMI_USERNAME
            valueFrom:
              secretKeyRef:
                name: vc-creds
                key: GOVMOMI_USERNAME
          - name: GOVMOMI_PASSWORD
            valueFrom:
              secretKeyRef:
                name: vc-creds
                key: GOVMOMI_PASSWORD
          - name: GOVMOMI_URL
            valueFrom:
              secretKeyRef:
                name: vc-creds
                key: GOVMOMI_URL
      volumes:
        - name: vc-creds
          secret:
            secretName: vc-creds
      terminationGracePeriodSeconds: 10
```

Note that the __secret__, called __vc-creds__ above, contains the vCenter credentials. This secret needs to be deployed in the same namespace that the controller is going to run in, which is __gpuinfo-system__. Thus, the namespace and secret are created using the following commands, with the environment modified to your own vSphere infrastructure obviously:

```shell
$ kubectl create ns gpuinfo-system
namespace gpuinfo-system created
```

```shell
$ kubectl create secret generic vc-creds \
--from-literal='GOVMOMI_USERNAME=administrator@vsphere.local' \
--from-literal='GOVMOMI_PASSWORD=VMware123!' \
--from-literal='GOVMOMI_URL=192.168.0.100' \
-n gpuinfo-system
secret/vc-creds created
```

We are now ready to deploy the controller to the Kubernetes cluster.

## Step 11 - ClusterRole and ClusterRoleBinding ##

Because this operator is going to try to access Kubernetes objects such as nodes, you need to ensure that the service account has the approriate privileges. Here is an example of what you might see in the logs when the operator is deployed, if you don't set the privileges correctly:

```shell
Failed to list *v1.Node: nodes is forbidden: User \
"system:serviceaccount:accelerator-operator-system:default" \
cannot list resource "nodes" in API group "" at the cluster scope
```

To address this, you can grant the service account additional privileges. There is an YAML manifest thaty opens up the privileges [here](./securityPolicy-svc-accs.yaml) for reference. This can be used as a reference, but you may want to adjust the rules to suit your environment.

## Step 12 - Deploy the controller ##

To deploy the controller, we run another __make__ command. This will take care of all of the RBAC, cluster roles and role bindings necessary to run the controller, as well as pinging up the correct image, etc.

```Makefile
make deploy IMG=quay.io/cormachogan/gpuinfo-controller:v1
```

The output looks something like this:

```Makefile
$ make deploy IMG=quay.io/cormachogan/gpuinfo-controller:v1
go: creating new go.mod: module tmp
go: found sigs.k8s.io/controller-tools/cmd/controller-gen in sigs.k8s.io/controller-tools v0.2.5
/usr/share/go/bin/controller-gen "crd:preserveUnknownFields=false,crdVersions=v1,trivialVersions=true" rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=config/crd/bases
cd config/manager && kustomize edit set image controller=quay.io/cormachogan/gpuinfo-controller:v1
kustomize build config/default | kubectl apply -f -
namespace/accelerator-operator-system unchanged
customresourcedefinition.apiextensions.k8s.io/gpuinfoes.topology.corinternal.com configured
role.rbac.authorization.k8s.io/accelerator-operator-leader-election-role unchanged
clusterrole.rbac.authorization.k8s.io/accelerator-operator-manager-role configured
clusterrole.rbac.authorization.k8s.io/accelerator-operator-proxy-role unchanged
clusterrole.rbac.authorization.k8s.io/accelerator-operator-metrics-reader unchanged
rolebinding.rbac.authorization.k8s.io/accelerator-operator-leader-election-rolebinding unchanged
clusterrolebinding.rbac.authorization.k8s.io/accelerator-operator-manager-rolebinding unchanged
clusterrolebinding.rbac.authorization.k8s.io/accelerator-operator-proxy-rolebinding unchanged
service/accelerator-operator-controller-manager-metrics-service unchanged
deployment.apps/accelerator-operator-controller-manager created
```

## Step 13 - Check controller functionality ##

Now that our controller has been deployed, let's see if it is working. There are a few different commands that we can run to verify the operator is working.

### Step 13.1 - Check the deployment and replicaset ###

The deployment should be READY. Remember to specify the namespace correctly when checking it.

```shell
$ kubectl get rs -n accelerator-operator-system
NAME                                                 DESIRED   CURRENT   READY   AGE
accelerator-operator-controller-manager-85b5f7c788   1         1         1       54m

$ kubectl get deploy -n accelerator-operator-system
NAME                                      READY   UP-TO-DATE   AVAILABLE   AGE
accelerator-operator-controller-manager   1/1     1            1           55m
```

### Step 13.2 - Check the Pods ###

The deployment manages a single controller Pod. There should be 2 containers READY in the controller Pod. One is the __controller / manager__ and the other is the __kube-rbac-proxy__. The [kube-rbac-proxy](https://github.com/brancz/kube-rbac-proxy/blob/master/README.md) is a small HTTP proxy that can perform RBAC authorization against the Kubernetes API. It restricts requests to authorized Pods only.

```shell
kubectl get pods -n accelerator-operator-system
NAME                                                       READY   STATUS    RESTARTS   AGE
accelerator-operator-controller-manager-85b5f7c788-zx49v   2/2     Running   0          56m
```

If you experience issues with the one of the pods not coming online, use the following command to display the Pod status and examine the events.

```shell
kubectl describe pod accelerator-operator-controller-manager-85b5f7c788-zx49v -n accelerator-operator-system
```

### Step 13.3 - Check the controller / manager logs ###

If we query the __logs__ on the manager container, we should be able to observe successful startup messages as well as successful reconcile requests from the GPUInfo CR that we already deployed back in step 5. These reconcile requests should update the __Status__ fields with node information as per our controller logic. The command to query the manager container logs in the controller Pod is as follows:

```shell
kubectl logs accelerator-operator-controller-manager-85b5f7c788-zx49v -n accelerator-operator-system manager
```

The output should be somewhat similar to this. Note that there is also a successful __Reconcile__ operation reported, which is good. We can also see some log messages which were added to the controller logic.

```shell
$ kubectl logs accelerator-operator-controller-manager-85b5f7c788-zx49v -n accelerator-operator-system manager
2021-02-11T14:57:28.469Z        INFO    controller-runtime.metrics      metrics server is starting to listen    {"addr": "127.0.0.1:8080"}
2021-02-11T14:57:28.695Z        INFO    setup   starting manager
I0211 14:57:28.695393       1 leaderelection.go:242] attempting to acquire leader lease  accelerator-operator-system/ce376515.corinternal.com...
2021-02-11T14:57:28.695Z        INFO    controller-runtime.manager      starting metrics server {"path": "/metrics"}
I0211 14:57:46.087582       1 leaderelection.go:252] successfully acquired lease accelerator-operator-system/ce376515.corinternal.com
2021-02-11T14:57:46.087Z        DEBUG   controller-runtime.manager.events       Normal  {"object": {"kind":"ConfigMap","namespace":"accelerator-operator-system","name":"ce376515.corinternal.com","uid":"16cc7050-6697-4832-81b8-e920a18553ac","apiVersion":"v1","resourceVersion":"1123496"}, "reason": "LeaderElection", "message": "accelerator-operator-controller-manager-85b5f7c788-zx49v_73181597-0080-442d-ae15-090508cf4ffc became leader"}
2021-02-11T14:57:46.087Z        INFO    controller-runtime.controller   Starting EventSource    {"controller": "gpuinfo", "source": "kind source: /, Kind="}
2021-02-11T14:57:46.188Z        INFO    controller-runtime.controller   Starting Controller     {"controller": "gpuinfo"}
2021-02-11T14:57:46.188Z        INFO    controller-runtime.controller   Starting workers        {"controller": "gpuinfo", "worker count": 1}
2021-02-11T14:57:46.188Z        INFO    controllers.GPUInfo     received reconcile request for "gpuinfo-sample" (namespace: "default")    {"gpuinfo": "default/gpuinfo-sample"}
2021-02-11T14:57:46.288Z        INFO    controllers.GPUInfo     DEBUG: found 4 nodes in cluster {"gpuinfo": "default/gpuinfo-sample"}
2021-02-11T14:57:46.767Z        INFO    controllers.GPUInfo     Found a total of *** 1 *** suitable candidates for the long running job
        {"gpuinfo": "default/gpuinfo-sample"}
2021-02-11T14:57:46.867Z        DEBUG   controller-runtime.controller   Successfully Reconciled {"controller": "gpuinfo", "request": "default/gpuinfo-sample"}
2021-02-11T14:57:46.867Z        DEBUG   controller-runtime.controller   Successfully Reconciled {"controller": "gpuinfo", "request": "default/gpuinfo-sample"}
```

### Step 13.4 - Check a if suitable candidate is returned in the status ###

Last but not least, let's see if we can see the candidate information in the __status__ fields of the GPUInfo object created earlier in Step 5, when we tested the CRD. If you deleted the GPUInfo object, create it again, and query it as follows:

```yaml
$ kubectl get gpu gpuinfo-sample -o yaml
apiVersion: topology.corinternal.com/v1
kind: GPUInfo
metadata:
  creationTimestamp: "2021-02-11T15:17:20Z"
  generation: 1
  managedFields:
  - apiVersion: topology.corinternal.com/v1
    fieldsType: FieldsV1
    fieldsV1:
      f:spec:
        .: {}
        f:desAccTime: {}
        f:gpuRequired: {}
    manager: kubectl
    operation: Update
    time: "2021-02-11T15:17:20Z"
  - apiVersion: topology.corinternal.com/v1
    fieldsType: FieldsV1
    fieldsV1:
      f:status:
        .: {}
        f:availableAcceleratorTime: {}
        f:nodeCPUUsage: {}
        f:nodeMemoryUsage: {}
        f:suitableHostName: {}
        f:suitableNodeName: {}
    manager: manager
    operation: Update
    time: "2021-02-11T15:17:21Z"
  name: gpuinfo-sample
  namespace: default
  resourceVersion: "1129225"
  selfLink: /apis/topology.corinternal.com/v1/namespaces/default/gpuinfoes/gpu1
  uid: 740ef10b-9723-49cd-9409-14cd52a2cb4f
spec:
  desAccTime: 100
  gpuRequired: true
status:
  availableAcceleratorTime: 399
  nodeCPUUsage: 87
  nodeMemoryUsage: 327
  suitableHostName: esxi-dell-h.rainpole.com
  suitableNodeName: tkg-cluster-1-18-5-workers-7n2q2-548d48669d-l9fgw
```

__Success!!!__ Note that the output above is showing us ```suitableNodeName```, ```suitableHostName```, and ```availableAcceleratorTime``` as well as other fields as per our business logic implemented in the controller. How cool is that? You can now go ahead and create additional GPUInfo manifests for long running jobs in your Kubernetes environment by specifying different __desAccTime__  and __gpuRequired__ in the manifest spec, and get information about other candidates as well.

## Cleanup ##

To remove the __gpuinfo__ CR, operator and CRD, run the following commands.

### Remove the GPUInfo CR ###

```shell
$ kubectl delete gpuinfo gpuinfo-sample
gpuinfo.topology.corinternal.com "gpuinfo-sample" deleted
```

### Removed the Operator/Controller deployment ###

Deleting the deployment will removed the ReplicaSet and Pods associated with the controller.

```shell
$ kubectl get deploy -n gpuinfo-system
NAME                         READY   UP-TO-DATE   AVAILABLE   AGE
gpuinfo-controller-manager   1/1     1            1           17m
```

```shell
$ kubectl delete deploy gpuinfo-controller-manager -n gpuinfo-system
deployment.apps "gpuinfo-controller-manager" deleted
```

### Remove the CRD ###

Next, remove the Custom Resource Definition, __gpuinfoes.topology.corinternal.com__.

```shell
$ kubectl get crds
NAME                                                               CREATED AT
antreaagentinfos.clusterinformation.antrea.tanzu.vmware.com        2021-02-08T13:54:42Z
antreacontrollerinfos.clusterinformation.antrea.tanzu.vmware.com   2021-02-08T13:54:42Z
clusternetworkpolicies.security.antrea.tanzu.vmware.com            2021-02-08T13:54:42Z
gpuinfoes.topology.corinternal.com                                 2021-02-09T13:07:19Z
traceflows.ops.antrea.tanzu.vmware.com                             2021-02-08T13:54:42Z
```

```Makefile
$ make uninstall
go: creating new go.mod: module tmp
go: found sigs.k8s.io/controller-tools/cmd/controller-gen in sigs.k8s.io/controller-tools v0.2.5
/usr/share/go/bin/controller-gen "crd:preserveUnknownFields=false,crdVersions=v1,trivialVersions=true" rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=config/crd/bases
kustomize build config/crd | kubectl delete -f -
customresourcedefinition.apiextensions.k8s.io "gpuinfoes.topology.corinternal.com" deleted
```

```shell
$ kubectl get crds
NAME                                                               CREATED AT
antreaagentinfos.clusterinformation.antrea.tanzu.vmware.com        2021-02-08T13:54:42Z
antreacontrollerinfos.clusterinformation.antrea.tanzu.vmware.com   2021-02-08T13:54:42Z
clusternetworkpolicies.security.antrea.tanzu.vmware.com            2021-02-08T13:54:42Z
traceflows.ops.antrea.tanzu.vmware.com                             2021-02-08T13:54:42Z
```

The CRD is now removed. At this point, you can also delete the namespace created for the exercise, in this case __gpuinfo-system__. Removing this namespace will also remove the __vc_creds__ secret created earlier.

## What next? ##

One thing you could do it to extend the __GPUInfo__ fields and Operator logic so that it returns even more information about the suitable nodes and clusters. There is a lot of information that can be retrieved via the govmomi API calls.

You can now use __kusomtize__ to package the CRD and controller and distribute it to other Kubernetes clusters. Simply point the __kustomize build__ command at the location of the __kustomize.yaml__ file which is in __config/default__.

```shell
kustomize build config/default/ >> /tmp/gpuinfo.yaml
```

This newly created __gpuinfo.yaml__ manifest includes the CRD, RBAC, Service and Deployment for rolling out the operator on other Kubernetes clusters. Nice, eh?

Finally, if this exercise has given you a desire to do more exciting stuff with Kubernetes Operators when Kubernetes is running on vSphere, check out the [vmGroup](https://github.com/embano1/codeconnect-vm-operator/blob/main/README.md) operator that my colleague __Micheal Gasch__ created. It will let you deploy and manage a set of virtual machines on your vSphere infrastructure via a Kubernetes operator. Cool stuff for sure.
