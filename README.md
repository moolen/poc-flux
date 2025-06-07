# EKS Platform installer

90% vibe coded.

1. discovers the AWS/EKS environment to gather important data for the installation process
2. checks if prerequisites are met before installing, e.g.: Kubernetes cluster version matches, node group and instance size requirements, supported AWS regions, check if IRSA is enabled
3. reconciler for IAM infrastructure to provision IRSA for various workloads
4. install flux and point to an OCI repo to pull in workloads

After flux is installed everything* is delegated to a gitops repository.


```
┌─────────────────────┐
│     Installer       │
└─────────────────────┘
        │ Prepare()
        ▼
┌────────────────────────────┐
│ awsmeta.GetAWSMetadata()   │
└────────────────────────────┘
        │ CheckPrerequisites()
        ▼
┌──────────────────────────┐
│ checkKubernetesVersion() │
│ checkIRSA()              │
│ checkNodeGroups()        │
└──────────────────────────┘
        │ ReconcileInfrastructure()              
        ▼               
┌───────────────────┐     
│ reconcileIRSA()   │     
│ reconcileS3()     │     
└───────────────────┘     
        │ ApplyBootstrapManifests()                          
        ▼                           
┌─────────────────────────────┐           
│ Build/ApplyFlux()           │───────────┐
│ Build/ApplyClusterConfig()  │           │ 
│ Build/ApplyFluxBootstrap()  │           │ 
└─────────────────────────────┘           │
        │                                 │
        ▼                                 ▼ 
┌─────────────────────────────┐   ┌───────────────────┐        
│   platform reconcilers      │──>│ Flux Reconcilers  │
│  (secrets/vault/product)    │   └───────────────────┘
└─────────────────────────────┘   

```

