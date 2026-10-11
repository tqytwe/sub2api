import base from '/workspace/p1-interactive-auth/frontend/vitest.config.ts'
export default {...base,test:{...base.test,include:['/tmp/p1-merge-ledger-storage-probe/probe.spec.ts']}}
