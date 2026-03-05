# About

Datadog is a helm plugin to annotate deployments with datadog specific annotation
to map deployed resource to their location. You can read more about it in the [documentation](https://docs.datadoghq.com/source_code/resource-mapping/?tab=rawkubernetesyaml).

# Helm 4 Plugin installation

Helm 4 introduced a new plugin system that requires splitting plugins into multiple packages when they have multiple capabilities. 

datadog: A plugin that adds add cli command support.
datadog-post-renderer: A plugin that adds annotations in post-rendering.

Both plugins needs to be installed using following commands (note verification is enabeld):

`helm plugin install https://github.com/DataDog/helm-plugin/releases/download/v0.1.3/datadog-0.1.3.tgz --verify=false`
`helm plugin install https://github.com/DataDog/helm-plugin/releases/download/v0.1.3/datadog-post-renderer-0.1.3.tgz --verify=false`

Note that verification is disabled for now, but we are working on enabling it.

Please check latest release version here: `https://github.com/DataDog/helm-plugin/releases`

### Verification

Verification of plugins is supported in Helm 4 and enabled by default. 
You can choose to skip verification of the plugins during installation by adding the --verify=false flag.

Install a specific version (recommend).

The --version flag is not supported in Helm 4, so you need to specify the exact download URL for the desired version.

# Helm 3 Plugin installation

Install a specific version (recommend):

`helm plugin install https://github.com/DataDog/helm-plugin --version vX.Y.Z`

Please check latest release version here: `https://github.com/DataDog/helm-plugin/releases`

Install latest version from main branch:

`helm plugin install https://github.com/DataDog/helm-plugin`

# Debugging the instalation

If you have issues installing the plugin you can debug it by
addind environment variable: `HELM_DEBUG=1`

Example: `HELM_DEBUG=1 helm plugin install https://github.com/DataDog/helm-plugin`

# Usage

If you already using helm to manage your deployments then switching to use the plugin could
be done just by adding a call to the plugin:

Regular Helm call:      `helm install <release> <chart/path> <flags>`

Helm call using plugin: `helm datadog <plugin flags> install <release> <chart/path> <flags>`
