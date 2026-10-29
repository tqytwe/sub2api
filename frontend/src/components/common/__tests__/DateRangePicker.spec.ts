import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { ref } from 'vue'

import DateRangePicker from '../DateRangePicker.vue'

const messages: Record<string, string> = {
  'dates.today': 'Today',
  'dates.yesterday': 'Yesterday',
  'dates.last24Hours': 'Last 24 Hours',
  'dates.last7Days': 'Last 7 Days',
  'dates.last14Days': 'Last 14 Days',
  'dates.last30Days': 'Last 30 Days',
  'dates.thisMonth': 'This Month',
  'dates.lastMonth': 'Last Month',
  'dates.startDate': 'Start Date',
  'dates.endDate': 'End Date',
  'dates.apply': 'Apply',
  'dates.selectDateRange': 'Select date range'
}

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => messages[key] ?? key,
    locale: ref('en')
  })
}))

const formatLocalDate = (date: Date): string => {
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}

describe('DateRangePicker', () => {
  it('uses last 24 hours as the default recognized preset', () => {
    const now = new Date()
    const yesterday = new Date(now.getTime() - 24 * 60 * 60 * 1000)

    const wrapper = mount(DateRangePicker, {
      props: {
        startDate: formatLocalDate(yesterday),
        endDate: formatLocalDate(now)
      },
      global: {
        stubs: {
          Icon: true
        }
      }
    })

    expect(wrapper.text()).toContain('Last 24 Hours')
  })

  it('emits range updates with last24Hours preset when applied', async () => {
    const now = new Date()
    const today = formatLocalDate(now)

    const wrapper = mount(DateRangePicker, {
      props: {
        startDate: today,
        endDate: today
      },
      global: {
        stubs: {
          Icon: true
        }
      }
    })

    await wrapper.find('.date-picker-trigger').trigger('click')
    const presetButton = wrapper.findAll('.date-picker-preset').find((node) =>
      node.text().includes('Last 24 Hours')
    )
    expect(presetButton).toBeDefined()

    await presetButton!.trigger('click')
    await wrapper.find('.date-picker-apply').trigger('click')

    const nowAfterClick = new Date()
    const yesterdayAfterClick = new Date(nowAfterClick.getTime() - 24 * 60 * 60 * 1000)
    const expectedStart = formatLocalDate(yesterdayAfterClick)
    const expectedEnd = formatLocalDate(nowAfterClick)

    expect(wrapper.emitted('update:startDate')?.[0]).toEqual([expectedStart])
    expect(wrapper.emitted('update:endDate')?.[0]).toEqual([expectedEnd])
    expect(wrapper.emitted('change')?.[0]).toEqual([
      {
        startDate: expectedStart,
        endDate: expectedEnd,
        preset: 'last24Hours'
      }
    ])
  })

  it('keeps a rolling 24-hour preset as exact instants for admin consumers', async () => {
    const wrapper = mount(DateRangePicker, {
      props: {
        startDate: '2026-10-27T12:00:00.000Z',
        endDate: '2026-10-28T12:00:00.000Z',
        rolling24Hours: true,
        selectedPreset: 'last24Hours',
      },
      global: { stubs: { Icon: true } },
    })

    await wrapper.find('.date-picker-trigger').trigger('click')
    const presetButton = wrapper.findAll('.date-picker-preset').find((node) =>
      node.text().includes('Last 24 Hours')
    )
    await presetButton!.trigger('click')
    await wrapper.find('.date-picker-apply').trigger('click')

    const range = wrapper.emitted('change')?.[0]?.[0] as {
      startDate: string
      endDate: string
      preset: string
    }
    expect(range.preset).toBe('last24Hours')
    expect(Date.parse(range.endDate) - Date.parse(range.startDate)).toBe(24 * 60 * 60 * 1000)
    expect(range.startDate).toContain('T')
    expect(range.endDate).toContain('T')
  })

  it('keeps the rolling preset label when exact timestamps are supplied by the parent', () => {
    const wrapper = mount(DateRangePicker, {
      props: {
        startDate: '2026-10-27T12:00:00.000Z',
        endDate: '2026-10-28T12:00:00.000Z',
        rolling24Hours: true,
        selectedPreset: 'last24Hours',
      },
      global: { stubs: { Icon: true } },
    })

    expect(wrapper.text()).toContain('Last 24 Hours')
  })

  it('does not reinterpret a historical exact 24-hour range as the rolling preset', async () => {
    const startDate = '2026-10-20T12:00:00.000Z'
    const endDate = '2026-10-21T12:00:00.000Z'
    const wrapper = mount(DateRangePicker, {
      props: { startDate, endDate, rolling24Hours: true, selectedPreset: null },
      global: { stubs: { Icon: true } },
    })

    await wrapper.find('.date-picker-trigger').trigger('click')
    await wrapper.find('.date-picker-apply').trigger('click')

    const applied = wrapper.emitted('change')?.at(-1)?.[0] as {
      startDate: string
      endDate: string
      preset: string | null
    }
    expect(applied.preset).toBeNull()
    expect(applied.startDate).toBe(formatLocalDate(new Date(startDate)))
    expect(applied.endDate).toBe(formatLocalDate(new Date(endDate)))
  })

  it('keeps manually edited dates instead of reapplying a stale rolling preset', async () => {
    const wrapper = mount(DateRangePicker, {
      props: {
        startDate: '2026-10-27T12:00:00.000Z',
        endDate: '2026-10-28T12:00:00.000Z',
        rolling24Hours: true,
        selectedPreset: 'last24Hours',
      },
      global: { stubs: { Icon: true } },
    })

    await wrapper.find('.date-picker-trigger').trigger('click')
    const inputs = wrapper.findAll('.date-picker-input')
    await inputs[0].setValue('2026-10-10')
    await inputs[1].setValue('2026-10-12')
    await wrapper.find('.date-picker-apply').trigger('click')

    expect(wrapper.emitted('change')?.at(-1)?.[0]).toEqual({
      startDate: '2026-10-10',
      endDate: '2026-10-12',
      preset: null,
    })
  })

  it('applies the today preset immediately when selected', async () => {
    const today = formatLocalDate(new Date())
    const wrapper = mount(DateRangePicker, {
      props: {
        startDate: formatLocalDate(new Date(Date.now() - 24 * 60 * 60 * 1000)),
        endDate: today
      },
      global: { stubs: { Icon: true } }
    })

    await wrapper.find('.date-picker-trigger').trigger('click')
    const presetButton = wrapper.findAll('.date-picker-preset').find((node) => node.text().includes('Today'))
    expect(presetButton).toBeDefined()
    await presetButton!.trigger('click')

    expect(wrapper.emitted('update:startDate')?.at(-1)).toEqual([today])
    expect(wrapper.emitted('update:endDate')?.at(-1)).toEqual([today])
    expect(wrapper.emitted('change')?.at(-1)).toEqual([{
      startDate: today,
      endDate: today,
      preset: 'today'
    }])
  })
})
