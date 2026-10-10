package service

import (
	"context"
	"github.com/Wei-Shaw/sub2api/internal/requestledger"
	coderws "github.com/coder/websocket"
	"sync"
)

type ledgerLiveFrameConn struct {
	inner     liveFrameConn
	audit     *realtimeLedgerAudit
	execution *requestledger.Handle
	ctx       context.Context
	once      sync.Once
}

func (c *ledgerLiveFrameConn) ReadFrame(ctx context.Context) (coderws.MessageType, []byte, error) {
	kind, payload, err := c.inner.ReadFrame(ctx)
	if err == nil {
		err = c.audit.Observe(payload)
	}
	if err != nil {
		c.finish(err)
	}
	return kind, payload, err
}
func (c *ledgerLiveFrameConn) WriteFrame(ctx context.Context, kind coderws.MessageType, payload []byte) error {
	if err := c.audit.BeforeWrite(payload); err != nil {
		c.finish(err)
		return err
	}
	err := c.inner.WriteFrame(ctx, kind, payload)
	c.audit.AfterWrite(err)
	if err != nil {
		c.finish(err)
	}
	return err
}
func (c *ledgerLiveFrameConn) finish(cause error) {
	c.once.Do(func() {
		if coderws.CloseStatus(cause) == coderws.StatusNormalClosure {
			cause = nil
		}
		c.audit.Close(cause)
		_ = c.execution.Finish(c.ctx, requestledger.Outcome(0, cause), 0, requestledger.ErrorCode(cause))
	})
}
func (c *ledgerLiveFrameConn) Close() error { c.finish(nil); return c.inner.Close() }
