# Job feedback and recovery

This is the first bounded E03 consolidation pass from the
[experience audit](experience-audit-2026-10-22.md). It covers the native queue,
conversion feedback and the upscaling handoff. Remote browsing layout and a new
server job-history database are outside this pass.

## Following work

**Settings → Tasks → Job queue** shows current native jobs. Queue and processing
feedback use Queued, Running, Stopping, Completed, Cancelled and Failed labels.
A conversion that finishes with returned item errors shows **Completed with
errors**; inspect its failed items before deciding what to do next. An unfamiliar
backend state keeps its original name rather than being presented as success.

Converter **Convert** and **Estimate / trial review / apply** tabs use the same
feedback. Failed native targets link directly to the existing Image or Video;
equal numeric IDs retain their media type. Following such a link closes the
converter. A trial without an actual media target does not get a fabricated
library link. **View job queue (new tab)** opens Tasks while preserving the current
processing dialog.

**Copy error** preserves complete multiline errors. Conversion shows at most 50
item-error rows but copies all errors returned by that job response, including
item IDs. Clipboard failures are reported, with the existing insecure-HTTP
fallback retained. A failed stop request remains retryable. A successful stop
request waits for a native status update; it does not claim completed work was
undone. Converter cancellation disables the request button while submitting.

Upscaling reports a **submitted** job and links to the queue. This acknowledgement
is not a live completion result. New-copy and replacement behavior, worker choices,
model defaults and original-file recovery keep their existing contracts.

## What is retained

A queue notice, a saved result and an original available for recovery have
separate lifetimes. Refreshing a page does not turn any of them into a complete
server job audit log.

| Data | Page refresh / navigation | Server restart / expiry |
| --- | --- | --- |
| Native live queue | Refetched from the server; subscription updates reconcile with query snapshots. | Process memory only. Completed jobs leave the server queue; IDs can be reused after restart. |
| Native failed-job notices | Captured application-wide and retained in this browser tab's session storage, scoped to the server URL, until dismissed. Storage failure is reported. | Already captured notices can remain in the same open tab. Work completed while disconnected is not reconstructed. Closing the tab ends this notice history. |
| Completed/cancelled queue notices | Depart about ten seconds after the removal event. | No durable history is created. |
| Converter and upscaling-restore job details | Process-memory results can be polled while retained. Remembered browser job IDs are pointers to those details. | Lost on server restart. A cleanup check runs 24 hours after submission and removes terminal entries; a still-active job at that check is left in memory. This is not a rolling 24-hour history guarantee. |
| Conversion results, trials and original-file records | Existing conversion History/review tabs read their separate on-disk records. | Records survive restart. Recoverable original bytes depend on cache limits and state; trial bytes also have their own cache/expiry rules. A completed job does not guarantee indefinite recovery. |
| Upscaling replacement records/originals | Separate upscaling replacement history and restore cache. | On-disk records survive restart; configured cache limits can evict originals. New-copy upscales retain their normal native media rather than creating a replacement original. |
| Remote download live details | The browser can remember a job/token in the same tab for up to an hour. | Process-memory job/token expires one hour after submission or on restart. Saved files and separately written download receipts have their own lifetime. |
| Download receipts | Saved on the server and revalidated against files/native matches before reuse. | Separate on-disk receipts survive restart; they are not proof that an unchanged local file still exists and are not general job history. |
| Source-identity scan progress | The server keeps the latest scan state per configuration while running. | Progress state is process memory; saved observations/explicit connections are separate database records and remain after cancellation/restart. |

Inventory sources: [native queue manager](../pkg/job/manager.go),
[notice reducer](../ui/v2.5/src/hooks/jobQueueState.ts),
[conversion jobs](../internal/api/routes_media_conversion.go),
[trial/review jobs](../internal/api/routes_media_conversion_review.go),
[upscaling restore](../internal/api/routes_image_upscale_restore.go),
[record storage](../pkg/mediaconvert/store.go),
[separate cache roots](../internal/manager/media_conversion.go),
[download jobs](../internal/api/routes_booru_browser_download.go),
[receipts](../internal/api/booru_download_receipt.go) and
[identity scans](../internal/api/routes_provider_identity_scan.go).

## Limits and next work

This pass changes presentation and navigation; it adds no schema, worker protocol,
persistence or automatic retries. Native queue descriptions lack structured media
targets, so they are not parsed into guessed item links. Conversion links use only
existing structured targets. Record/trial states such as prepared, committed and
restoring retain their precise existing meanings.

Remaining E03 work includes reconnect/restart acceptance with representative jobs,
context links for other producers, explicit handling of remembered custom job IDs
across server restarts, and a decision about whether durable server job history
is needed. That decision should follow the retention inventory, rather than adding
another overlapping browser history store.

The Job feedback CI suite mounts the real queue and converter plus shared feedback
at desktop 1440 × 1000 and mobile 390 × 844. Its browser-local HTTP/GraphQL fixtures
perform no real conversion, GPU inference or library writes. Those checks do not
establish production restart, codec or recovery acceptance.
