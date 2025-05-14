local defaults = {
  local defaults = self,
  name: 'perses-operator',
  namespace: error 'must provide namespace',
  version: error 'must provide version',
  image: error 'must provide image',
  resources: {
    limits: { cpu: '500m', memory: '128Mi' },
    requests: { cpu: '10m', memory: '64Mi' },
  },
  commonLabels:: {
    'app.kubernetes.io/name': 'perses-operator',
    'app.kubernetes.io/version': defaults.version,
    'app.kubernetes.io/component': 'controller',
    'app.kubernetes.io/created-by': 'perses-operator',
    'app.kubernetes.io/part-of': 'perses-operator',
  },
  selectorLabels:: {
    [labelName]: defaults.commonLabels[labelName]
    for labelName in std.objectFields(defaults.commonLabels)
    if !std.setMember(labelName, ['app.kubernetes.io/version'])
  },
};

function(params) {
  local po = self,
  config:: defaults + params,

  // Prefixing with 0 to ensure these manifests are listed and therefore created first.
  '0persesCustomResourceDefinition': import 'perses-crd.json',
  '0persesdashboardsCustomResourceDefinition': import 'persesdashboards-crd.json',
  '0persesdatasourcesCustomResourceDefinition': import 'persesdatasources-crd.json',

  namespace: {
    apiVersion: 'v1',
    kind: 'Namespace',
    metadata: {
      name: po.config.namespace,
      labels: po.config.commonLabels,
    },
  },

  deployment: {
    apiVersion: 'apps/v1',
    kind: 'Deployment',
    metadata: {
      name: po.config.name,
      namespace: po.config.namespace,
      labels: po.config.commonLabels,
    },
    spec: {
      replicas: 1,
      selector: { matchLabels: po.config.selectorLabels },
      template: {
        metadata: {
          labels: po.config.commonLabels,
          annotations: {
            'kubectl.kubernetes.io/default-container': 'manager',
          },
        },
        spec: {
          containers: [{
            name: 'manager',
            image: po.config.image,
            args: ['--leader-elect'],
            securityContext: {
              allowPrivilegeEscalation: false,
              capabilities: { drop: ['ALL'] },
            },
            livenessProbe: {
              httpGet: {
                path: '/healthz',
                port: 8081,
              },
              initialDelaySeconds: 15,
              periodSeconds: 20,
            },
            readinessProbe: {
              httpGet: {
                path: '/readyz',
                port: 8081,
              },
              initialDelaySeconds: 5,
              periodSeconds: 10,
            },
            resources: po.config.resources,
          }],
          serviceAccountName: po.config.name,
          terminationGracePeriodSeconds: 10,
          affinity: {
            nodeAffinity: {
              requiredDuringSchedulingIgnoredDuringExecution: {
                nodeSelectorTerms: [{
                  matchExpressions: [
                    {
                      key: 'kubernetes.io/arch',
                      operator: 'In',
                      values: ['amd64', 'arm64'],
                    },
                    {
                      key: 'kubernetes.io/os',
                      operator: 'In',
                      values: ['linux'],
                    },
                  ],
                }],
              },
            },
          },
        },
      },
    },
  },

  serviceAccount: {
    apiVersion: 'v1',
    kind: 'ServiceAccount',
    metadata: {
      name: po.config.name,
      namespace: po.config.namespace,
      labels: po.config.commonLabels,
    },
  },

  clusterRole: {
    apiVersion: 'rbac.authorization.k8s.io/v1',
    kind: 'ClusterRole',
    metadata: {
      name: po.config.name,
      labels: po.config.commonLabels,
    },
    rules: [
      {
        apiGroups: ['apps'],
        resources: ['deployments', 'statefulsets'],
        verbs: ['create', 'delete', 'get', 'list', 'patch', 'update', 'watch'],
      },
      {
        apiGroups: [''],
        resources: ['events'],
        verbs: ['create', 'patch'],
      },
      {
        apiGroups: [''],
        resources: ['services', 'configmaps', 'secrets'],
        verbs: ['get', 'patch', 'update', 'create', 'delete', 'list', 'watch'],
      },
      {
        apiGroups: ['perses.dev'],
        resources: ['perses'],
        verbs: ['create', 'delete', 'get', 'list', 'patch', 'update', 'watch'],
      },
      {
        apiGroups: ['perses.dev'],
        resources: ['perses/finalizers'],
        verbs: ['update'],
      },
      {
        apiGroups: ['perses.dev'],
        resources: ['perses/status'],
        verbs: ['get', 'patch', 'update'],
      },
      {
        apiGroups: ['perses.dev'],
        resources: ['persesdashboards'],
        verbs: ['create', 'delete', 'get', 'list', 'patch', 'update', 'watch'],
      },
      {
        apiGroups: ['perses.dev'],
        resources: ['persesdashboards/finalizers'],
        verbs: ['update'],
      },
      {
        apiGroups: ['perses.dev'],
        resources: ['persesdashboards/status'],
        verbs: ['get', 'patch', 'update'],
      },
      {
        apiGroups: ['perses.dev'],
        resources: ['persesdatasources'],
        verbs: ['create', 'delete', 'get', 'list', 'patch', 'update', 'watch'],
      },
      {
        apiGroups: ['perses.dev'],
        resources: ['persesdatasources/finalizers'],
        verbs: ['update'],
      },
      {
        apiGroups: ['perses.dev'],
        resources: ['persesdatasources/status'],
        verbs: ['get', 'patch', 'update'],
      },
    ],
  },

  clusterRoleBinding: {
    apiVersion: 'rbac.authorization.k8s.io/v1',
    kind: 'ClusterRoleBinding',
    metadata: {
      name: po.config.name,
      labels: po.config.commonLabels,
    },
    roleRef: {
      apiGroup: 'rbac.authorization.k8s.io',
      kind: 'ClusterRole',
      name: po.config.name,
    },
    subjects: [{
      kind: 'ServiceAccount',
      name: po.config.name,
      namespace: po.config.namespace,
    }],
  },
}
