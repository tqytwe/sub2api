import { nextTick, onBeforeUnmount, watch, type Ref } from 'vue'

type DialogAccessibilityOptions = {
  closeOnEscape?: boolean
  onClose?: () => void
}

const focusableSelector = [
  'button:not([disabled])',
  '[href]',
  'input:not([disabled])',
  'select:not([disabled])',
  'textarea:not([disabled])',
  '[tabindex]:not([tabindex="-1"])',
].join(', ')

const dialogStack: symbol[] = []
const dialogEntries = new Map<symbol, {
  closeOnEscape: boolean
  dialogRef: Ref<HTMLElement | null>
  onClose?: () => void
}>()
let previousAppState: {
  ariaHidden: string | null
  inert: boolean
} | null = null

function getAppRoot() {
  if (typeof document === 'undefined') return null
  return document.getElementById('app') as (HTMLElement & { inert?: boolean }) | null
}

function setBackgroundInert(isInert: boolean) {
  const appRoot = getAppRoot()
  if (!appRoot) return

  if (isInert) {
    if (!previousAppState) {
      previousAppState = {
        ariaHidden: appRoot.getAttribute('aria-hidden'),
        inert: Boolean(appRoot.inert),
      }
    }
    appRoot.setAttribute('aria-hidden', 'true')
    appRoot.inert = true
    return
  }

  if (!previousAppState) return
  if (previousAppState.ariaHidden === null) {
    appRoot.removeAttribute('aria-hidden')
  } else {
    appRoot.setAttribute('aria-hidden', previousAppState.ariaHidden)
  }
  appRoot.inert = previousAppState.inert
  previousAppState = null
}

function lockDocument(dialogId: symbol) {
  if (typeof document === 'undefined') return
  dialogStack.push(dialogId)
  if (dialogStack.length === 1) {
    document.addEventListener('keydown', handleDocumentKeydown, true)
  }
  document.body.classList.add('modal-open')
}

function applyBackgroundInert() {
  if (typeof document === 'undefined') return
  if (dialogStack.length > 0) {
    setBackgroundInert(true)
  }
}

function unlockDocument(dialogId: symbol) {
  if (typeof document === 'undefined') return
  const index = dialogStack.lastIndexOf(dialogId)
  if (index >= 0) {
    dialogStack.splice(index, 1)
  }
  if (dialogStack.length > 0) return
  document.removeEventListener('keydown', handleDocumentKeydown, true)
  document.body.classList.remove('modal-open')
  setBackgroundInert(false)
}

function isTopDialog(dialogId: symbol) {
  return dialogStack[dialogStack.length - 1] === dialogId
}

function topDialogEntry() {
  const dialogId = dialogStack[dialogStack.length - 1]
  if (!dialogId) return null
  return dialogEntries.get(dialogId) ?? null
}

function canRestoreFocus(element: HTMLElement | null, stackStillOpen: boolean) {
  if (!element || typeof element.focus !== 'function') return false
  if (!stackStillOpen) return true
  const appRoot = getAppRoot()
  return !appRoot?.contains(element)
}

function isFocusable(element: HTMLElement) {
  if (element.getAttribute('aria-hidden') === 'true') return false
  if (element.tabIndex < 0) return false
  const style = window.getComputedStyle(element)
  return style.visibility !== 'hidden' && style.display !== 'none'
}

function getFocusableElements(dialog: HTMLElement | null) {
  if (!dialog) return []
  return Array.from(dialog.querySelectorAll<HTMLElement>(focusableSelector))
    .filter(isFocusable)
}

function handleDocumentKeydown(event: KeyboardEvent) {
  const entry = topDialogEntry()
  const dialog = entry?.dialogRef.value
  if (!entry || !dialog) return

  if (entry.closeOnEscape && event.key === 'Escape') {
    event.preventDefault()
    event.stopPropagation()
    entry.onClose?.()
    return
  }

  if (event.key !== 'Tab') return
  const focusable = getFocusableElements(dialog)
  if (!focusable.length) {
    event.preventDefault()
    dialog.focus()
    return
  }

  const first = focusable[0]
  const last = focusable[focusable.length - 1]
  const activeElement = document.activeElement
  const focusOutsideDialog = !dialog.contains(activeElement)
  if (event.shiftKey && (activeElement === first || focusOutsideDialog)) {
    event.preventDefault()
    last.focus()
  } else if (!event.shiftKey && (activeElement === last || focusOutsideDialog)) {
    event.preventDefault()
    first.focus()
  }
}

export function useDialogAccessibility(
  isOpen: Ref<boolean>,
  dialogRef: Ref<HTMLElement | null>,
  options: DialogAccessibilityOptions = {}
) {
  let active = false
  let previousActiveElement: HTMLElement | null = null
  const dialogId = Symbol('dialog')
  const closeOnEscape = options.closeOnEscape ?? true

  const focusDialog = async () => {
    await nextTick()
    if (!active || !dialogRef.value) return
    const firstFocusable = getFocusableElements(dialogRef.value)[0]
    if (firstFocusable) {
      firstFocusable.focus()
      return
    }
    dialogRef.value.focus()
  }

  const activate = () => {
    if (active || typeof document === 'undefined') return
    active = true
    previousActiveElement = document.activeElement as HTMLElement | null
    dialogEntries.set(dialogId, {
      closeOnEscape,
      dialogRef,
      onClose: options.onClose,
    })
    lockDocument(dialogId)
    void focusDialog().then(() => {
      if (active && isTopDialog(dialogId)) {
        applyBackgroundInert()
      }
    })
  }

  const deactivate = () => {
    if (!active || typeof document === 'undefined') return
    active = false
    const wasTopDialog = isTopDialog(dialogId)
    const elementToRestore = previousActiveElement
    unlockDocument(dialogId)
    dialogEntries.delete(dialogId)
    if (wasTopDialog && elementToRestore && canRestoreFocus(elementToRestore, dialogStack.length > 0)) {
      elementToRestore.focus({ preventScroll: true })
    }
    previousActiveElement = null
  }

  watch(isOpen, (open) => {
    if (open) activate()
    else deactivate()
  }, { immediate: true })

  onBeforeUnmount(() => {
    deactivate()
  })

  return {
    focusDialog,
  }
}
