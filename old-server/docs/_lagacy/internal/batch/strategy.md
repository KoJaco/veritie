# Batch Upload strat

```bash
POST /api/v1/batch
Content-Type: multipart/form-data
Headers: x-api-key: your-api-key
Form data:

-   file: audio.wav
-   config: {"function_config": {...}}

# Get job status

GET /api/v1/batch/status?job_id=uuid

# List jobs for authenticated app

GET /api/v1/batch/jobs?limit=10&offset=0
Headers: x-api-key: your-api-key
```
