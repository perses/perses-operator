## Openshift Sample Configuration

### Connect Perses to an Openshift Cluster Thanos querier

Openshift provides a built-in Thanos Querier as part of its Monitoring stack. Using the pre-configured routes in OpenShift you can query Thanos from Perses.

1. Login to your openshift cluster using the `oc` CLI tool:
2. Retrieve the Thanos Querier route URL:
   ```bash
   oc get route thanos-querier -n openshift-monitoring -o jsonpath='{.spec.host}'
   ```
3. Create a Perses project or use an existing one.
4. In your Perses project, create a secret to store your OpenShift token:
    - **Name**: "openshift-token"
    - select **Custom Authorization**
    - **Type**: "Bearer"
    - **Credentials**: Paste your OpenShift token, which you can obtain by running:
      ```bash
      oc whoami -t
      ```
    - Enable the TLS configuration and toggle the "Insecure Skip Verify" option to true if your cluster uses self-signed certificates.
    - Save the secret.
5. In your Perses project, create a new Datasource with the following configuration:
    - **Name**: Openshift Thanos Querier
    - **Set as Default**: true
    - **Plugin Options**: Source: Prometheus Datasource
    - **HTTP Settings**: Select Proxy and set the URL to `https://<thanos-querier-route-url>` (obtained from step 2)
    - **Secret**: Fill the name of the secret, in this case "openshift-token" (created in step 4)
    - Save the Datasource.
6. In your Perses project, create a Dashboard and its Panels. As the Datasource is the default, all the queries will use the Openshift Thanos Querier by default.
> [!IMPORTANT]
> If you face any issues connecting to the Thanos Querier, ensure that your Perses instance can reach the Openshift cluster network and that the token has the necessary permissions to access the monitoring data.