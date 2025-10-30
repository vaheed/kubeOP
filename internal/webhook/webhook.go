package webhook

import (
    "crypto/hmac"
    "crypto/sha256"
    "encoding/hex"
    "encoding/json"
    "io"
    "net/http"
    "time"

    "github.com/vaheed/kubeop/internal/metrics"
)

type Client struct {
    URL    string
    Secret []byte
    HTTP   *http.Client
}

func (c *Client) Send(event string, payload any) error {
    if c == nil || c.URL == "" { return nil }
    body, _ := json.Marshal(struct {
        Event string `json:"event"`
        Data  any    `json:"data"`
    }{event, payload})
    req, _ := http.NewRequest("POST", c.URL, bytesReader(body))
    mac := hmac.New(sha256.New, c.Secret)
    mac.Write(body)
    sig := hex.EncodeToString(mac.Sum(nil))
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("X-KubeOP-Signature", "sha256="+sig)
    httpc := c.HTTP
    if httpc == nil { httpc = http.DefaultClient }
    var lastErr error
    for i := 0; i < 3; i++ {
        start := time.Now()
        resp, err := httpc.Do(req)
        metrics.ObserveWebhookLatency(time.Since(start))
        if err != nil {
            lastErr = err
            metrics.IncWebhookFailure()
            time.Sleep(time.Duration(1<<i) * 100 * time.Millisecond)
            continue
        }
        if resp.StatusCode >= 300 {
            metrics.IncWebhookFailure()
            lastErr = io.EOF
            _ = resp.Body.Close()
            time.Sleep(time.Duration(1<<i) * 100 * time.Millisecond)
            continue
        }
        _ = resp.Body.Close()
        metrics.IncWebhookEvent(event, "success")
        return nil
    }
    metrics.IncWebhookEvent(event, "failure")
    return lastErr
}

// small helper to avoid importing bytes in all callers
func bytesReader(b []byte) *bytesReaderWrap { return &bytesReaderWrap{b: b} }
type bytesReaderWrap struct{ b []byte; i int64 }
func (r *bytesReaderWrap) Read(p []byte) (int, error) {
    if r.i >= int64(len(r.b)) { return 0, io.EOF }
    n := copy(p, r.b[r.i:])
    r.i += int64(n)
    return n, nil
}
func (r *bytesReaderWrap) Close() error { return nil }

// satisfy io.ReadCloser
var _ interface{ Read([]byte) (int, error); Close() error } = (*bytesReaderWrap)(nil)
