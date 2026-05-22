# LetoRollout

<div align="right">
<button onclick="switchLanguage('en')">English</button> | <button onclick="switchLanguage('zh')">中文</button>
</div>

<div id="en-content" style="display: block;">

LetoRollout is a small Go service that runs inside Kubernetes and patches a Deployment container image through an HTTP API.

## API

Health check:

```bash
curl http://localhost:8080/healthz
```

Update a Deployment image:

```bash
curl -X POST http://localhost:8080/api/v1/deployments/image \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer change-me' \
  -d '{
    "namespace": "default",
    "deployment": "nginx",
    "container": "nginx",
    "image": "nginx:1.27.0"
  }'
```

If `AUTH_TOKEN` is empty, the update API does not require the `Authorization` header.

## Configuration

| Environment variable | Default | Description |
| --- | --- | --- |
| `ADDR` | `:8080` | HTTP listen address. |
| `AUTH_TOKEN` | empty | Optional bearer token for update requests. |

The service uses Kubernetes in-cluster credentials through `rest.InClusterConfig()`.

## Test And Build

```bash
go test ./...
go build ./cmd/server
docker build -t letorollout:latest .
```

## Deploy

Create a token Secret if you want API auth:

```bash
kubectl create secret generic letorollout-auth \
  --from-literal=token='change-me' \
  -n default
```

Apply RBAC and the example Deployment:

```bash
kubectl apply -f deploy/rbac.yaml
kubectl apply -f deploy/deployment.yaml
```

The default RBAC grants the service cluster-wide Deployment access with only these verbs:

```text
apps/deployments: get, patch
```

The ServiceAccount and example Deployment run in `default`, while `ClusterRoleBinding` lets that ServiceAccount patch Deployments in any namespace. If you deploy LetoRollout into another namespace, update the ServiceAccount namespace in `deploy/rbac.yaml` and `deploy/deployment.yaml`.

## Response Example

```json
{
  "namespace": "default",
  "deployment": "nginx",
  "container": "nginx",
  "oldImage": "nginx:1.26.0",
  "newImage": "nginx:1.27.0",
  "generation": 2
}
```

</div>

<div id="zh-content" style="display: none;">

LetoRollout 是一个运行在 Kubernetes 内部的小型 Go 服务，通过 HTTP API 来更新 Deployment 的容器镜像。

## API

健康检查：

```bash
curl http://localhost:8080/healthz
```

更新 Deployment 镜像：

```bash
curl -X POST http://localhost:8080/api/v1/deployments/image \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer change-me' \
  -d '{
    "namespace": "default",
    "deployment": "nginx",
    "container": "nginx",
    "image": "nginx:1.27.0"
  }'
```

如果 `AUTH_TOKEN` 为空，更新 API 不需要 `Authorization` 请求头。

## 配置

| 环境变量 | 默认值 | 描述 |
| --- | --- | --- |
| `ADDR` | `:8080` | HTTP 监听地址。 |
| `AUTH_TOKEN` | 空 | 更新请求的可选 Bearer token。 |

服务通过 `rest.InClusterConfig()` 使用 Kubernetes 集群内凭证。

## 测试与构建

```bash
go test ./...
go build ./cmd/server
docker build -t letorollout:latest .
```

## 部署

如果需要 API 认证，创建 token Secret：

```bash
kubectl create secret generic letorollout-auth \
  --from-literal=token='change-me' \
  -n default
```

应用 RBAC 和示例 Deployment：

```bash
kubectl apply -f deploy/rbac.yaml
kubectl apply -f deploy/deployment.yaml
```

默认的 RBAC 授予服务集群范围的 Deployment 访问权限，仅包含以下操作：

```text
apps/deployments: get, patch
```

ServiceAccount 和示例 Deployment 在 `default` 命名空间中运行，而 `ClusterRoleBinding` 允许该 ServiceAccount 在任何命名空间中更新 Deployments。如果将 LetoRollout 部署到其他命名空间，请更新 `deploy/rbac.yaml` 和 `deploy/deployment.yaml` 中的 ServiceAccount 命名空间。

## 响应示例

```json
{
  "namespace": "default",
  "deployment": "nginx",
  "container": "nginx",
  "oldImage": "nginx:1.26.0",
  "newImage": "nginx:1.27.0",
  "generation": 2
}
```

</div>

<script>
function switchLanguage(lang) {
    if (lang === 'en') {
        document.getElementById('en-content').style.display = 'block';
        document.getElementById('zh-content').style.display = 'none';
    } else if (lang === 'zh') {
        document.getElementById('en-content').style.display = 'none';
        document.getElementById('zh-content').style.display = 'block';
    }

    const buttons = document.querySelectorAll('button');
    buttons.forEach(btn => {
        if (btn.textContent.includes(lang === 'en' ? 'English' : '中文')) {
            btn.style.fontWeight = 'bold';
            btn.style.textDecoration = 'underline';
        } else {
            btn.style.fontWeight = 'normal';
            btn.style.textDecoration = 'none';
        }
    });
}

document.addEventListener('DOMContentLoaded', function() {
    const enButton = document.querySelector("button[onclick=\"switchLanguage('en')\"]");
    if (enButton) {
        enButton.style.fontWeight = 'bold';
        enButton.style.textDecoration = 'underline';
    }
});
</script>

<style>
button {
    background: none;
    border: 1px solid #ccc;
    padding: 4px 12px;
    border-radius: 4px;
    cursor: pointer;
    font-size: 14px;
    margin: 0 2px;
}

button:hover {
    background-color: #f0f0f0;
}
</style>