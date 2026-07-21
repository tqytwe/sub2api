<script setup lang="ts">
import { computed } from 'vue'
import { useAppStore } from '@/stores'
import type { SupportContactConfig, SupportContactMethod } from '@/types'
import Icon from '@/components/icons/Icon.vue'
import {
  enabledSupportContacts,
  normalizeSupportContactConfig,
  primaryQRCodeContacts,
  sanitizeSupportContactImage,
  supportContactActionUrl,
  supportContactCopyValue,
  supportContactDisplayValue,
} from '@/utils/supportContact'

const props = withDefaults(defineProps<{
  config?: SupportContactConfig | null
  compact?: boolean
  showHeader?: boolean
}>(), {
  config: null,
  compact: false,
  showHeader: true,
})

const normalizedConfig = computed(() =>
  normalizeSupportContactConfig(props.config)
)
const contacts = computed(() => enabledSupportContacts(normalizedConfig.value))
const primaryContacts = computed(() => primaryQRCodeContacts(normalizedConfig.value))
const primaryContactIDs = computed(() => new Set(primaryContacts.value.map((contact) => contact.id)))
const secondaryContacts = computed(() =>
  contacts.value.filter((contact) => !primaryContactIDs.value.has(contact.id))
)
const hasContacts = computed(() => contacts.value.length > 0)

function contactTypeLabel(type: string): string {
  switch (type) {
    case 'wechat':
      return '微信'
    case 'qq':
      return 'QQ'
    case 'telegram':
      return 'TG'
    case 'email':
      return '邮箱'
    case 'docs':
      return '文档'
    default:
      return '客服'
  }
}

function contactIconName(type: string): 'chat' | 'mail' | 'book' | 'link' {
  if (type === 'email') return 'mail'
  if (type === 'docs') return 'book'
  if (type === 'custom') return 'link'
  return 'chat'
}

function qrImage(contact: SupportContactMethod): string {
  return sanitizeSupportContactImage(contact.qr_image || '')
}

function actionUrl(contact: SupportContactMethod): string {
  return supportContactActionUrl(contact)
}

function displayValue(contact: SupportContactMethod): string {
  return supportContactDisplayValue(contact)
}

async function copyContact(contact: SupportContactMethod): Promise<void> {
  const value = supportContactCopyValue(contact)
  if (!value) return
  const copied = await copyText(value)
  notifyCopyResult(copied)
}

function notifyCopyResult(copied: boolean): void {
  let appStore: ReturnType<typeof useAppStore> | null = null
  try {
    appStore = useAppStore()
  } catch {
    return
  }
  if (copied) {
    appStore.showSuccess('已复制到剪贴板')
  } else {
    appStore.showError('复制失败')
  }
}

function openContact(contact: SupportContactMethod): void {
  const url = actionUrl(contact)
  if (!url || typeof window === 'undefined') return
  window.open(url, '_blank', 'noopener')
}

async function copyText(text: string): Promise<boolean> {
  if (!text || typeof window === 'undefined' || typeof document === 'undefined') return false

  if (navigator.clipboard && window.isSecureContext) {
    try {
      await navigator.clipboard.writeText(text)
      return true
    } catch {
      return fallbackCopyText(text)
    }
  }

  return fallbackCopyText(text)
}

function fallbackCopyText(text: string): boolean {
  const textarea = document.createElement('textarea')
  textarea.value = text
  textarea.setAttribute('readonly', 'true')
  textarea.style.cssText = 'position:fixed;left:0;top:0;width:1px;height:1px;opacity:0;pointer-events:none'
  document.body.appendChild(textarea)
  textarea.focus({ preventScroll: true })
  textarea.select()
  textarea.setSelectionRange(0, textarea.value.length)
  try {
    return document.execCommand('copy')
  } finally {
    document.body.removeChild(textarea)
  }
}
</script>

