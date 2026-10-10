package requestledger

import "context"

// BeginQueryExecution gives a shared, detached model-catalogue refresh its own
// durable lifetime. The first cache miss/stale reader owns the actual refresh;
// other cache consumers retain their own admissions without fictitious sends.
// Only the verified handle is copied, never headers or request content.
func BeginQueryExecution(ctx context.Context) (*Handle, error) {
	parent := FromContext(ctx)
	if parent == nil {
		return nil, nil
	}
	h, err := parent.ledger.begin(ctx, parent.route, "WORKER", "async_execution", parent, 0)
	if err != nil {
		parent.mu.Lock()
		parent.admissionFailed = true
		parent.mu.Unlock()
		return nil, err
	}
	h.metered = false
	return h, nil
}
