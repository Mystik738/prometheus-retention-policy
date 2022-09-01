# prometheus-retention-policy
Adds a little more complexity to Prometheus's retention, allowing metrics to be stored for different periods of time.

## Environment Variables

This service requires two envirnment variables to be set for the job to run successfully

| Name | Description | Type | Example | 
|------|-------------|------|---------|
| SOURCE_TYPE | The type of source. Current accepted values are `api`, `minio` | `string` | api |
| SOURCE_URL | Base URL of your prometheus service | `string` | https://prometheus.local | 
| SOURCE_BUCKET | Bucket for your source if using object based storage | `string` | bucket_name |
| SOURCE_USERNAME | Username to access your source | `string` | admin |
| SOURCE_PASSWORD | Password to access your source | `string` | password |
| SOURCE_PORT | Port to access your source | `string` | 443 |
| LOG_LEVEL | Logging level to run on | `string` | info |
| POLICY | Your retention policy | `string` | <pre>{<br>  {"set_default": true, "default": 31536000, "retentions":[{"seconds":86400,"metrics":["series_a", "series_b"]}]}<br>}</pre> | no |

# Building the image

This image is built using multi-platform images. To do so, you need a custom builder (run this once)

```
docker run --rm --privileged docker/binfmt:a7996909642ee92942dcd6cff44b9b95f08dad64
docker buildx create --name mybuilder
docker buildx use mybuilder
docker buildx inspect --bootstrap
```

After you have a custom builder, you can build as necessary:

```
export VERSION=
docker buildx build --platform linux/amd64,linux/arm64,linux/arm/v7 --push -t mystik738/prometheus-retention:latest -t mystik738/prometheus-retention:${VERSION} .
```
