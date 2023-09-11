package provider

import (
	"github.com/acarl005/stripansi"
	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"os/exec"
)

// Version is initialized by the Go linker to contain the semver of this build.
var Version string

const Name string = "k3d"

func Provider() p.Provider {
	// We tell the provider what resources it needs to support.
	// In this case, a single custom resource.
	return infer.Provider(infer.Options{
		Resources: []infer.InferredResource{
			infer.Resource[*Cluster, ClusterArgs, ClusterState](),
		},
		ModuleMap: map[tokens.ModuleName]tokens.ModuleName{
			"provider": "index",
		},
	})
}

// Each resource has a controlling struct.
// Resource behavior is determined by implementing methods on the controlling struct.
// The `Create` method is mandatory, but other methods are optional.
// - Check: Remap inputs before they are typed.
// - Diff: Change how instances of a resource are compared.
// - Update: Mutate a resource in place.
// - Read: Get the state of a resource from the backing provider.
// - Delete: Custom logic when the resource is deleted.
// - Annotate: Describe fields and set defaults for a resource.
// - WireDependencies: Control how outputs and secrets flows through values.
type Cluster struct{}

type ClusterArgs struct {
	Name    string `pulumi:"name,optional"`
	Version string `pulumi:"version,optional"`
	Config  string `pulumi:"config,optional"`
}

type ClusterState struct {
	ClusterArgs
	KubeConfig string `pulumi:"kubeConfig" provider:"secret"`
}

func (*Cluster) Create(ctx p.Context, name string, input ClusterArgs, preview bool) (string, ClusterState, error) {
	state := ClusterState{ClusterArgs: input}
	if preview {
		return name, state, nil
	}

	if &input.Config == nil || input.Config == "" {
		input.Config = defaultConfig(name)
	}

	// Pass the name and version to k3d to create a cluster. Also pass the config via stdin.
	cmd := exec.Command("k3d", "cluster", "create", name, "--config", "-")

	// Debug the command.
	//fullCmd := cmd.String()
	//ctx.Logf(diag.Warning, "command: %q", fullCmd)

	// Get a pipe to stdin.
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return name, state, err
	}

	// Write the config to stdin.
	go func() {
		defer stdin.Close()
		_, err = stdin.Write([]byte(input.Config))
		if err != nil {
			return
		}
	}()

	// Run the command.
	output, err := cmd.CombinedOutput()
	if err != nil {
		ctx.Logf(diag.Error, "Error creating k3d cluster: %q", stripansi.Strip(string(output)))
		return name, state, err
	}

	// Get the kubeconfig for the cluster and set it as an output.
	kc, err := exec.Command("k3d", "kubeconfig", "get", name).Output()
	if err != nil {
		return name, state, err
	}

	state.KubeConfig = string(kc)

	return name, state, nil
}

func (*Cluster) Delete(ctx p.Context, id string, props ClusterState) error {
	cmd := exec.Command("k3d", "cluster", "delete", props.Name)

	output, err := cmd.CombinedOutput()
	if err != nil {
		ctx.Logf(diag.Error, "Error deleting k3d cluster: %q", stripansi.Strip(string(output)))
		return err
	}

	return err
}

func (*Cluster) Check(ctx p.Context, name string, oldInputs, newInputs resource.PropertyMap) (ClusterArgs, []p.CheckFailure, error) {
	// Set default values.
	if _, ok := newInputs["name"]; !ok {
		newInputs["name"] = resource.NewStringProperty(name)
	}
	if _, ok := newInputs["config"]; !ok {
		newInputs["config"] = resource.NewStringProperty(defaultConfig(name))
	}
	return infer.DefaultCheck[ClusterArgs](newInputs)
}

// Create a default config for the cluster.  Cluster name default to the resource name.
func defaultConfig(name string) string {
	return `apiVersion: k3d.io/v1alpha5
kind: Simple
metadata:
  name: ` + name + `
servers: 1
agents: 1
`
}
