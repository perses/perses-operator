local v = importstr '../VERSION';
local persesOperator = import 'perses-operator.libsonnet';

// example.jsonnet with high availability enabled
local config = {
  namespace: 'perses-dev',
  version: 'v' + std.strReplace(v, '\n', ''),
  highAvailability: true,
};

// Generate all manifests
persesOperator(config)
