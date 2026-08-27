import { describe, expect, it } from 'vitest'
import {
  cloneMediaCapabilities,
  emptyMediaCapabilities,
  knownMediaModalities,
  normalizeMediaCapabilities,
  validateMediaCapabilities,
} from '@/utils/modelMediaCapabilities'

describe('model media capabilities', () => {
  it('normalizes declared capabilities without dropping unknown persisted fields', () => {
    const capabilities = normalizeMediaCapabilities({
      version: 'v1',
      adapter: 'sensenova-u1.5',
      modalities: ['image', 'provider_future_mode', 'image'],
      image: {
        operations: ['create', 'edit'],
        supported_output_formats: ['png', 'jpeg'],
        provider_extension: 'keep-me',
      },
      release_channel: 'stable',
    })

    expect(capabilities).toMatchObject({
      version: 'v1',
      adapter: 'sensenova-u1.5',
      modalities: ['image', 'provider_future_mode'],
      image: {
        operations: ['create', 'edit'],
        supported_output_formats: ['png', 'jpeg'],
        provider_extension: 'keep-me',
      },
      release_channel: 'stable',
    })
    expect(knownMediaModalities(capabilities)).toEqual(['image'])
  })

  it('returns an independent editable copy', () => {
    const source = normalizeMediaCapabilities({
      version: 'v1',
      adapter: 'image-adapter',
      modalities: ['image'],
      image: { operations: ['create'] },
    })
    const copy = cloneMediaCapabilities(source)

    copy.image?.operations.push('edit')

    expect(source.image?.operations).toEqual(['create'])
    expect(copy.image?.operations).toEqual(['create', 'edit'])
  })

  it('preserves unknown persisted modalities while exposing only supported choices to the editor', () => {
    const source = normalizeMediaCapabilities({
      version: 'v1',
      adapter: 'provider-adapter',
      modalities: ['image', 'embedding', 'image'],
      image: { operations: ['create'] },
    })
    const copy = cloneMediaCapabilities(source)

    expect(copy.modalities).toEqual(['image', 'embedding'])
    expect(knownMediaModalities(copy)).toEqual(['image'])
  })

  it('requires a complete declaration for each selected media modality', () => {
    const capabilities = emptyMediaCapabilities()
    capabilities.version = ''
    capabilities.modalities = ['image', 'video']

    expect(validateMediaCapabilities(capabilities)).toEqual([
      'version_required',
      'adapter_required',
      'image_operations_required',
      'video_operations_required',
    ])

    capabilities.version = 'v1'
    capabilities.adapter = 'provider-adapter'
    capabilities.image = { operations: ['create'] }
    capabilities.video = { operations: ['generate'] }

    expect(validateMediaCapabilities(capabilities)).toEqual([])
  })

  it('rejects aliases and invalid limits before an administrator can save them', () => {
    const capabilities = {
      version: 'v1',
      adapter: 'provider-adapter',
      modalities: ['image', 'video'],
      image: {
        operations: ['generation'],
        min_dimension: 4096,
        max_dimension: 512,
      },
      video: {
        operations: ['create'],
        durations_seconds: [8, 8],
        max_reference_images: -1,
      },
    }

    expect(validateMediaCapabilities(capabilities)).toEqual([
      'image_operations_invalid',
      'image_limits_invalid',
      'video_operations_invalid',
      'video_limits_invalid',
    ])
  })

  it('rejects duplicate canonical operations instead of silently accepting them', () => {
    const capabilities = {
      version: 'v1',
      adapter: 'provider-adapter',
      modalities: ['image'],
      image: { operations: ['create', 'CREATE'] },
    }

    expect(validateMediaCapabilities(capabilities)).toEqual(['image_operations_invalid'])
  })

  it('requires a modality when an administrator starts a declaration', () => {
    expect(normalizeMediaCapabilities(null)).toBeNull()
    expect(validateMediaCapabilities(emptyMediaCapabilities())).toEqual(['modalities_required'])
  })
})
