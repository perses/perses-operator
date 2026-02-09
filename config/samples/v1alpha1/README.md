# v1alpha1 Samples (DEPRECATED)

⚠️ **WARNING: The v1alpha1 API is deprecated.**

## Status

- **Deprecated**: Yes
- **Current API Version**: v1alpha2
- **Migration Path**: Automatic via conversion webhooks

## Why Deprecated?

The v1alpha2 API provides:

- GlobalDataSource support
- Improved schema validation
- Better field organization
- Additional features and improvements

## Migration

Existing v1alpha1 resources will automatically be converted to v1alpha2 by the operator's conversion webhooks. No manual migration is required.

For new deployments, please use the [v1alpha2 samples](../v1alpha2/) instead.

## Using These Samples

These samples are kept for:

- Historical reference
- Migration documentation
- Testing conversion webhooks

To use v1alpha1 samples (not recommended):

```bash
kubectl apply -k config/samples/v1alpha1/
```

## Recommended Action

Use v1alpha2 samples instead:

```bash
kubectl apply -k config/samples/v1alpha2/
```
