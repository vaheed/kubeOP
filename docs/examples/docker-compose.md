# Docker Compose example

Use this example to run kubeOP locally with Docker Compose.

```bash
git clone https://github.com/vaheed/kubeOP.git
cd kubeOP
cp docs/examples/docker-compose.env .env
mkdir -p logs
docker compose up -d --build
```

Check health once the stack is running:

```bash
curl http://localhost:8080/healthz
curl http://localhost:8080/v1/version | jq '.build.version'
```

To tear down the stack:

```bash
docker compose down -v
```
