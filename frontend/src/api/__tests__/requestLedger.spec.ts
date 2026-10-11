import { beforeEach, expect, it, vi } from 'vitest'
import { requestLedgerAPI } from '../requestLedger'
const { get } = vi.hoisted(() => ({ get: vi.fn() }))
vi.mock('../client', () => ({ apiClient: { get } }))
beforeEach(() => { get.mockReset(); get.mockResolvedValue({ data: {} }) })
it.each([false,true])('uses only explicit summary and section requests for details (admin=%s)', async admin => {
 const signal = new AbortController().signal
 const path = `${admin ? '/admin/requests' : '/requests'}/private-id`
 await requestLedgerAPI.detail(admin, 'private-id', signal)
 expect(get).toHaveBeenLastCalledWith(path, { params:{view:'summary'},signal })
 for(const section of ['attempts','billing','wallet','diagnostics'] as const){
  await requestLedgerAPI.sectionPage(admin,'private-id',section,'cursor',signal)
  expect(get).toHaveBeenLastCalledWith(path,{params:{section,cursor:'cursor',page_size:20},signal})
 }
 expect(get).toHaveBeenCalledTimes(5)
})
it('does not fall back to the unbounded legacy detail on a page failure', async()=>{
 get.mockRejectedValueOnce(new Error('synthetic failure'))
 await expect(requestLedgerAPI.sectionPage(false,'id','billing','',new AbortController().signal)).rejects.toThrow('synthetic failure')
 expect(get).toHaveBeenCalledTimes(1)
})
