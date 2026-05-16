load('ext://namespace', 'namespace_create', 'namespace_inject')

docker_build('llmrouter', '.')

namespace_create('llmrouter')

# Deploy using the Helm chart
k8s_yaml(helm(
  'charts/llmrouter',
  # The release name, equivalent to helm --name
  name='llmrouter',
  # The namespace to install in, equivalent to helm --namespace
  namespace='llmrouter',
  # The values file to substitute into the chart.
  values=[],
  # Values to set from the command-line
  set=['config.create=false']
  )   
)

# Resource settings
k8s_resource('llmrouter', port_forwards=8080)
