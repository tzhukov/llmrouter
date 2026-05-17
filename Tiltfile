load('ext://namespace', 'namespace_create', 'namespace_inject')

docker_build('llmrouter', '.')

namespace_create('llmrouter')

# Local dev config and secrets used by llmrouter
k8s_yaml(['local-dev/configmap.yaml', 'local-dev/secret.yaml'])

# Deploy using the Helm chart
k8s_yaml(helm(
  'charts/llmrouter',
  name='llmrouter',
  namespace='llmrouter',
  set=['config.create=false']
  )   
)

# LiteLLM Proxy
k8s_yaml('local-dev/litellm-proxy.yaml')

# Resource settings
k8s_resource('llmrouter', port_forwards=8080)
k8s_resource('litellm', port_forwards=8000)
