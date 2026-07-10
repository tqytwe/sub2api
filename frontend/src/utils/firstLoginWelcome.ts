const SESSION_FLAG = 'sub2api_show_first_login_welcome'

function doneKey(userId: number): string {
  return `sub2api_first_login_welcome_done_${userId}`
}

export function markFirstLoginWelcomePending(): void {
  try {
    sessionStorage.setItem(SESSION_FLAG, '1')
  } catch {
    // ignore quota / private mode
  }
}

export function consumeFirstLoginWelcomePending(): boolean {
  try {
    if (sessionStorage.getItem(SESSION_FLAG) !== '1') return false
    sessionStorage.removeItem(SESSION_FLAG)
    return true
  } catch {
    return false
  }
}

export function isFirstLoginWelcomeDone(userId: number): boolean {
  try {
    return localStorage.getItem(doneKey(userId)) === '1'
  } catch {
    return false
  }
}

export function markFirstLoginWelcomeDone(userId: number): void {
  try {
    localStorage.setItem(doneKey(userId), '1')
  } catch {
    // ignore
  }
}
