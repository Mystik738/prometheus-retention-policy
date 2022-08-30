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
| POLICY | Your retention policy | `string` | <pre>{<br>  {"set_default": true, "default": 31536000, "retentions":[{"seconds":86400,"metrics":["series_a", "series_b"]}]}<br>}</pre> | no |