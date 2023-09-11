import * as k3d from "@viuweb/k3d";
import * as pulumi from "@pulumi/pulumi";

const clusterConfig = `apiVersion: k3d.io/v1alpha5
kind: Simple
servers: 1
agents: 2
`

const cluster = new k3d.Cluster(
    `my-cluster-${pulumi.getStack()}`,
    {
        config: clusterConfig
    }
)

export const kubeConfig = cluster.kubeConfig
