export const CHINESE_PRODUCT_TERMS = {
  Prompt: '提示词',
  'Image Studio': '图像工作室',
  Gallery: '作品库',
  Remix: '智能改写',
  Recipe: '创作配方',
  Featured: '精选',
  'Reference Image': '参考图',
  Copy: '复制',
  Generate: '开始生成',
  Source: '内容说明',
  'Use Cases': '使用场景',
} as const

export type ForbiddenProductUiTerm = keyof typeof CHINESE_PRODUCT_TERMS
