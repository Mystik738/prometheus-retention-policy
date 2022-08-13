# prometheus-retention-policy
Adds a little more complexity to Prometheus's retention, allowing metrics to be stored for different periods of time.

## Environment Variables

This service requires two envirnment variables to be set for the job to run successfully

| Name | Description | Type | Example | 
|------|-------------|------|---------|
| PROMETHEUS_URL | Base URL of your prometheus service | `string` | https://prometheus.local | 
| POLICY | Your retention policy | `string` | <pre>{<br>  "nextgen.networking.type": "peering"<br>}</pre> | no |