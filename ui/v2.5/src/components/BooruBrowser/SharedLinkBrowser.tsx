import { useRef, useState, type FormEvent } from "react";
import { Alert, Button, Form, Spinner } from "react-bootstrap";
import { BooruArchive } from "./BooruArchive";
import { BooruDownload } from "./BooruDownload";
import {
  booruMediaURL,
  type BooruPost,
  type BooruSource,
  readBooruResponse,
} from "./browser";

interface SharedLinkResult {
  source: BooruSource;
  post: BooruPost;
}

// The original link and its access key live only in component state. In
// particular, never route this value through the Booru search URL or bookmarks.
export function SharedLinkBrowser() {
  const [url, setURL] = useState("");
  const [result, setResult] = useState<SharedLinkResult>();
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);
  const scrollBody = useRef<HTMLDivElement>(null);

  async function inspect(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setLoading(true);
    setResult(undefined);
    setError("");
    try {
      const response = await fetch("booru-browser/shared-link", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ url: url.trim() }),
      });
      setResult(await readBooruResponse<SharedLinkResult>(response));
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "Cannot inspect share link.");
    } finally {
      setLoading(false);
    }
  }

  const post = result?.post;
  const archive = ["zip", "cbz", "7z"].includes(post?.format?.toLowerCase() ?? "");
  const image = post && booruMediaURL(post, "preview");

  return (
    <details className="booru-shared-links mb-3">
      <summary>Browse a shared link (Dropbox / MEGA)</summary>
      <div className="mt-2" ref={scrollBody}>
        <Form onSubmit={(event) => void inspect(event)}>
          <Form.Group controlId="booru-shared-link" className="mb-2">
            <Form.Label>Public file or folder share link</Form.Label>
            <Form.Control
              type="url"
              autoComplete="off"
              value={url}
              placeholder="https://www.dropbox.com/scl/fi/… or https://mega.nz/file/…"
              maxLength={2048}
              onChange={(event) => setURL(event.currentTarget.value)}
              required
            />
          </Form.Group>
          <Button type="submit" size="sm" disabled={loading || !url.trim()}>
            {loading ? <><Spinner animation="border" size="sm" /> Inspecting…</> : "Find media"}
          </Button>
        </Form>
        <p className="small text-muted mt-2">
          Public Dropbox images/videos and folder ZIPs; public MEGA individual files.
          Folder archives let you choose the media to import. MEGA files require
          megatools on the Stash server; private or password-protected links are not supported.
          Share links are not stored in search history.
        </p>
        {error && <Alert variant="danger" role="alert">{error}</Alert>}
        {result && post && (
          <section aria-label="Shared media" className="mt-3">
            <h5>{post.title || "Shared media"}</h5>
            {post.mediaNotice && <Alert variant="info">{post.mediaNotice}</Alert>}
            {archive ? (
              <BooruArchive
                key={post.mediaKey}
                post={post}
                instance={result.source.instance}
                scrollBody={scrollBody}
              />
            ) : (
              <>
                {image && <img src={image} alt={post.title || "Shared image"} className="img-fluid booru-sample mb-2" loading="lazy" />}
                <BooruDownload key={post.mediaKey} post={post} instance={result.source.instance} />
              </>
            )}
          </section>
        )}
      </div>
    </details>
  );
}