<template>
  <section
    v-if="hasContacts"
    class="support-contact-panel"
    :class="{ 'support-contact-panel--compact': compact }"
  >
    <div v-if="showHeader" class="mb-4">
      <div class="flex items-center gap-2">
        <span class="inline-flex h-8 w-8 items-center justify-center rounded-lg bg-primary-50 text-primary-600 dark:bg-primary-900/30 dark:text-primary-300">
          <Icon name="chat" size="sm" />
        </span>
        <h3 class="text-base font-semibold text-gray-900 dark:text-white">
          {{ normalizedConfig.title }}
        </h3>
      </div>
      <p
        v-if="normalizedConfig.subtitle"
        class="mt-2 text-sm leading-6 text-gray-500 dark:text-dark-400"
      >
        {{ normalizedConfig.subtitle }}
      </p>
    </div>

    <div
      v-if="primaryContacts.length"
      class="grid gap-3"
      :class="compact ? 'sm:grid-cols-2' : 'md:grid-cols-2'"
    >
      <article
        v-for="contact in primaryContacts"
        :key="contact.id"
        class="rounded-lg border border-gray-200 bg-white p-3 shadow-sm dark:border-dark-700 dark:bg-dark-900"
      >
        <div class="aspect-square overflow-hidden rounded-md border border-gray-100 bg-gray-50 dark:border-dark-700 dark:bg-dark-800">
          <img
            :src="qrImage(contact)"
            :alt="contact.label"
            class="h-full w-full object-contain"
            loading="lazy"
          />
        </div>
        <div class="mt-3 min-w-0">
          <div class="flex items-center gap-2">
            <span class="inline-flex h-6 min-w-6 items-center justify-center rounded-md bg-gray-100 px-1.5 text-[11px] font-semibold text-gray-700 dark:bg-dark-800 dark:text-dark-200">
              {{ contactTypeLabel(contact.type) }}
            </span>
            <p class="truncate text-sm font-semibold text-gray-900 dark:text-white">
              {{ contact.label }}
            </p>
          </div>
          <p
            v-if="displayValue(contact)"
            class="mt-1 truncate font-mono text-xs text-gray-500 dark:text-dark-400"
          >
            {{ displayValue(contact) }}
          </p>
          <p
            v-if="contact.description"
            class="mt-1 text-xs leading-5 text-gray-500 dark:text-dark-400"
          >
            {{ contact.description }}
          </p>
        </div>
        <div class="mt-3 flex gap-2">
          <button
            v-if="supportContactCopyValue(contact)"
            type="button"
            class="btn btn-secondary btn-sm flex-1"
            @click="copyContact(contact)"
          >
            <Icon name="copy" size="xs" class="mr-1" />
            复制
          </button>
          <button
            v-if="actionUrl(contact)"
            type="button"
            class="btn btn-secondary btn-sm flex-1"
            @click="openContact(contact)"
          >
            <Icon name="externalLink" size="xs" class="mr-1" />
            打开
          </button>
        </div>
      </article>
    </div>

    <div v-if="secondaryContacts.length" :class="primaryContacts.length ? 'mt-4' : ''">
      <p
        v-if="primaryContacts.length"
        class="mb-2 text-xs font-medium uppercase tracking-wide text-gray-400 dark:text-dark-500"
      >
        更多联系方式
      </p>
      <div class="space-y-2">
        <div
          v-for="contact in secondaryContacts"
          :key="contact.id"
          class="flex items-center gap-3 rounded-lg border border-gray-200 bg-white px-3 py-2.5 dark:border-dark-700 dark:bg-dark-900"
        >
          <span class="flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-lg bg-gray-100 text-gray-600 dark:bg-dark-800 dark:text-dark-200">
            <Icon :name="contactIconName(contact.type)" size="sm" />
          </span>
          <div class="min-w-0 flex-1">
            <div class="flex items-center gap-2">
              <p class="truncate text-sm font-medium text-gray-900 dark:text-white">
                {{ contact.label }}
              </p>
              <span class="rounded-md bg-gray-100 px-1.5 py-0.5 text-[10px] font-medium text-gray-500 dark:bg-dark-800 dark:text-dark-300">
                {{ contactTypeLabel(contact.type) }}
              </span>
            </div>
            <p
              v-if="displayValue(contact)"
              class="truncate font-mono text-xs text-gray-500 dark:text-dark-400"
            >
              {{ displayValue(contact) }}
            </p>
            <p
              v-if="contact.description"
              class="mt-0.5 line-clamp-2 text-xs text-gray-500 dark:text-dark-400"
            >
              {{ contact.description }}
            </p>
          </div>
          <div class="flex flex-shrink-0 items-center gap-1">
            <button
              v-if="supportContactCopyValue(contact)"
              type="button"
              class="btn-ghost btn-icon"
              title="复制"
              @click="copyContact(contact)"
            >
              <Icon name="copy" size="sm" />
            </button>
            <button
              v-if="actionUrl(contact)"
              type="button"
              class="btn-ghost btn-icon"
              title="打开"
              @click="openContact(contact)"
            >
              <Icon name="externalLink" size="sm" />
            </button>
          </div>
        </div>
      </div>
    </div>
  </section>
</template>
