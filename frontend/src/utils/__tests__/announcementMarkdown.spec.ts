import { describe, expect, it } from 'vitest'
import { renderAnnouncementMarkdown } from '@/utils/announcementMarkdown'

describe('renderAnnouncementMarkdown', () => {
  it('renders platform-hosted images, fixed highlights, and semantic tones', () => {
    const html = renderAnnouncementMarkdown([
      '![海报](/api/v1/announcement-assets/announcements/a.png)',
      '',
      '==重点== ::success[已上线]',
      '',
      '> [!WARNING] 注意维护窗口',
    ].join('\n'))

    expect(html).toContain('/api/v1/announcement-assets/announcements/a.png')
    expect(html).toContain('<mark>重点</mark>')
    expect(html).toContain('data-announcement-tone="success"')
    expect(html).toContain('data-announcement-alert="warning"')
  })

  it('removes external/data images and unsafe inline html', () => {
    const html = renderAnnouncementMarkdown([
      '![外部](https://example.com/a.png)',
      '![内联](data:image/png;base64,AAAA)',
      '<span style="color:red" onclick="alert(1)">red</span>',
    ].join('\n'))

    expect(html).not.toContain('https://example.com/a.png')
    expect(html).not.toContain('data:image')
    expect(html).not.toContain('style=')
    expect(html).not.toContain('onclick')
  })
})
