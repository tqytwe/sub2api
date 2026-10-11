from pathlib import Path
import json,sys,hashlib
root=Path(sys.argv[1]).resolve();mode=sys.argv[2]
assert mode in ('release_on_first_stats','drop_ack')
source=root/'backend/internal/repository/image_task_worker_integration_test.go'
text=source.read_text()
needle='queue := NewImageTaskQueue(rdb, cfg)'
assert text.count(needle)>=1
text=text.replace(needle,'queue := &ackBoundaryProbeQueue{ImageTaskQueue: NewImageTaskQueue(rdb, cfg), gate: make(chan struct{}), drop: '+str(mode=='drop_ack').lower()+'}',1)
# Test-only wrapper delegates every production operation except the final two
# lease acknowledgements. No task body, processor or production file is changed.
text+='''

type ackBoundaryProbeQueue struct {
 service.ImageTaskQueue
 gate chan struct{}
 once sync.Once
 drop bool
}
func (q *ackBoundaryProbeQueue) TryAcquireJobLock(ctx context.Context, id string, ttl time.Duration) (service.ImageTaskJobLock, bool, error) {
 lock, ok, err := q.ImageTaskQueue.TryAcquireJobLock(ctx,id,ttl)
 if err != nil || !ok || (id != "imgtask_worker_c" && id != "imgtask_worker_d") { return lock,ok,err }
 return &ackBoundaryProbeLock{ImageTaskJobLock:lock, owner:ctx, queue:q},ok,err
}
func (q *ackBoundaryProbeQueue) Stats(ctx context.Context) (service.ImageTaskQueueStats,error) {
 stats,err := q.ImageTaskQueue.Stats(ctx)
 q.once.Do(func(){close(q.gate)})
 return stats,err
}
type ackBoundaryProbeLock struct {service.ImageTaskJobLock; owner context.Context; queue *ackBoundaryProbeQueue}
func (l *ackBoundaryProbeLock) Ack(ctx context.Context) error {
 if l.queue.drop {return nil}
 select {case <-l.queue.gate: case <-l.owner.Done(): return l.owner.Err()}
 return l.ImageTaskJobLock.Ack(ctx)
}
'''
prefix=Path('/tmp/routing-worker-'+mode+'-'+root.name);derived=prefix.with_suffix('.go');derived.write_text(text);overlay=prefix.with_suffix('.json');overlay.write_text(json.dumps({'Replace':{str(source):str(derived)}})+'\n')
print(json.dumps({'overlay':str(overlay),'source_sha256':hashlib.sha256(source.read_bytes()).hexdigest(),'derived_sha256':hashlib.sha256(derived.read_bytes()).hexdigest(),'mode':mode}))
