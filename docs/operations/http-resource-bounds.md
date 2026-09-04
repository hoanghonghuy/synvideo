# HTTP resource bounds

TASK-043 defines the application HTTP-edge resource policy that deployment and reverse-proxy configuration must preserve.

## Server connection bounds

The API server uses these defaults:

- `ReadHeaderTimeout`: 10 seconds. Slow/incomplete request headers are bounded.
- `IdleTimeout`: 60 seconds. Idle keep-alive connections are bounded.
- `MaxHeaderBytes`: 1 MiB.
- `ReadTimeout`: intentionally `0` (unbounded at the whole-request level). A global read deadline would also cap legitimate configured media uploads; ordinary API request bodies are instead bounded by the shared request-body policy below. TASK-041 reverse proxies should apply compatible connection/body-read controls without setting a deadline below legitimate upload requirements.
- `WriteTimeout`: intentionally `0` (unbounded at the whole-response level). Media content can be streamed and asynchronous API operations are polled rather than held open. A global write deadline could truncate legitimate streamed responses. TASK-041 may add proxy-specific write/idle safeguards only when they preserve media streaming semantics.

## Shared API request-body boundary

The shared 1 MiB body limit applies to every `POST`, `PUT`, and `PATCH` request except `multipart/form-data`. It is intentionally method/boundary based rather than limited to requests that declare `application/json`: missing, malformed, or other non-multipart content types do not bypass the bound.

If a request declares a `Content-Length` greater than 1 MiB, it is rejected immediately with HTTP `413` and the stable error code `request_body_too_large` before downstream handler/service/domain work runs.

For requests without a usable declared length (including chunked transfer), the boundary performs a bounded pre-read of at most `max + 1` bytes with `io.LimitReader` before invoking downstream handlers. If more than 1 MiB is observed, the request is rejected with the same `413 / request_body_too_large` contract and downstream work is not invoked. If the body is within the limit, the consumed bytes are reconstructed into `r.Body` so normal downstream decoding sees the complete request body.

The shared API limit is intentionally separate from `MediaStorage.MaxUploadBytes`. `multipart/form-data` media uploads are exempt from this generic boundary and keep their endpoint-specific configured upload-size semantics. Media download/content routes also keep their existing streaming semantics.

## TASK-041 reverse-proxy alignment

Deployment/reverse-proxy configuration must not silently weaken these guarantees or impose smaller generic request limits that break configured media uploads. Proxy body policies should preserve the application split: bounded ordinary POST/PUT/PATCH API bodies versus separately sized multipart/media transfers. Proxy header/read/write/idle limits should be documented alongside this application policy and must remain compatible with legitimate media upload/download streaming requirements.
