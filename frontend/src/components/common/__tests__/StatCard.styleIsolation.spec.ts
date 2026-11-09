import { mount } from '@vue/test-utils'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import postcss from 'postcss'
import { describe, expect, it } from 'vitest'
import StatCard from '../StatCard.vue'

// Route styles stay loaded after navigating away from the home page. Include
// media-query rules too: a mobile override must not change dashboard numbers.
const homeStyles = postcss.parse(readFileSync(resolve(process.cwd(), 'src/styles/home-view.css'), 'utf8'))

describe('StatCard after visiting the home page', () => {
  it.each(['0', '4', '66.7%', '70'])('keeps home typography away from the value %s and label', value => {
    const wrapper = mount(StatCard, { props: { title: '本月有效首付人数', value } })
    for (const [target, element] of [
      ['.stat-value', wrapper.get('p[title]').element],
      ['.stat-label', wrapper.findAll('p')[0].element],
    ] as const) {
      const leaks: string[] = []
      homeStyles.walkRules(rule => {
        if (!rule.selector.includes(target)) return
        if (element.matches(rule.selector)) leaks.push(rule.selector)
      })
      expect(leaks, `Home route CSS must not affect a shared dashboard ${target}`).toEqual([])
    }
    expect(wrapper.get('p[title]').text()).toBe(value)
    wrapper.unmount()
  })

  it('retains home number styling at desktop and mobile sizes', () => {
    const home = document.createElement('div')
    home.className = 'home-page'
    home.innerHTML = '<div class="stat"><span class="stat-value">123<span class="stat-unit">+</span></span><span class="stat-label">Requests</span></div>'
    const number = home.querySelector('.stat-value')!
    const responsiveRules: boolean[] = []
    homeStyles.walkRules(rule => {
      if (!rule.selector.includes('.stat-value') || !number.matches(rule.selector)) return
      if (rule.nodes.some(node => node.type === 'decl' && node.prop === 'font-size')) {
        responsiveRules.push(rule.parent?.type === 'atrule')
      }
    })
    expect(responsiveRules).toContain(false)
    expect(responsiveRules).toContain(true)
  })
})
