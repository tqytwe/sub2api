import { describe, expect, it } from 'vitest'
import { getHostedQRCodeImageUrl } from '@/components/payment/qrDisplay'

describe('getHostedQRCodeImageUrl', () => {
  it('keeps hosted XunhuPay QR URLs as displayable images', () => {
    expect(getHostedQRCodeImageUrl('https://api.xunhupay.com/qrcode/42')).toBe('https://api.xunhupay.com/qrcode/42')
    expect(getHostedQRCodeImageUrl('https://api.dpweixin.com/payment/qrcode?id=42')).toBe('https://api.dpweixin.com/payment/qrcode?id=42')
  })

  it('keeps direct image URLs from other hosts displayable', () => {
    expect(getHostedQRCodeImageUrl('https://cdn.example.com/orders/42.png')).toBe('https://cdn.example.com/orders/42.png')
  })

  it('leaves native payment payloads for QR generation', () => {
    expect(getHostedQRCodeImageUrl('weixin://wxpay/bizpayurl?pr=42')).toBe('')
    expect(getHostedQRCodeImageUrl('https://qr.alipay.com/fkx123456789')).toBe('')
    expect(getHostedQRCodeImageUrl('not a url')).toBe('')
  })
})
