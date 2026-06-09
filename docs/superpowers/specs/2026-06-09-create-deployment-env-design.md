# Create Deployment Env Support Design

## Goal

Allow `POST /api/v1/deployments` to set environment variables on the
container created by LetoRollout.

## Scope

The first version supports two env value sources:

- Literal values, using `name` and `value`.
- Kubernetes Secret key references, using `name` and `secret.name` /
  `secret.key`.

ConfigMap references, `envFrom`, multiple containers, probes, resources, and
other Deployment customization are out of scope for this change.

## Request Shape

`DeploymentCreateRequest` gains an optional `env` array:

```json
{
  "namespace": "default",
  "name": "nginx",
  "image": "nginx:1.27.0",
  "env": [
    { "name": "APP_ENV", "value": "prod" },
    {
      "name": "DATABASE_URL",
      "secret": {
        "name": "nginx-secret",
        "key": "database-url"
      }
    }
  ]
}
```

## Validation

Each env item must satisfy these rules:

- `name` is required after trimming whitespace.
- Exactly one of `value` or `secret` must be provided.
- A `secret` value must include non-empty `name` and `key`.

Invalid input returns `400 Bad Request` before Kubernetes is called.

## Kubernetes Mapping

Literal env entries map to `corev1.EnvVar{Name, Value}`.

Secret env entries map to:

```go
corev1.EnvVar{
  Name: env.Name,
  ValueFrom: &corev1.EnvVarSource{
    SecretKeyRef: &corev1.SecretKeySelector{
      LocalObjectReference: corev1.LocalObjectReference{Name: env.Secret.Name},
      Key: env.Secret.Key,
    },
  },
}
```

The env array is applied to the single generated container named `app`.

## Response

`DeploymentCreateResult` includes the accepted `env` array so callers can
confirm what was applied.

## Testing

Tests cover:

- Creating a Deployment with literal env.
- Creating a Deployment with Secret-backed env.
- Rejecting env entries with missing names.
- Rejecting env entries with both literal and Secret sources.
- Rejecting Secret env entries with missing Secret name or key.
