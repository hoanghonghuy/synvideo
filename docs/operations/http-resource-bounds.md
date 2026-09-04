# HTTP resource bounds

TASK-043 defines the application HTTP-edge resource policy that deployment and reverse-proxy configuration must preserve.

## Server connection bounds

The API server uses these defaults:

- `ReadHeaderTimeout`: 10 seconds. Slow/incomplete request headers are bounded.
- `IdleTimeout`: 60 seconds. Idle keep-alive connections are bounded.
- `MaxHeaderBytes`: 1 MiB.
- `ReadTimeout`: intentionally `0` (unbounded at the whole-request level). A global read deadline would also cap legitimate configured media uploads; ordinary JSON requests are instead bounded by the shared JSON-body policy below. TASK-041 reverse proxies should apply compatible connection/body-read controls without setting a deadline below legitimate upload requirements.
- `WriteTimeout`: intentionally `0` (unbounded at the whole-response level). Media content can be streamed and asynchronous API operations are polled rather than held open. A global write deadline could truncate legitimate streamed responses. TASK-041 may add proxy-specific write/idle safeguards only when they preserve media streaming semantics.

## JSON request bodies

Requests with `Content-Type: application/json` or a `+json` media type are limited to 1 MiB at the shared HTTP boundary before routing/service work. Requests whose declared `Content-Length` exceeds the limit are rejected immediately with HTTP `413` and the stable error code `request_body_too_large`. Requests without a declared length are wrapped with `http.MaxBytesReader`, so body consumption remains finite.

The JSON limit is intentionally separate from `MediaStorage.MaxUploadBytes`. Multipart/media upload and media download/content routes keep their existing endpoint-specific size and streaming semantics.

## TASK-041 reverse-proxy alignment

Deployment/reverse-proxy configuration must not silently weaken these guarantees or impose smaller generic request limits that break configured media uploads. Proxy header/body/read/write/idle limits should be documented alongside this application policy, with JSON/API limits kept distinct from media-transfer limits.
